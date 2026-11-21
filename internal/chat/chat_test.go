package chat

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartPromptStreamsReply(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	info := c.Info()
	if info.Mode != "default" {
		t.Fatalf("mode = %q, want the asking mode", info.Mode)
	}
	if info.Model != "fast" || len(info.Models) != 2 {
		t.Fatalf("model %q of %v", info.Model, info.Models)
	}
	r.prompt(c, "hello")
	r.waitState(c, StateIdle)
	msgs := c.messages()
	if len(msgs) != 2 || msgs[0].Kind != KindUser || msgs[1].Kind != KindAgent || msgs[1].Text != "Hello world" {
		t.Fatalf("messages: %+v", msgs)
	}
	if u := c.Info().Usage; u == nil || u.Used != 1200 || u.Size != 200000 || u.Currency != "USD" {
		t.Fatalf("usage: %+v", u)
	}
	if c.Info().Title != "hello" {
		t.Fatalf("title %q", c.Info().Title)
	}
	if r.rec.count("chat.added") != 1 || r.rec.count("chat.changed") == 0 {
		t.Fatalf("events: %v", r.rec.kinds)
	}
	if _, err := r.run("chat.setModel", map[string]any{"id": "c1", "value": "deep"}); err != nil || c.Info().Model != "deep" {
		t.Fatalf("setModel: %v, model %q", err, c.Info().Model)
	}
	if _, err := r.run("chat.setModel", map[string]any{"id": "c1", "value": "nope"}); err == nil {
		t.Fatal("unknown model accepted")
	}
	if _, err := r.run("chat.prompt", map[string]any{"id": "c1", "text": "   "}); err == nil {
		t.Fatal("empty prompt accepted")
	}
}

func TestPermissionAnswerWinsOnce(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	r.prompt(c, "ask")
	waitFor(t, "permission request", func() bool { return c.find(KindPermission) != nil })
	perm := c.find(KindPermission)
	if perm.State != PermPending || perm.ToolKind != "execute" || len(perm.Options) != 2 || !c.Info().Attention || c.Info().LimitOnly {
		t.Fatalf("permission: %+v attention=%v", perm, c.Info().Attention)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	for _, opt := range []string{"allow-once", "reject"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := r.run("chat.permission", map[string]any{"id": "c1", "requestId": perm.RequestID, "optionId": opt})
			if err != nil {
				t.Errorf("answer: %v", err)
				return
			}
			if v.(map[string]any)["accepted"].(bool) {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if accepted != 1 {
		t.Fatalf("%d answers accepted, want 1", accepted)
	}
	r.waitState(c, StateIdle)
	done := c.find(KindPermission)
	if done.State != PermAnswered || done.Chosen == "" || c.Info().Attention {
		t.Fatalf("after answer: %+v", done)
	}
	tool := c.find(KindTool)
	want := map[string]string{"allow-once": "completed", "reject": "failed"}[done.Chosen]
	if tool == nil || tool.Status != want {
		t.Fatalf("tool %+v, chosen %s", tool, done.Chosen)
	}
	// Answering again changes nothing and says what was chosen.
	v, err := r.run("chat.permission", map[string]any{"id": "c1", "requestId": perm.RequestID, "optionId": "reject"})
	if err != nil || v.(map[string]any)["accepted"].(bool) || v.(map[string]any)["chosen"] != done.Chosen {
		t.Fatalf("late answer: %v %v", v, err)
	}
}

func TestCancelLongReplyAndPendingPermission(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	r.prompt(c, "long")
	waitFor(t, "streamed text", func() bool { return c.find(KindAgent) != nil })
	if _, err := r.run("chat.prompt", map[string]any{"id": "c1", "text": "again"}); err == nil || !has(err.Error(), "BUSY") {
		t.Fatalf("second prompt while running: %v", err)
	}
	_, _ = r.run("chat.cancel", map[string]any{"id": "c1"})
	r.waitState(c, StateIdle)
	r.prompt(c, "hello") // the same process takes the next prompt
	r.waitState(c, StateIdle)

	r.prompt(c, "ask")
	waitFor(t, "permission request", func() bool { return c.Info().Attention })
	_, _ = r.run("chat.cancel", map[string]any{"id": "c1"})
	r.waitState(c, StateIdle)
	var last Message
	for _, m := range c.messages() {
		if m.Kind == KindPermission {
			last = m
		}
	}
	if last.State != PermCancelled || c.Info().Attention {
		t.Fatalf("permission after cancel: %+v", last)
	}
}

// Claude's adapter says "unauthenticated" at start even when signed in, so the notice
// alone leaves the chat idle; the first prompt's -32000 error marks the missing login.
func TestMissingLoginIsReported(t *testing.T) {
	r := newRig(t, "FAKE_AUTH=missing")
	c := r.start("c1")
	if s := c.Info().State; s != StateIdle {
		t.Fatalf("state after the notice: %s", s)
	}
	r.prompt(c, "hi")
	r.waitState(c, StateNeedsLogin)
	if e := c.Info().Error; e == nil || e.Code != "AUTH_REQUIRED" {
		t.Fatalf("error: %+v", e)
	}
	if _, err := r.run("chat.prompt", map[string]any{"id": "c1", "text": "hi"}); err == nil || !has(err.Error(), "AUTH_REQUIRED") {
		t.Fatalf("prompt: %v", err)
	}
}

func TestMissingLoginFoundOnFirstPrompt(t *testing.T) {
	r := newRig(t, "FAKE_AUTH=prompt") // no notification, only the -32000 error
	c := r.start("c1")
	r.prompt(c, "hi")
	r.waitState(c, StateNeedsLogin)
	if e := c.Info().Error; e == nil || e.Code != "AUTH_REQUIRED" {
		t.Fatalf("error: %+v", e)
	}
}

func TestResumeAfterRestartLoadsTheSession(t *testing.T) {
	r := newRig(t, "FAKE_LOAD=1")
	c := r.start("c1")
	r.prompt(c, "hello")
	r.waitState(c, StateIdle)
	r.m.Shutdown()

	r.boot() // a new server on the same database
	c = r.m.get("c1")
	if c == nil || c.Info().State != StateStopped || c.Info().Messages != 2 {
		t.Fatalf("after restart: %+v", c)
	}
	if _, err := r.run("chat.prompt", map[string]any{"id": "c1", "text": "x"}); err == nil || !has(err.Error(), "NOT_RUNNING") {
		t.Fatalf("prompt on stopped chat: %v", err)
	}
	if _, err := r.run("chat.resume", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
	r.prompt(c, "again")
	r.waitState(c, StateIdle)
	var texts []string
	for _, m := range c.messages() {
		texts = append(texts, m.Kind+":"+m.Text)
	}
	got := strings.Join(texts, "|")
	if got != "user:hello|agent:Hello world|user:again|agent:echo: again" {
		t.Fatalf("history after resume (replay must not be stored twice): %s", got)
	}
}

func TestResumeWithoutLoadStartsFreshWithNotice(t *testing.T) {
	r := newRig(t) // the stand-in cannot load sessions
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
	msgs := c.messages()
	if len(msgs) != 3 || msgs[2].Kind != KindNotice || msgs[0].Text != "hello" {
		t.Fatalf("messages: %+v", msgs)
	}
}

func TestSavedChatsListedAndPermissionNotLeftPending(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	r.prompt(c, "ask")
	waitFor(t, "permission request", func() bool { return c.Info().Attention })
	r.m.Shutdown() // flushes the open request, then the agent is gone
	r.boot()
	chats, agents := r.m.Snapshot()
	if len(chats["p1"]) != 1 || len(agents) != 1 || agents[0].ID != "fake" || !agents[0].Installed {
		t.Fatalf("snapshot: %+v %+v", chats, agents)
	}
	c = r.m.get("c1")
	if err := c.load(); err != nil {
		t.Fatal(err)
	}
	if p := c.find(KindPermission); p == nil || p.State != PermCancelled {
		t.Fatalf("permission after restart: %+v", p)
	}
}

func TestClosingKillsTheProcessGroup(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	r.prompt(c, "spawn")
	r.waitState(c, StateIdle)
	pid, err := strconv.Atoi(strings.TrimPrefix(c.find(KindAgent).Text, "pid:"))
	if err != nil || !alive(pid) {
		t.Fatalf("grandchild pid %v: %v", pid, err)
	}
	if _, err := r.run("chat.close", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "grandchild to die", func() bool { return !alive(pid) })
	if c.Info().State != StateStopped {
		t.Fatalf("state %s", c.Info().State)
	}

	// Server shutdown does the same.
	if _, err := r.run("chat.resume", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
	r.prompt(c, "spawn")
	r.waitState(c, StateIdle)
	msgs := c.messages()
	pid2, _ := strconv.Atoi(strings.TrimPrefix(msgs[len(msgs)-1].Text, "pid:"))
	r.m.Shutdown()
	waitFor(t, "grandchild to die at shutdown", func() bool { return !alive(pid2) })
}

func TestAgentFilesStayInsideTheProject(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	if err := os.WriteFile(filepath.Join(r.dir, "a.txt"), []byte("one\ntwo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.prompt(c, "readfile "+filepath.Join(r.dir, "a.txt"))
	r.waitState(c, StateIdle)
	if got := c.find(KindAgent).Text; !has(got, "one") {
		t.Fatalf("read inside: %s", got)
	}
	r.prompt(c, "readfile /etc/hosts")
	r.waitState(c, StateIdle)
	msgs := c.messages()
	if got := msgs[len(msgs)-1].Text; !has(got, "outside the project") || has(got, "localhost") {
		t.Fatalf("read outside: %s", got)
	}
}

func TestRemoveDeletesSavedChat(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	r.prompt(c, "hello")
	r.waitState(c, StateIdle)
	if _, err := r.run("chat.remove", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(700 * time.Millisecond) // a late flush must not bring it back
	var n int
	_ = r.db.QueryRow(`SELECT (SELECT COUNT(*) FROM chats) + (SELECT COUNT(*) FROM chat_messages)`).Scan(&n)
	if n != 0 || r.m.get("c1") != nil || r.rec.count("chat.removed") != 1 {
		t.Fatalf("rows left: %d", n)
	}
	if _, err := r.run("chat.start", map[string]any{"id": "c1", "projectId": "p1"}); err != nil {
		t.Fatalf("id is free again: %v", err)
	}
}
