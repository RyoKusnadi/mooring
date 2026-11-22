// Package fakelsp is a stand-in language server for tests. A test binary runs
// it when asked (see Run), so no separate program has to be built.
//
// It speaks LSP over stdio with Content-Length framing. Completion lists the
// distinct words of three letters or more in every open document plus
// "fakeCompletion"; hover says "fake hover". Open and changed documents get
// one error "found ERROR" per line containing ERROR. When
// workspace/didChangeWatchedFiles names a file on disk that contains
// BREAKS_OTHERS, every open document in that folder gets the error "broken by
// <file name>" until that file changes again. Formatting removes trailing
// spaces and tabs, one edit per line. Document symbols and the slow initialize switch are in
// symbols.go.
//
// Triggers in document text: BLOAT typed into a document (not one opened with it, so a
// server restarted for memory does not grow again on reopen) keeps about 300 MiB resident, CRASH writes
// a line to stderr and exits with status 3, ORPHAN exits with status 3 and leaves
// a sleeping child that holds stdout, SERVERREQ sends the client a
// workspace/configuration and a client/registerCapability request and logs
// the answers as window/logMessage. The request fake/state returns what the
// server saw: the initialize params, open documents and every message method.
package fakelsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type msg struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

type server struct {
	out io.Writer
	wmu sync.Mutex

	docs     map[string]string // URI -> text
	broken   map[string]string // file path -> its name, for files that contain BREAKS_OTHERS
	initArgs json.RawMessage
	seen     []string
	bloat    [][]byte
	nextReq  int
	asked    map[string]string // request id -> what was asked
}

var word = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{2,}`)

// Run serves one connection until in ends.
func Run(in io.Reader, out io.Writer) {
	s := &server{out: out, docs: map[string]string{}, broken: map[string]string{}, asked: map[string]string{}}
	br := bufio.NewReaderSize(in, 1<<20)
	for {
		body, err := readFrame(br)
		if err != nil {
			return
		}
		var m msg
		if json.Unmarshal(body, &m) == nil {
			s.handle(m)
		}
	}
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	n := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(k, "Content-Length") {
			n, _ = strconv.Atoi(strings.TrimSpace(v))
		}
	}
	if n < 0 {
		return nil, fmt.Errorf("no Content-Length")
	}
	body := make([]byte, n)
	_, err := io.ReadFull(r, body)
	return body, err
}

func (s *server) send(v any) {
	raw, _ := json.Marshal(v)
	s.wmu.Lock()
	defer s.wmu.Unlock()
	_, _ = fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n%s", len(raw), raw)
}

func (s *server) reply(id json.RawMessage, result any) {
	s.send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (s *server) notify(method string, params any) {
	s.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

type textDoc struct {
	URI  string `json:"uri"`
	Text string `json:"text"`
}

func (s *server) handle(m msg) {
	if m.Method == "" {
		s.answered(m)
		return
	}
	hasID := len(m.ID) > 0
	if m.Method != "fake/state" {
		s.seen = append(s.seen, m.Method+s.uriOf(m.Params))
	}
	switch m.Method {
	case "initialize":
		s.initArgs = m.Params
		time.Sleep(initDelay(m.Params))
		s.reply(m.ID, map[string]any{
			"capabilities": capabilities(),
			"serverInfo":   map[string]string{"name": "fakelsp"},
		})
	case "shutdown":
		s.reply(m.ID, nil)
	case "exit":
		os.Exit(0)
	case "textDocument/didOpen":
		var p struct{ TextDocument textDoc }
		_ = json.Unmarshal(m.Params, &p)
		s.docs[p.TextDocument.URI] = p.TextDocument.Text
		s.changed(p.TextDocument.URI, p.TextDocument.Text, false)
	case "textDocument/didChange":
		s.didChange(m.Params)
	case "textDocument/didClose":
		var p struct{ TextDocument textDoc }
		_ = json.Unmarshal(m.Params, &p)
		delete(s.docs, p.TextDocument.URI)
		s.notify("textDocument/publishDiagnostics", map[string]any{"uri": p.TextDocument.URI, "diagnostics": []any{}})
	case "workspace/didChangeWatchedFiles":
		s.watched(m.Params)
	case "textDocument/completion":
		s.reply(m.ID, map[string]any{"isIncomplete": false, "items": s.completions()})
	case "textDocument/hover":
		s.reply(m.ID, map[string]any{"contents": map[string]string{"kind": "plaintext", "value": "fake hover"}})
	case "textDocument/formatting":
		s.reply(m.ID, s.formatting(m.Params))
	case "textDocument/references", "textDocument/rename":
		s.reply(m.ID, s.refactor(m.Method, m.Params))
	case "textDocument/documentSymbol":
		var p struct{ TextDocument textDoc }
		_ = json.Unmarshal(m.Params, &p)
		s.reply(m.ID, symbols(s.docs[p.TextDocument.URI]))
	case "fake/state":
		uris := make([]string, 0, len(s.docs))
		for u := range s.docs {
			uris = append(uris, u)
		}
		sort.Strings(uris)
		s.reply(m.ID, map[string]any{"initialize": s.initArgs, "open": uris, "seen": s.seen})
	default:
		if hasID {
			s.reply(m.ID, nil)
		}
	}
}

func (s *server) uriOf(params json.RawMessage) string {
	var p struct{ TextDocument textDoc }
	if json.Unmarshal(params, &p) == nil && p.TextDocument.URI != "" {
		return " " + p.TextDocument.URI
	}
	return ""
}

// answered logs the answer to a request the server made.
func (s *server) answered(m msg) {
	key := string(m.ID)
	what, ok := s.asked[key]
	if !ok {
		return
	}
	delete(s.asked, key)
	text := string(m.Result)
	if len(m.Error) > 0 {
		text = "error " + string(m.Error)
	}
	s.notify("window/logMessage", map[string]any{"type": 4, "message": what + " answered " + text})
}

func (s *server) ask(method string, params any) {
	s.nextReq++
	id := fmt.Sprintf("s%d", s.nextReq)
	s.asked[strconv.Quote(id)] = method
	s.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
}

// changed reacts to new text of a document; edited is false for the text it was opened with.
func (s *server) changed(uri, text string, edited bool) {
	if edited && strings.Contains(text, "BLOAT") && len(s.bloat) == 0 {
		// Random bytes: macOS compresses pages of zeros, which would keep the
		// resident size under the test's limit.
		b := make([]byte, 300<<20)
		_, _ = rand.NewChaCha8([32]byte{1}).Read(b)
		s.bloat = append(s.bloat, b)
		// Untouched pages are paged out within a second when the machine is busy
		// (the whole browser suite at once), so keep writing to every page.
		go func() {
			for {
				for i := 0; i < len(b); i += 4096 {
					b[i]++
				}
				time.Sleep(50 * time.Millisecond)
			}
		}()
	}
	if strings.Contains(text, "CRASH") {
		fmt.Fprintln(os.Stderr, "fakelsp: crashing now, Authorization: Bearer abc123def456")
		os.Exit(3)
	}
	if strings.Contains(text, "ORPHAN") {
		child := exec.Command("sleep", "30")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		_ = child.Start()
		os.Exit(3)
	}
	if strings.Contains(text, "SERVERREQ") {
		s.ask("workspace/configuration", map[string]any{"items": []any{map[string]string{"section": "a"}, map[string]string{"section": "b"}}})
		s.ask("client/registerCapability", map[string]any{"registrations": []any{}})
		s.ask("window/workDoneProgress/create", map[string]any{"token": "t1"})
		s.ask("workspace/applyEdit", map[string]any{"edit": map[string]any{}})
		s.ask("fake/unknownRequest", map[string]any{})
	}
	s.publish(uri)
}

// formatting answers "format document" with one edit per line that ends in spaces or tabs: the
// trailing white space goes (and OUT_OF_RANGE below). The edits are what a real server's gofmt would be, in miniature.
func (s *server) formatting(params json.RawMessage) []map[string]any {
	var p struct{ TextDocument textDoc }
	_ = json.Unmarshal(params, &p)
	edits := []map[string]any{}
	// A document that says OUT_OF_RANGE gets an edit that ends past the end of its first line, as a buggy server would send.
	if strings.Contains(s.docs[p.TextDocument.URI], "OUT_OF_RANGE") {
		return []map[string]any{{
			"range": map[string]any{
				"start": map[string]int{"line": 0, "character": 0},
				"end":   map[string]int{"line": 0, "character": 999},
			},
			"newText": "",
		}}
	}
	for i, line := range strings.Split(s.docs[p.TextDocument.URI], "\n") {
		trimmed := strings.TrimRight(line, " \t")
		if len(trimmed) == len(line) {
			continue
		}
		edits = append(edits, map[string]any{
			"range": map[string]any{
				"start": map[string]int{"line": i, "character": len(utf16(trimmed))},
				"end":   map[string]int{"line": i, "character": len(utf16(line))},
			},
			"newText": "",
		})
	}
	return edits
}

func utf16(s string) []uint16 {
	out := make([]uint16, 0, len(s))
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
			continue
		}
		out = append(out, uint16(r))
	}
	return out
}
