package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func startIdle(t *testing.T, r *rig, id string) *Chat {
	t.Helper()
	if _, err := r.run("chat.start", map[string]any{"id": id, "projectId": r.project}); err != nil {
		t.Fatal(err)
	}
	c := r.m.get(id)
	r.waitState(c, StateIdle)
	return c
}

func waitJobs(t *testing.T, r *rig) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := r.m.WaitJobs(ctx); err != nil {
		t.Fatal(err)
	}
}

// Removing a chat asks the agent to delete its own copy of the session, once.
func TestRemoveDeletesTheAgentsSession(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	r := newRig(t, "FAKE_CAPS=delete", "FAKE_CALLS="+calls)
	c := startIdle(t, r, "c1")
	saved := filepath.Join(fakeState(r), "fake-session-"+c.session()+".json")
	r.prompt(c, "hello")
	r.waitState(c, StateIdle)
	if _, err := os.Stat(saved); err != nil {
		t.Fatalf("the fake did not save the session: %v", err)
	}
	if _, err := r.run("chat.remove", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	waitJobs(t, r)
	if raw, _ := os.ReadFile(calls); string(raw) != "session/delete\n" {
		t.Fatalf("calls: %q", raw)
	}
	if _, err := os.Stat(saved); !os.IsNotExist(err) {
		t.Fatalf("the agent's copy is still there: %v", err)
	}
}

// An agent that cannot delete or close sessions is not asked to.
func TestRemoveAndCloseWithoutTheCapability(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	r := newRig(t, "FAKE_CALLS="+calls)
	startIdle(t, r, "c1")
	if _, err := r.run("chat.close", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.run("chat.remove", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	waitJobs(t, r)
	if raw, _ := os.ReadFile(calls); len(raw) != 0 {
		t.Fatalf("calls: %q", raw)
	}
}

// Closing a running chat tells the agent first.
func TestCloseTellsTheAgent(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	r := newRig(t, "FAKE_CAPS=close", "FAKE_CALLS="+calls)
	c := startIdle(t, r, "c1")
	if _, err := r.run("chat.close", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateStopped)
	if raw, _ := os.ReadFile(calls); string(raw) != "session/close\n" {
		t.Fatalf("calls: %q", raw)
	}
}

// A chat brought in from the agent's own program leaves the agent's copy alone.
func TestRemoveKeepsAnImportedSession(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	r := newRig(t, "FAKE_CAPS=list,delete", "FAKE_LOAD=1", "FAKE_CALLS="+calls)
	saved := filepath.Join(fakeState(r), "fake-session-cli.json")
	_ = os.WriteFile(saved, []byte(`[{"role":"user","text":"hi"}]`), 0o600)
	if _, err := r.run("chat.import", map[string]any{"id": "c1", "projectId": r.project, "agent": "fake", "sessionId": "cli"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(r.m.get("c1"), StateIdle)
	if _, err := r.run("chat.remove", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	waitJobs(t, r)
	if raw, _ := os.ReadFile(calls); len(raw) != 0 {
		t.Fatalf("calls: %q", raw)
	}
	if _, err := os.Stat(saved); err != nil {
		t.Fatalf("the agent's own session went: %v", err)
	}
}
