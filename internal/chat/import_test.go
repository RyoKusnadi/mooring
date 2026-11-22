package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (r *rig) agentSessions() []AgentSession {
	r.t.Helper()
	res, err := r.run("chat.agentSessions", map[string]any{"projectId": r.project, "agent": "fake"})
	if err != nil {
		r.t.Fatalf("agentSessions: %v", err)
	}
	return res.(map[string]any)["sessions"].([]AgentSession)
}

func (c *Chat) session() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionID
}

func fakeState(r *rig) string {
	for _, e := range r.env {
		if v, ok := strings.CutPrefix(e, "FAKE_STATE="); ok {
			return v
		}
	}
	r.t.Fatal("no FAKE_STATE")
	return ""
}

func TestImportAgentSession(t *testing.T) {
	r := newRig(t, "FAKE_LOAD=1", "FAKE_CAPS=list")
	c := r.start("c1")
	r.prompt(c, "hello")
	r.waitState(c, StateIdle)
	sid := c.session()
	// Mooring forgets the chat; the agent still has the session, as if made in its own program.
	if _, err := r.run("chat.remove", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	// A session made in another folder is not offered.
	st := fakeState(r)
	_ = os.WriteFile(filepath.Join(st, "fake-session-other.json"), []byte(`[{"role":"user","text":"elsewhere"}]`), 0o600)
	_ = os.WriteFile(filepath.Join(st, "fake-session-other.cwd"), []byte("/somewhere/else"), 0o600)

	list := r.agentSessions()
	if len(list) != 1 || list[0].SessionID != sid || list[0].Title != "hello" || list[0].UpdatedAt == "" {
		t.Fatalf("list %+v (want %s)", list, sid)
	}

	if _, err := r.run("chat.import", map[string]any{"id": "c2", "projectId": r.project, "agent": "fake", "sessionId": sid, "title": "hello"}); err != nil {
		t.Fatal(err)
	}
	c2 := r.m.get("c2")
	r.waitState(c2, StateIdle)
	r.prompt(c2, "again")
	r.waitState(c2, StateIdle)
	var texts []string
	for _, m := range c2.messages() {
		texts = append(texts, m.Kind+":"+m.Text)
	}
	if got := strings.Join(texts, "|"); got != "user:hello|agent:Hello world|user:again|agent:echo: again" {
		t.Fatalf("imported history: %s", got)
	}
	if c2.Info().Title != "hello" || c2.session() != sid {
		t.Fatalf("info %+v", c2.Info())
	}

	if list := r.agentSessions(); len(list) != 0 {
		t.Fatalf("a session in use is still offered: %+v", list)
	}
	if _, err := r.run("chat.import", map[string]any{"id": "c3", "projectId": r.project, "agent": "fake", "sessionId": sid}); err == nil || !has(err.Error(), "ALREADY_EXISTS") {
		t.Fatalf("second import: %v", err)
	}
	if _, err := r.run("chat.import", map[string]any{"id": "c4", "projectId": r.project, "agent": "fake", "sessionId": "a\nb"}); err == nil || !has(err.Error(), "BAD_REQUEST") {
		t.Fatalf("bad id: %v", err)
	}
}

func TestAgentSessionsNeedsListAndLoad(t *testing.T) {
	r := newRig(t, "FAKE_LOAD=1")
	_, err := r.run("chat.agentSessions", map[string]any{"projectId": r.project, "agent": "fake"})
	if err == nil || !has(err.Error(), "NOT_SUPPORTED") {
		t.Fatalf("no list: %v", err)
	}
}

func TestImportRetriesAFailedLoadAndKeepsHistoryOnce(t *testing.T) {
	r := newRig(t, "FAKE_LOAD=1", "FAKE_CAPS=list")
	st := fakeState(r)
	// The session is not there yet: the load fails and the chat keeps the link.
	if _, err := r.run("chat.import", map[string]any{"id": "c1", "projectId": r.project, "agent": "fake", "sessionId": "late"}); err != nil {
		t.Fatal(err)
	}
	c := r.m.get("c1")
	r.waitState(c, StateStopped)
	if c.session() != "late" || c.find(KindUser) != nil {
		t.Fatalf("after a failed load: session %q, messages %+v", c.session(), c.messages())
	}
	_ = os.WriteFile(filepath.Join(st, "fake-session-late.json"), []byte(`[{"role":"user","text":"hi"},{"role":"agent","text":"hello there"}]`), 0o600)
	// A restart in between: the mark is saved, so the next start still keeps the replay.
	r.m.Shutdown()
	r.boot()
	c = r.m.get("c1")
	if _, err := r.run("chat.resume", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
	talk := func() string {
		var out []string
		for _, m := range c.messages() {
			if m.Kind == KindUser || m.Kind == KindAgent {
				out = append(out, m.Kind+":"+m.Text)
			}
		}
		return strings.Join(out, "|")
	}
	if got := talk(); got != "user:hi|agent:hello there" {
		t.Fatalf("history: %s", got)
	}
	// Later starts replay without keeping it again.
	if _, err := r.run("chat.close", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateStopped)
	if _, err := r.run("chat.resume", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
	if got := talk(); got != "user:hi|agent:hello there" {
		t.Fatalf("history after a second start: %s", got)
	}
}

func TestStartIgnoresASessionID(t *testing.T) {
	r := newRig(t, "FAKE_LOAD=1")
	if _, err := r.run("chat.start", map[string]any{"id": "c1", "projectId": r.project, "sessionId": "someone-elses"}); err != nil {
		t.Fatal(err)
	}
	c := r.m.get("c1")
	r.waitState(c, StateIdle)
	if c.session() == "someone-elses" || c.Info().Spec.Imported {
		t.Fatalf("chat.start used the session id: %q", c.session())
	}
}

func TestImportAfterALoadThatStoppedPartway(t *testing.T) {
	failOnce := filepath.Join(t.TempDir(), "fail")
	_ = os.WriteFile(failOnce, nil, 0o600)
	r := newRig(t, "FAKE_LOAD=1", "FAKE_CAPS=list", "FAKE_LOAD_FAILONCE="+failOnce)
	_ = os.WriteFile(filepath.Join(fakeState(r), "fake-session-s1.json"), []byte(`[{"role":"user","text":"hi"},{"role":"agent","text":"hello there"}]`), 0o600)
	if _, err := r.run("chat.import", map[string]any{"id": "c1", "projectId": r.project, "agent": "fake", "sessionId": "s1"}); err != nil {
		t.Fatal(err)
	}
	c := r.m.get("c1")
	r.waitState(c, StateStopped)
	if _, err := r.run("chat.resume", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
	var kinds []string
	for _, m := range c.messages() {
		if m.Kind == KindUser || m.Kind == KindAgent || (m.Kind == KindNotice && strings.Contains(m.Text, "stopped partway")) {
			kinds = append(kinds, m.Kind)
		}
	}
	// The partial replay, the note, then the whole session again.
	want := KindUser + "," + KindAgent + "," + KindNotice + "," + KindUser + "," + KindAgent
	if got := strings.Join(kinds, ","); got != want {
		t.Fatalf("messages: %s, want %s", got, want)
	}
	if c.Info().Spec.Imported {
		t.Fatal("the import mark stayed after a load that worked")
	}
}
