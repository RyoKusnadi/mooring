// Package fakeagent is a stand-in ACP agent for tests. The test binary runs it
// when asked (see Run), so no separate program has to be built.
//
// Behaviour follows the prompt text: "hello" streams two chunks and usage,
// "ask" asks for permission, "long" streams until cancelled, "readfile PATH"
// reads through the client, "SHOWCASE" streams markdown, an edit with a diff and a command (showcase.go), "spawn" starts a grandchild process and reports its
// pid. Environment: FAKE_LOAD=1 or FAKE_RESUME=1 advertise those, FAKE_STATE
// names a folder where sessions are kept, FAKE_AUTH=prompt fails every prompt
// as "not signed in", and FAKE_AUTH=missing also announces it when a session opens.
// FAKE_CAPS=list,close,delete,fork,systemPrompt,additionalDirectories,mcpAcp,audio advertises those
// (caps.go); FAKE_STREAM_TOOL=1 makes "STREAM" run a command that streams five outputs (stream.go).
// FAKE_CLOSE=close or delete advertises that session method and FAKE_CALLS names a file
// that gets one line per such call. A session whose folder is named "jobs" answers
// "Fake title", or "fake: commit subject" for a commit message prompt. "BENCH task [on|off]"
// is the bench's stand-in turn.
// FAKE_CONFIG names an account folder: a prompt fails as "not signed in" unless that
// folder has a file "signed-in". "RATE [window [percent]]" reports one rate-limit
// window, "LIMIT" reports a refusing window and fails the turn for a limit, and
// "RATE x 100" reports a refusing window and still answers, and "LIMITTEXT" fails the turn with the limit error alone.
package fakeagent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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

type agent struct {
	out io.Writer
	wmu sync.Mutex

	mu       sync.Mutex
	next     int
	waiting  map[string]chan msg
	cancelCh map[string]chan struct{} // by session id
	sessions int
	servers  map[string][]server // tool servers by session id
	cwds     map[string]string   // working folder by session id
}

// Run serves one ACP connection until in ends.
func Run(in io.Reader, out io.Writer) {
	a := &agent{out: out, waiting: map[string]chan msg{}, cancelCh: map[string]chan struct{}{}, servers: map[string][]server{}, cwds: map[string]string{}}
	br := bufio.NewReaderSize(in, 1<<20)
	for {
		line, err := br.ReadBytes('\n')
		var m msg
		if len(line) > 0 && json.Unmarshal(line, &m) == nil {
			a.dispatch(m)
		}
		if err != nil {
			return
		}
	}
}

func (a *agent) send(v any) {
	raw, _ := json.Marshal(v)
	a.wmu.Lock()
	defer a.wmu.Unlock()
	_, _ = a.out.Write(append(raw, '\n'))
}

func (a *agent) reply(id json.RawMessage, result any) {
	a.send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (a *agent) fail(id json.RawMessage, code int, text string) {
	a.send(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": text}})
}

func (a *agent) update(sid string, u map[string]any) {
	a.send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": sid, "update": u}})
}

func (a *agent) chunk(sid, text string) {
	a.update(sid, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": text}})
	a.remember(sid, "agent", text)
}

// call sends a request to the client and waits for its answer.
func (a *agent) call(method string, params any) msg {
	a.mu.Lock()
	a.next++
	id := fmt.Sprint(a.next)
	ch := make(chan msg, 1)
	a.waiting[id] = ch
	a.mu.Unlock()
	a.send(map[string]any{"jsonrpc": "2.0", "id": json.Number(id), "method": method, "params": params})
	return <-ch
}

func (a *agent) dispatch(m msg) {
	if m.Method == "" {
		a.mu.Lock()
		ch := a.waiting[string(m.ID)]
		delete(a.waiting, string(m.ID))
		a.mu.Unlock()
		if ch != nil {
			ch <- m
		}
		return
	}
	var p struct {
		SessionID string   `json:"sessionId"`
		ModeID    string   `json:"modeId"`
		Servers   []server `json:"mcpServers"`
		Value     string   `json:"value"`
		Prompt    []block  `json:"prompt"`
		Cwd       string   `json:"cwd"`
	}
	_ = json.Unmarshal(m.Params, &p)
	switch m.Method {
	case "initialize":
		a.reply(m.ID, initializeResult())
	case "session/new":
		a.mu.Lock()
		a.sessions++
		sid := fmt.Sprintf("s%d-%d", os.Getpid(), a.sessions)
		a.mu.Unlock()
		a.open(sid)
		a.setServers(sid, p.Servers)
		a.mu.Lock()
		a.cwds[sid] = p.Cwd
		a.mu.Unlock()
		a.reply(m.ID, a.sessionBody(sid))
	case "session/close", "session/delete":
		a.logCall(m.Method)
		a.reply(m.ID, map[string]any{})
	case "session/load", "session/resume":
		if _, err := os.Stat(a.file(p.SessionID)); err != nil {
			a.fail(m.ID, -32002, "no such session")
			return
		}
		a.open(p.SessionID)
		a.setServers(p.SessionID, p.Servers)
		if m.Method == "session/load" {
			a.replay(p.SessionID)
		}
		a.reply(m.ID, a.sessionBody(p.SessionID))
	case "session/set_mode":
		if os.Getenv("FAKE_MODES") == "failset" {
			a.fail(m.ID, -32603, "cannot switch")
			return
		}
		a.reply(m.ID, map[string]any{})
	case "session/set_config_option":
		a.logCall(m.Method + ":" + p.Value)
		a.reply(m.ID, map[string]any{"configOptions": []any{modelOption(p.Value)}})
	case "session/cancel":
		a.mu.Lock()
		if ch := a.cancelCh[p.SessionID]; ch != nil {
			close(ch)
			delete(a.cancelCh, p.SessionID)
		}
		a.mu.Unlock()
	case "session/prompt":
		text := ""
		for _, b := range p.Prompt {
			if b.Type == "text" {
				text = b.Text
				break
			}
		}
		// The cancel channel is registered before the reply runs, so a cancel read
		// right after the prompt finds it.
		cancel := make(chan struct{})
		a.mu.Lock()
		a.cancelCh[p.SessionID] = cancel
		a.mu.Unlock()
		go a.prompt(m.ID, p.SessionID, text, p.Prompt, cancel)
	}
}

func (a *agent) sessionBody(sid string) map[string]any {
	if os.Getenv("FAKE_AUTH") == "missing" {
		a.send(map[string]any{"jsonrpc": "2.0", "method": "auth/status_update", "params": map[string]string{"state": "unauthenticated"}})
	}
	body := map[string]any{
		"sessionId": sid,
		"modes": map[string]any{"currentModeId": "auto", "availableModes": []map[string]string{
			{"id": "default", "name": "Manual"}, {"id": "auto", "name": "Auto"}, {"id": "plan", "name": "Plan"}}},
		"configOptions": []any{modelOption("fast")},
	}
	switch os.Getenv("FAKE_MODES") {
	case "none":
		delete(body, "modes")
	case "nomatch":
		body["modes"] = map[string]any{"currentModeId": "auto", "availableModes": []map[string]string{
			{"id": "auto", "name": "Auto"}, {"id": "plan", "name": "Plan"}}}
	}
	return body
}

func (a *agent) prompt(id json.RawMessage, sid, text string, blocks []block, cancel chan struct{}) {
	a.remember(sid, "user", text)
	if auth := os.Getenv("FAKE_AUTH"); auth == "missing" || auth == "prompt" {
		a.fail(id, -32000, "Authentication required")
		return
	}
	if a.signedOut() {
		a.failAuth(id)
		return
	}
	stop := "end_turn"
	usage := map[string]int{"inputTokens": 1000, "outputTokens": 50, "totalTokens": 1050}
	job := a.isJob(sid)
	switch {
	case job:
		a.chunk(sid, jobReply(text))
	case text == "LIMIT" || text == "LIMITTEXT":
		a.limit(id, sid, text)
		return
	case text == "RATE" || strings.HasPrefix(text, "RATE "):
		a.rate(sid, text)
	case strings.HasPrefix(text, "TERM "), strings.HasPrefix(text, "TERMKILL "):
		a.termTurn(sid, text)
	case strings.HasPrefix(text, "BENCH "):
		usage = a.benchTurn(sid, text)
	case strings.HasPrefix(text, "Summarise this chat"):
		a.chunk(sid, "Goal: build it. Decisions: keep it small. Next step: ship.")
	case text == "SLOW":
		select {}
	case strings.Contains(text, "BLOCKS"):
		a.chunk(sid, describeBlocks(blocks))
	case text == "hello":
		a.chunk(sid, "Hello ")
		time.Sleep(20 * time.Millisecond)
		a.chunk(sid, "world")
		a.update(sid, map[string]any{"sessionUpdate": "usage_update", "used": 1200, "size": 200000, "cost": map[string]any{"amount": 0.01, "currency": "USD"}})
	case text == "TOOLS":
		a.chunk(sid, a.toolsAnswer(sid))
	case text == "SHOWCASE":
		a.showcase(sid)
	case text == "STREAM" && os.Getenv("FAKE_STREAM_TOOL") == "1":
		a.streamTool(sid)
	case text == "ask":
		stop = a.ask(sid)
	case text == demoPrompt:
		stop = a.demoTurn(sid)
	case text == "long":
		stop = a.long(sid, cancel)
	case strings.HasPrefix(text, "readfile "):
		res := a.call("fs/read_text_file", map[string]any{"sessionId": sid, "path": strings.TrimPrefix(text, "readfile ")})
		if len(res.Error) > 0 {
			a.chunk(sid, "error:"+string(res.Error))
		} else {
			a.chunk(sid, "read:"+string(res.Result))
		}
	case strings.HasPrefix(text, "readlimit "):
		res := a.call("fs/read_text_file", map[string]any{"sessionId": sid, "path": strings.TrimPrefix(text, "readlimit "), "line": 2, "limit": json.Number("9223372036854775807")})
		a.chunk(sid, "read:"+string(res.Result)+string(res.Error))
	case text == "hangup":
		a.send(map[string]any{"jsonrpc": "2.0", "id": 900, "method": "session/request_permission", "params": map[string]any{
			"sessionId": sid, "toolCall": map[string]any{"toolCallId": "t9", "title": "x", "kind": "execute"},
			"options": []map[string]string{{"optionId": "allow-once", "name": "Allow", "kind": "allow_once"}}}})
		time.Sleep(200 * time.Millisecond)
		if f, ok := a.out.(*os.File); ok {
			_ = f.Close()
		}
		select {}
	case text == "asklate":
		<-cancel
		stop = a.ask(sid)
	case text == "spawn":
		cmd := exec.Command("sleep", "300")
		_ = cmd.Start()
		a.chunk(sid, fmt.Sprintf("pid:%d", cmd.Process.Pid))
	default:
		a.chunk(sid, "echo: "+text)
	}
	if !job && os.Getenv("FAKE_TITLE") == "1" && os.Getenv("FAKE_NOTITLE") != "1" {
		a.update(sid, map[string]any{"sessionUpdate": "session_info_update", "title": "Fake chat title"})
	}
	a.reply(id, map[string]any{"stopReason": stop, "usage": usage})
}
