package accounts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveRefusesFoldersAnotherLoginUses(t *testing.T) {
	r := newRig(t)
	builtin := filepath.Join(r.home, ".claude")
	if err := os.MkdirAll(filepath.Join(builtin, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	a := r.save("Managed")
	own := filepath.Join(r.home, "mine")
	if err := os.Mkdir(own, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := r.run("account.save", map[string]any{"account": map[string]any{"agent": "claude", "name": "Mine", "dir": own}}); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{builtin, filepath.Join(builtin, "sub"), a.Dir, own, r.data} {
		_, err := r.run("account.save", map[string]any{"account": map[string]any{"agent": "claude", "name": "Other", "dir": dir}})
		if err == nil {
			t.Fatalf("folder %q accepted", dir)
		}
	}
}

func TestRemovingAccountCannotBeStartedOn(t *testing.T) {
	r := newRig(t)
	a := r.save("Work")
	r.s.setRemoving(a.ID, true)
	if _, err := r.s.Env("claude", a.ID); err == nil {
		t.Fatal("Env gave a folder for an account being removed")
	}
	if _, err := r.s.Pick("p1", "claude", a.ID, false); err == nil {
		t.Fatal("Pick chose an account being removed")
	}
	if got, err := r.s.Pick("p1", "claude", a.ID, true); err != nil || got != "default:claude" {
		t.Fatalf("lenient pick: %q %v", got, err)
	}
	r.s.setRemoving(a.ID, false)
	if _, err := r.s.Env("claude", a.ID); err != nil {
		t.Fatal(err)
	}
}

type countingChats struct {
	fakeChats
	during func()
}

func (c *countingChats) LiveOn(string) int {
	if c.during != nil {
		c.during()
	}
	return 0
}

func TestRemoveBlocksStartsWhileItChecks(t *testing.T) {
	r := newRig(t)
	a := r.save("Work")
	var envErr, pickErr error
	r.s.SetChats(&countingChats{during: func() {
		_, envErr = r.s.Env("claude", a.ID)
		_, pickErr = r.s.Pick("p1", "claude", a.ID, false)
	}})
	if _, err := r.run("account.remove", map[string]any{"id": a.ID}); err != nil {
		t.Fatal(err)
	}
	if envErr == nil || pickErr == nil {
		t.Fatalf("a chat could start on the account during its removal: %v %v", envErr, pickErr)
	}
}

func TestFailedMoveKeepsProjectChoice(t *testing.T) {
	r := newRig(t)
	a, b := r.save("A"), r.save("B")
	if _, err := r.run("project.setAccount", map[string]any{"projectId": "p1", "agent": "claude", "accountId": a.ID}); err != nil {
		t.Fatal(err)
	}
	seen := ""
	r.s.SetChats(&movingChats{fn: func(id string) error {
		seen = r.s.Effective("p1", "claude")
		return apiErr("BUSY", "x")
	}})
	_, err := r.run("project.setAccount", map[string]any{"projectId": "p1", "agent": "claude", "accountId": b.ID})
	wantCode(t, err, "BUSY")
	if seen != b.ID {
		t.Fatalf("chats should see the new choice while moving, saw %q", seen)
	}
	if got := r.s.Effective("p1", "claude"); got != a.ID {
		t.Fatalf("choice not put back: %q", got)
	}
}

type movingChats struct{ fn func(id string) error }

func (m *movingChats) LiveOn(string) int { return 0 }
func (m *movingChats) MoveAccount(_, _, id string) ([]string, error) {
	return nil, m.fn(id)
}
