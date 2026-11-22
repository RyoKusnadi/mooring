package lsp

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFraming(t *testing.T) {
	var buf bytes.Buffer
	for _, body := range []string{`{"a":1}`, `{"b":"ü"}`} {
		if err := writeMessage(&buf, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	// Other headers and a lower-case name are accepted.
	buf.WriteString("content-type: x\r\ncontent-length: 2\r\n\r\n{}")
	r := bufio.NewReader(&buf)
	for _, want := range []string{`{"a":1}`, `{"b":"ü"}`, `{}`} {
		got, err := readMessage(r)
		if err != nil || string(got) != want {
			t.Fatalf("got %q, %v; want %q", got, err, want)
		}
	}
	if _, err := readMessage(r); err == nil {
		t.Fatal("end of input should be an error")
	}
	if _, err := readMessage(bufio.NewReader(strings.NewReader("X: 1\r\n\r\n{}"))); err == nil {
		t.Fatal("a message without Content-Length should be an error")
	}
	if _, err := readMessage(bufio.NewReader(strings.NewReader("Content-Length: 99999999999\r\n\r\n"))); err == nil {
		t.Fatal("a huge Content-Length should be an error")
	}
	if _, err := readMessage(bufio.NewReader(strings.NewReader("Content-Length: 10\r\n\r\n{}"))); err == nil {
		t.Fatal("a short body should be an error")
	}
}

func TestInitializeIsCachedAndIdsAreRenumbered(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	e.allow("go")
	a, b := e.connect("go", ""), e.connect("go", "")
	root := filepath.Join(e.root)
	resA := a.handshake(1, map[string]any{"processId": nil, "rootUri": "file:///elsewhere", "initializationOptions": map[string]any{"own": 1}})
	caps := resA["result"].(map[string]any)["capabilities"].(map[string]any)
	if caps["completionProvider"] == nil {
		t.Fatalf("no completion capability: %v", resA)
	}
	// The second socket's initialize is answered without reaching the server.
	resB := b.handshake(1, map[string]any{"rootUri": "file:///other"})
	if resB["id"] != float64(1) || resB["result"] == nil {
		t.Fatalf("second initialize: %v", resB)
	}
	seen, _, init := a.state(2)
	if n := countPrefix(seen, "initialize"); n != 1 {
		t.Fatalf("server got %d initialize requests: %v", n, seen)
	}
	if n := countPrefix(seen, "initialized"); n != 1 {
		t.Fatalf("server got %d initialized notifications: %v", n, seen)
	}
	if init["rootUri"] != fileURI(root) || init["rootPath"] != root {
		t.Fatalf("root was not set to the place: %v", init)
	}
	if folders, _ := init["workspaceFolders"].([]any); len(folders) != 1 {
		t.Fatalf("workspaceFolders: %v", init["workspaceFolders"])
	}
	if opts, _ := init["initializationOptions"].(map[string]any); opts["own"] != float64(1) {
		t.Fatalf("the client's own options are lost: %v", init["initializationOptions"])
	}

	// Both sockets use id 7 for different requests; each gets its own answer.
	doc := e.write("a.go", "alpha beta")
	uri := uriOf(doc)
	a.open(uri, "alphaword beta")
	b.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uriOf(e.write("b.go", "")), "text": "gammaword"}})
	// Two sockets have no order between them: let b's open reach the server before a asks.
	b.state(3)
	a.send(map[string]any{"id": 7, "method": "textDocument/completion", "params": map[string]any{"textDocument": map[string]any{"uri": uri}}})
	b.send(map[string]any{"id": 7, "method": "textDocument/hover", "params": map[string]any{"textDocument": map[string]any{"uri": uri}}})
	ra := a.wait(func(m map[string]any) bool { return m["id"] == float64(7) })
	rb := b.wait(func(m map[string]any) bool { return m["id"] == float64(7) })
	if _, ok := ra["result"].(map[string]any)["items"]; !ok {
		t.Fatalf("socket a did not get its completion: %v", ra)
	}
	if _, ok := rb["result"].(map[string]any)["contents"]; !ok {
		t.Fatalf("socket b did not get its hover: %v", rb)
	}
	items := ra["result"].(map[string]any)["items"].([]any)
	var labels []string
	for _, it := range items {
		labels = append(labels, it.(map[string]any)["label"].(string))
	}
	for _, want := range []string{"alphaword", "gammaword", "fakeCompletion"} {
		if !slices.Contains(labels, want) {
			t.Fatalf("completion lacks %q: %v", want, labels)
		}
	}
	// Diagnostics reach every socket.
	a.change(uri, "x\nERROR here\n")
	for _, c := range []*client{a, b} {
		d := c.diagnostics(uri, func(d []any) bool { return len(d) == 1 })
		if d[0].(map[string]any)["message"] != "found ERROR" {
			t.Fatalf("diagnostic: %v", d)
		}
	}
	// shutdown from a socket is answered by the relay and never reaches the server.
	if r := b.call(9, "shutdown", nil); r["result"] != nil || r["error"] != nil {
		t.Fatalf("shutdown: %v", r)
	}
	b.notify("exit", nil)
	seen, _, _ = a.state(3)
	if slices.Contains(seen, "shutdown") || slices.Contains(seen, "exit") {
		t.Fatalf("shutdown reached the server: %v", seen)
	}
}

func countPrefix(list []string, p string) int {
	n := 0
	for _, s := range list {
		if s == p || strings.HasPrefix(s, p+" ") {
			n++
		}
	}
	return n
}

func TestServerRequestsAreAnsweredByTheRelay(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	e.allow("go")
	a, b := e.connect("go", ""), e.connect("go", "")
	a.handshake(1, map[string]any{})
	b.handshake(1, map[string]any{})
	uri := uriOf(e.write("a.go", ""))
	a.open(uri, "package a // SERVERREQ")
	want := map[string]string{
		"workspace/configuration":        "answered [null,null]",
		"client/registerCapability":      "answered null",
		"window/workDoneProgress/create": "answered null",
		"workspace/applyEdit":            `answered {"applied":false}`,
		"fake/unknownRequest":            "-32601",
	}
	for _, c := range []*client{a, b} {
		for method, text := range want {
			m := c.wait(func(m map[string]any) bool {
				p, _ := m["params"].(map[string]any)
				msg, _ := p["message"].(string)
				return m["method"] == "window/logMessage" && strings.HasPrefix(msg, method+" ") && strings.Contains(msg, text)
			})
			_ = m
		}
		c.mu.Lock()
		for _, m := range c.inbox {
			if m["method"] != nil && m["id"] != nil {
				t.Errorf("a server request reached a socket: %v", m)
			}
		}
		c.mu.Unlock()
	}
}

func TestDidOpenOwnership(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	e.allow("go")
	a, b := e.connect("go", ""), e.connect("go", "")
	a.handshake(1, map[string]any{})
	b.handshake(1, map[string]any{})
	uri := uriOf(e.write("a.go", ""))
	// Two sockets have no order between them, so each step waits for a round
	// trip through the server before the other socket goes on.
	a.open(uri, "first")
	a.state(10)
	b.open(uri, "second")
	b.state(11)
	a.change(uri, "from a, dropped")
	a.notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": uri}}) // not the owner: dropped
	a.state(12)
	b.change(uri, "from b")
	seen, open, _ := b.state(2)
	want := []string{"textDocument/didOpen " + uri, "textDocument/didClose " + uri, "textDocument/didOpen " + uri, "textDocument/didChange " + uri}
	var got []string
	for _, s := range seen {
		if strings.HasPrefix(s, "textDocument/") {
			got = append(got, s)
		}
	}
	if !slices.Equal(got, want) || !slices.Equal(open, []string{uri}) {
		t.Fatalf("server saw %v (open %v), want %v", got, open, want)
	}
	// When the owner's socket closes, its document is closed for the server.
	_ = b.ws.CloseNow()
	for range 200 {
		if _, open, _ = a.state(3); len(open) == 0 {
			return
		}
	}
	t.Fatalf("the document is still open after its socket closed: %v", open)
}

func TestURIsOutsideThePlaceAreRejected(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	e.allow("go")
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(e.root, "link")); err != nil {
		t.Fatal(err)
	}
	e.write("sub/in.go", "")
	c := e.connect("go", "sub")
	c.handshake(1, map[string]any{})
	for i, uri := range []string{
		"file:///etc/passwd",
		uriOf(filepath.Join(e.root, "a.go")), // inside the project, outside the place
		uriOf(filepath.Join(e.root, "link", "secret.go")),
		uriOf(filepath.Join(e.root, "sub", "..", "a.go")),
		"http://example.com/x.go",
		"file://remote-host" + filepath.Join(e.root, "sub", "in.go"),
	} {
		r := c.call(10+i, "textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": uri}})
		errObj, _ := r["error"].(map[string]any)
		if errObj["code"] != float64(-32602) {
			t.Errorf("%s: got %v, want error -32602", uri, r)
		}
		c.open(uri, "ignored")
	}
	good := uriOf(filepath.Join(e.root, "sub", "in.go"))
	c.open(good, "fine")
	seen, open, init := c.state(30)
	if !slices.Equal(open, []string{good}) {
		t.Fatalf("open documents %v; seen %v", open, seen)
	}
	if init["rootUri"] != fileURI(filepath.Join(e.root, "sub")) {
		t.Fatalf("rootUri for the folder: %v", init["rootUri"])
	}
	if r := c.call(31, "textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": good}}); r["error"] != nil {
		t.Fatalf("a file inside the place was refused: %v", r)
	}
}
