package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// client is a socket speaking JSON-RPC to the relay.
type client struct {
	t      *testing.T
	ws     *websocket.Conn
	mu     sync.Mutex
	inbox  []map[string]any
	closed chan websocket.CloseError
}

func (e *env) dial(id, folder string) (*client, *http.Response, error) {
	url := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/api/lsp/" + id + "?project=p1&folder=" + folder
	return e.dialURL(url)
}

func (e *env) dialURL(url string) (*client, *http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, resp, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, resp, err
	}
	ws.SetReadLimit(32 << 20)
	c := &client{t: e.t, ws: ws, closed: make(chan websocket.CloseError, 1)}
	go c.readLoop()
	e.t.Cleanup(func() { _ = ws.CloseNow() })
	return c, resp, nil
}

func (e *env) connect(id, folder string) *client {
	e.t.Helper()
	c, resp, err := e.dial(id, folder)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		e.t.Fatalf("dial: %v (status %d)", err, status)
	}
	return c
}

func (c *client) readLoop() {
	for {
		_, data, err := c.ws.Read(context.Background())
		if err != nil {
			var ce websocket.CloseError
			_ = errors.As(err, &ce)
			c.closed <- ce
			return
		}
		var m map[string]any
		if json.Unmarshal(data, &m) == nil {
			c.mu.Lock()
			c.inbox = append(c.inbox, m)
			c.mu.Unlock()
		}
	}
}

func (c *client) send(m map[string]any) {
	c.t.Helper()
	m["jsonrpc"] = "2.0"
	raw, _ := json.Marshal(m)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, raw); err != nil {
		c.t.Fatalf("send: %v", err)
	}
}

func (c *client) notify(method string, params any) {
	c.t.Helper()
	c.send(map[string]any{"method": method, "params": params})
}

// call sends a request with the given id and returns the whole answer.
func (c *client) call(id int, method string, params any) map[string]any {
	c.t.Helper()
	c.send(map[string]any{"id": id, "method": method, "params": params})
	return c.wait(func(m map[string]any) bool { return m["method"] == nil && m["id"] == float64(id) })
}

func (c *client) wait(pred func(map[string]any) bool) map[string]any {
	c.t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		for k, m := range c.inbox {
			if pred(m) {
				c.inbox = slices.Delete(c.inbox, k, k+1)
				c.mu.Unlock()
				return m
			}
		}
		c.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t.Fatalf("no matching message; inbox: %v", c.inbox)
	return nil
}

func (c *client) waitClose() websocket.CloseError {
	c.t.Helper()
	select {
	case ce := <-c.closed:
		return ce
	case <-time.After(8 * time.Second):
		c.t.Fatal("the socket did not close")
		return websocket.CloseError{}
	}
}

// handshake does initialize and initialized.
func (c *client) handshake(id int, params map[string]any) map[string]any {
	c.t.Helper()
	res := c.call(id, "initialize", params)
	c.notify("initialized", map[string]any{})
	return res
}

func (c *client) open(uri, text string) {
	c.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "go", "version": 1, "text": text}})
}

func (c *client) change(uri, text string) {
	c.notify("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2},
		"contentChanges": []any{map[string]any{"text": text}}})
}

func (c *client) diagnostics(uri string, pred func([]any) bool) []any {
	c.t.Helper()
	m := c.wait(func(m map[string]any) bool {
		if m["method"] != "textDocument/publishDiagnostics" {
			return false
		}
		p, _ := m["params"].(map[string]any)
		d, _ := p["diagnostics"].([]any)
		return p["uri"] == uri && pred(d)
	})
	return m["params"].(map[string]any)["diagnostics"].([]any)
}

// state asks the stand-in what it saw.
func (c *client) state(id int) (seen []string, open []string, init map[string]any) {
	c.t.Helper()
	res := c.call(id, "fake/state", map[string]any{})
	r, _ := res["result"].(map[string]any)
	for _, s := range r["seen"].([]any) {
		seen = append(seen, s.(string))
	}
	for _, s := range r["open"].([]any) {
		open = append(open, s.(string))
	}
	init, _ = r["initialize"].(map[string]any)
	return
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func waitDead(t *testing.T, pid int) {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if !alive(pid) {
			return
		}
	}
	t.Fatalf("process %d is still running", pid)
}

func (e *env) pidOf(id string) int {
	e.t.Helper()
	for _, r := range e.svc.running() {
		if r.ID == id {
			return r.PID
		}
	}
	e.t.Fatalf("server %s is not running", id)
	return 0
}
