package lsp

import (
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestIdleStop(t *testing.T) {
	e := newEnv(t, []string{"gopls"}, func(c *Config) { c.Idle = 300 * time.Millisecond })
	e.allow("go")
	c := e.connect("go", "")
	c.handshake(1, map[string]any{})
	pid := e.pidOf("go")
	e.pub.wait(t, "lsp.started", nil)
	time.Sleep(500 * time.Millisecond)
	if !alive(pid) {
		t.Fatal("the server stopped while a socket was connected")
	}
	_ = c.ws.CloseNow()
	ev := e.pub.wait(t, "lsp.stopped", nil)
	if ev["reason"] != "idle" || ev["projectId"] != "p1" || ev["id"] != "go" {
		t.Fatalf("stopped event: %v", ev)
	}
	waitDead(t, pid)
	if n := len(e.svc.running()); n != 0 {
		t.Fatalf("%d servers still listed", n)
	}
}

func TestANewSocketCancelsTheIdleTimer(t *testing.T) {
	e := newEnv(t, []string{"gopls"}, func(c *Config) { c.Idle = 400 * time.Millisecond })
	e.allow("go")
	a := e.connect("go", "")
	a.handshake(1, map[string]any{})
	pid := e.pidOf("go")
	_ = a.ws.CloseNow()
	time.Sleep(150 * time.Millisecond)
	b := e.connect("go", "")
	time.Sleep(700 * time.Millisecond)
	if !alive(pid) || e.pidOf("go") != pid {
		t.Fatal("the server stopped although a socket joined before the idle time ended")
	}
	b.handshake(1, map[string]any{}) // answered from the cache
}

func TestMemoryRestart(t *testing.T) {
	e := newEnv(t, []string{"gopls"}, func(c *Config) { c.MemoryMB = 150; c.MemoryEvery = 100 * time.Millisecond })
	e.allow("go")
	a, b := e.connect("go", ""), e.connect("go", "")
	a.handshake(1, map[string]any{})
	b.handshake(1, map[string]any{})
	pid := e.pidOf("go")
	uri := uriOf(e.write("a.go", ""))
	a.open(uri, "")
	a.change(uri, "BLOAT")
	for _, c := range []*client{a, b} {
		if ce := c.waitClose(); ce.Code != 4001 || ce.Reason != "restarted" {
			t.Fatalf("close: %+v", ce)
		}
	}
	ev := e.pub.wait(t, "lsp.restarted", nil)
	if ev["reason"] != "memory" || ev["limitMb"] != float64(150) || ev["mb"].(float64) <= 150 {
		t.Fatalf("restarted event: %v", ev)
	}
	waitDead(t, pid)
	// The next socket starts a fresh process.
	c := e.connect("go", "")
	c.handshake(1, map[string]any{})
	if np := e.pidOf("go"); np == pid {
		t.Fatal("the same process is running")
	}
}

func TestCrash(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	e.allow("go")
	c := e.connect("go", "")
	c.handshake(1, map[string]any{})
	pid := e.pidOf("go")
	c.open(uriOf(e.write("a.go", "")), "CRASH")
	if ce := c.waitClose(); ce.Code != 4002 {
		t.Fatalf("close: %+v", ce)
	}
	ev := e.pub.wait(t, "lsp.stopped", nil)
	stderr, _ := ev["stderr"].(string)
	if ev["reason"] != "crashed" || !strings.Contains(stderr, "crashing now") {
		t.Fatalf("stopped event: %v", ev)
	}
	if strings.Contains(stderr, "abc123def456") {
		t.Fatalf("a credential is in the stderr tail: %q", stderr)
	}
	waitDead(t, pid)
	if len(e.svc.running()) != 0 {
		t.Fatal("a crashed server is still listed")
	}
	c2 := e.connect("go", "") // the next socket starts a new one
	c2.handshake(1, map[string]any{})
}

func TestStopCommandProjectRemovalAndShutdown(t *testing.T) {
	e := newEnv(t, []string{"gopls", "typescript-language-server"})
	e.allow("go")
	e.allow("typescript")
	a := e.connect("go", "")
	a.handshake(1, map[string]any{})
	b := e.connect("typescript", "")
	b.handshake(1, map[string]any{})
	e.write("sub/x.go", "")
	c := e.connect("go", "sub")
	c.handshake(1, map[string]any{})
	pids := map[string]int{}
	for _, r := range e.svc.running() {
		pids[r.ID+"/"+r.Folder] = r.PID
	}
	if len(pids) != 3 {
		t.Fatalf("running: %v", pids)
	}
	e.svc.stopKey(instKey{"p1", "sub", "go"}, "closed")
	waitDead(t, pids["go/sub"])
	if ce := c.waitClose(); ce.Code != websocket.StatusGoingAway {
		t.Fatalf("close: %+v", ce)
	}
	ev := e.pub.wait(t, "lsp.stopped", func(m map[string]any) bool { return m["folder"] == "sub" })
	if ev["reason"] != "closed" {
		t.Fatalf("stopped: %v", ev)
	}
	e.svc.CloseProject("p1")
	waitDead(t, pids["go/"])
	waitDead(t, pids["typescript/"])
	d := e.connect("go", "")
	d.handshake(1, map[string]any{})
	pid := e.pidOf("go")
	e.svc.Shutdown()
	waitDead(t, pid)
	if _, _, err := e.dial("go", ""); err == nil {
		t.Fatal("a socket was accepted after shutdown")
	}
}

func TestRefusedBeforeUpgrade(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	check := func(url string, status int, code string) {
		t.Helper()
		_, resp, err := e.dialURL(url)
		if err == nil || resp == nil || resp.StatusCode != status {
			t.Fatalf("%s: status %v, error %v; want %d", url, resp, err, status)
		}
		_ = code
	}
	base := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/api/lsp/"
	check(base+"nope?project=p1", 404, "NOT_FOUND")
	check(base+"go?project=nope", 404, "NOT_FOUND")
	check(base+"go?project=p1&folder=..", 400, "OUTSIDE_PROJECT")
	check(base+"go?project=p1&folder=missing", 404, "NOT_FOUND")
	check(base+"go?project=p1", 409, "NEEDS_CONSENT") // found, not allowed
	check(base+"python?project=p1", 409, "NOT_INSTALLED")
}
