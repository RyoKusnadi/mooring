package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadWithHugeLimitDoesNotCrash(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	path := filepath.Join(r.dir, "a.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.prompt(c, "readlimit "+path)
	r.waitState(c, StateIdle)
	if got := c.find(KindAgent).Text; !has(got, "two") || has(got, "one") {
		t.Fatalf("read: %s", got)
	}
}

func TestRequestPanicIsAnsweredNotFatal(t *testing.T) {
	var out bytes.Buffer
	c := &rpcConn{w: &out, h: panicHandler{}, pending: map[string]chan rpcMsg{}, done: make(chan struct{})}
	c.serve(rpcMsg{ID: json.RawMessage("1"), Method: "x"})
	if !has(out.String(), "-32603") {
		t.Fatalf("reply: %q", out.String())
	}
}

type panicHandler struct{}

func (panicHandler) notification(string, json.RawMessage) {}
func (panicHandler) request(context.Context, string, json.RawMessage) (any, error) {
	panic("boom")
}

func TestAgentClosingItsOutputStopsTheChat(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	c.mu.Lock()
	pid := c.cur.p.cmd.Process.Pid
	c.mu.Unlock()
	r.prompt(c, "hangup") // asks for permission, then closes its output and stays alive
	waitFor(t, "permission request", func() bool { return c.Info().Attention })
	r.waitState(c, StateStopped)
	waitFor(t, "process gone", func() bool { return !alive(pid) })
	if c.Info().Attention {
		t.Fatal("permission still waiting")
	}
}

func TestAskingModeNoticeWhenItCannotBeSet(t *testing.T) {
	for _, env := range []string{"FAKE_MODES=nomatch", "FAKE_MODES=failset"} {
		r := newRig(t, env)
		c := r.start("c1")
		n := c.find(KindNotice)
		if n == nil || !has(n.Text, "did not switch to asking before it acts") || !has(n.Text, "Fake") {
			t.Fatalf("%s: notice %+v", env, n)
		}
	}
	r := newRig(t, "FAKE_MODES=none")
	if c := r.start("c1"); c.find(KindNotice) != nil {
		t.Fatal("notice for an agent without modes")
	}
	r = newRig(t)
	if c := r.start("c1"); c.find(KindNotice) != nil || c.Info().Mode != "default" {
		t.Fatalf("normal start: %+v", c.Info())
	}
}

func TestAskingModeAppliedOnResumedSession(t *testing.T) {
	r := newRig(t, "FAKE_LOAD=1")
	c := r.start("c1")
	r.prompt(c, "hello")
	r.waitState(c, StateIdle)
	r.m.Shutdown()
	r.boot()
	c = r.m.get("c1")
	if _, err := r.run("chat.resume", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
	if c.Info().Mode != "default" {
		t.Fatalf("mode after resume: %q", c.Info().Mode)
	}
}

func TestRestartedAgentKeepsEarlierToolMessages(t *testing.T) {
	r := newRig(t, "FAKE_LOAD=1")
	c := r.start("c1")
	for i := 0; i < 2; i++ {
		r.prompt(c, "ask")
		waitFor(t, "permission", func() bool { return c.Info().Attention })
		perm := c.messages()
		id := perm[len(perm)-1].RequestID
		if _, err := r.run("chat.permission", map[string]any{"id": "c1", "requestId": id, "optionId": "allow-once"}); err != nil {
			t.Fatal(err)
		}
		r.waitState(c, StateIdle)
		if i == 0 {
			_, _ = r.run("chat.close", map[string]any{"id": "c1"})
			_, _ = r.run("chat.resume", map[string]any{"id": "c1"})
			r.waitState(c, StateIdle)
		}
	}
	tools := 0
	for _, m := range c.messages() {
		if m.Kind == KindTool {
			tools++
		}
	}
	if tools != 2 {
		t.Fatalf("%d tool messages, want 2", tools)
	}
}

func TestPermissionDuringCancelIsAnsweredCancelled(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	r.prompt(c, "asklate")
	waitFor(t, "running", func() bool { return c.Info().State == StateRunning })
	_, _ = r.run("chat.cancel", map[string]any{"id": "c1"})
	r.waitState(c, StateIdle)
	if c.Info().Attention {
		t.Fatal("permission left waiting")
	}
	if p := c.find(KindPermission); p != nil && p.State == PermPending {
		t.Fatalf("pending: %+v", p)
	}
}

func TestStopAgentIgnoresAReplacedRun(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	c.mu.Lock()
	old := c.cur
	c.mu.Unlock()
	c.stopAgent(&run{c: c}, StateStopped, nil) // some other run
	if c.Info().State != StateIdle {
		t.Fatalf("a foreign run stopped the chat: %s", c.Info().State)
	}
	c.stopAgent(old, StateStopped, nil)
	if c.Info().State != StateStopped {
		t.Fatal("own run did not stop")
	}
}
