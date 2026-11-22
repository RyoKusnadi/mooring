package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryokusnadi/mooring/internal/accounts"
	"github.com/ryokusnadi/mooring/internal/events"
)

// withAccounts gives the rig an account service on temp folders, never the real home.
func (r *rig) withAccounts() *accounts.Service {
	r.t.Helper()
	adapters, names := Adapters([]AgentDef{fakeDef()})
	s, err := accounts.New(accounts.Config{DB: r.db, Pub: r.rec, DataDir: r.t.TempDir(), Home: r.t.TempDir(), Adapters: adapters, Agents: names})
	if err != nil {
		r.t.Fatal(err)
	}
	r.accts = s
	r.boot()
	return s
}

func (r *rig) acct(name string, a map[string]any) (any, error) {
	r.t.Helper()
	raw, _ := json.Marshal(a)
	return r.accts.Run(events.Command{ID: "c", Name: name, Args: raw})
}

func (r *rig) newAccount(name string, signedIn bool) accounts.Account {
	r.t.Helper()
	v, err := r.acct("account.save", map[string]any{"account": map[string]any{"agent": "fake", "name": name}})
	if err != nil {
		r.t.Fatalf("save account: %v", err)
	}
	a := v.(accounts.Account)
	if signedIn {
		if err := os.WriteFile(filepath.Join(a.Dir, "signed-in"), nil, 0o600); err != nil {
			r.t.Fatal(err)
		}
	}
	return a
}

func TestChatFollowsItsAccountAndMovesOnALimit(t *testing.T) {
	r := newRig(t)
	r.withAccounts()
	c := r.start("c1")
	if id := c.Info().Spec.Account.ID; id != "default:fake" {
		t.Fatalf("a new chat should use the project's account, got %q", id)
	}
	r.prompt(c, "hello")
	r.waitState(c, StateIdle)
	work := r.newAccount("Work", true)

	v, err := r.acct("project.setAccount", map[string]any{"projectId": "p1", "agent": "fake", "accountId": work.ID})
	if err != nil || len(v.(map[string]any)["restarted"].([]string)) != 1 {
		t.Fatalf("setAccount: %v %v", v, err)
	}
	r.waitState(c, StateIdle)
	if c.Info().Spec.Account.ID != work.ID {
		t.Fatalf("the chat was not moved: %+v", c.Info().Spec)
	}
	r.prompt(c, "LIMIT")
	waitFor(t, "limit", func() bool { return c.Info().SwitchTo != nil })
	info := c.Info()
	last := c.messages()[len(c.messages())-1]
	if info.State != StateIdle || !info.Attention || !info.LimitOnly || *info.SwitchTo != "default:fake" || last.Code != "LIMIT_REACHED" || last.ResetsAt == 0 {
		t.Fatalf("after a limit: %+v last %+v", info, last)
	}
	waitFor(t, "limit learned", func() bool { // the account hears of it just after the chat's state changes
		got, _ := r.accts.Snapshot()
		for _, a := range got["accounts"].([]accounts.Account) {
			if a.ID == work.ID {
				return a.Usage.LimitHit != 0
			}
		}
		return false
	})
	got, _ := r.accts.Snapshot()
	for _, a := range got["accounts"].([]accounts.Account) {
		if a.ID == work.ID && (a.Usage.LimitHit == 0 || a.Login != accounts.LoginSignedIn || len(a.Usage.Windows) != 1 || a.Usage.Windows[0].Status != "rejected") {
			t.Fatalf("usage of the limited account: %+v", a)
		}
	}
	if _, err := r.acct("project.setAccount", map[string]any{"projectId": "p1", "agent": "fake", "accountId": "default:fake"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
	if info := c.Info(); info.Attention || info.SwitchTo != nil {
		t.Fatalf("a restart should clear the limit: %+v", info)
	}
	r.prompt(c, "hello")
	r.waitState(c, StateIdle)
}

func TestAccountNotSignedInShowsNeedsLogin(t *testing.T) {
	r := newRig(t)
	r.withAccounts()
	work := r.newAccount("Work", false)
	if _, err := r.acct("project.setAccount", map[string]any{"projectId": "p1", "agent": "fake", "accountId": work.ID}); err != nil {
		t.Fatal(err)
	}
	c := r.start("c1")
	r.prompt(c, "hello")
	r.waitState(c, StateNeedsLogin)
	waitFor(t, "login needed", func() bool {
		a, err := r.accts.Env("fake", work.ID)
		_ = a
		got, _ := r.accts.Snapshot()
		for _, x := range got["accounts"].([]accounts.Account) {
			if x.ID == work.ID {
				return err == nil && x.Login == accounts.LoginNeeded
			}
		}
		return false
	})
	if err := os.WriteFile(filepath.Join(work.Dir, "signed-in"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.run("chat.resume", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
	r.prompt(c, "hello")
	r.waitState(c, StateIdle)
	waitFor(t, "signed in", func() bool {
		got, _ := r.accts.Snapshot()
		for _, x := range got["accounts"].([]accounts.Account) {
			if x.ID == work.ID {
				return x.Login == accounts.LoginSignedIn
			}
		}
		return false
	})
}

func TestAccountMoveAndRemoveRefuseWhileAnswering(t *testing.T) {
	r := newRig(t)
	r.withAccounts()
	work := r.newAccount("Work", true)
	c := r.start("c1")
	if _, err := r.acct("project.setAccount", map[string]any{"projectId": "p1", "agent": "fake", "accountId": work.ID}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
	r.prompt(c, "long")
	r.waitState(c, StateRunning)
	if _, err := r.acct("project.setAccount", map[string]any{"projectId": "p1", "agent": "fake", "accountId": "default:fake"}); err == nil || !has(err.Error(), "BUSY") {
		t.Fatalf("move while answering: %v", err)
	}
	if _, err := r.acct("account.remove", map[string]any{"id": work.ID}); err == nil || !has(err.Error(), "BUSY") {
		t.Fatalf("remove while running: %v", err)
	}
	if c.Info().Spec.Account.ID != work.ID {
		t.Fatal("a refused move changed the chat")
	}
	r.run("chat.cancel", map[string]any{"id": "c1"}) //nolint:errcheck
	r.waitState(c, StateIdle)
}

func TestBrowserCannotChooseTheFolder(t *testing.T) {
	r := newRig(t)
	r.withAccounts()
	spec := map[string]any{"account": map[string]string{"configDir": "/tmp/elsewhere"}}
	if _, err := r.run("chat.start", map[string]any{"id": "c1", "projectId": "p1", "spec": spec}); err != nil {
		t.Fatal(err)
	}
	c := r.m.get("c1")
	r.waitState(c, StateIdle)
	if has(c.Info().Spec.Account.ID, "/") {
		t.Fatalf("folder accepted: %+v", c.Info().Spec)
	}
	if _, err := r.run("chat.start", map[string]any{"id": "c2", "projectId": "p1", "spec": map[string]any{"account": map[string]string{"id": "nope"}}}); err == nil || !has(err.Error(), "NOT_FOUND") {
		t.Fatalf("unknown account: %v", err)
	}
}

func TestMovingChatRefusesPromptAndResume(t *testing.T) {
	r := newRig(t)
	r.withAccounts()
	c := r.start("c1")
	c.mu.Lock()
	c.moving = true
	c.mu.Unlock()
	if err := c.Prompt("hello", nil); err == nil || !has(err.Error(), "BUSY") {
		t.Fatalf("prompt while moving: %v", err)
	}
	c.stopAgent(nil, StateStopped, nil)
	if err := r.m.claim(c); err == nil || !has(err.Error(), "BUSY") {
		t.Fatalf("resume while moving: %v", err)
	}
	c.mu.Lock()
	c.moving = false
	c.mu.Unlock()
	if err := r.m.claim(c); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
}

func TestBusyMoveLeavesNoChatMarked(t *testing.T) {
	r := newRig(t)
	r.withAccounts()
	work := r.newAccount("Work", true)
	idle, busy := r.start("c1"), r.start("c2")
	r.prompt(busy, "long")
	r.waitState(busy, StateRunning)
	for i := 0; i < 5; i++ { // map order is random: try both orders
		if _, err := r.m.MoveAccount("p1", "fake", work.ID); err == nil || !has(err.Error(), "BUSY") {
			t.Fatalf("move: %v", err)
		}
	}
	for _, c := range []*Chat{idle, busy} {
		c.mu.Lock()
		moving, id := c.moving, c.spec.Account.ID
		c.mu.Unlock()
		if moving || id == work.ID {
			t.Fatalf("a refused move changed chat %s: moving=%v account=%s", c.id, moving, id)
		}
	}
	if err := idle.Prompt("hello", nil); err != nil {
		t.Fatalf("prompt after refused move: %v", err)
	}
	r.run("chat.cancel", map[string]any{"id": "c2"}) //nolint:errcheck
	r.waitState(busy, StateIdle)
}

func TestNeedsLoginChatCountsAsLive(t *testing.T) {
	r := newRig(t)
	r.withAccounts()
	work := r.newAccount("Work", false)
	if _, err := r.acct("project.setAccount", map[string]any{"projectId": "p1", "agent": "fake", "accountId": work.ID}); err != nil {
		t.Fatal(err)
	}
	c := r.start("c1")
	r.prompt(c, "hello")
	r.waitState(c, StateNeedsLogin)
	if n := r.m.LiveOn(work.ID); n != 1 {
		t.Fatalf("LiveOn = %d", n)
	}
	if _, err := r.acct("account.remove", map[string]any{"id": work.ID}); err == nil || !has(err.Error(), "BUSY") {
		t.Fatalf("remove under a live agent: %v", err)
	}
	if _, err := os.Stat(work.Dir); err != nil {
		t.Fatalf("folder moved under a live agent: %v", err)
	}
}

func TestRefusingWindowOnASuccessfulTurnIsNotALimit(t *testing.T) {
	r := newRig(t)
	r.withAccounts()
	c := r.start("c1")
	r.prompt(c, "RATE five_hour 100")
	r.waitState(c, StateIdle)
	waitFor(t, "turn end", func() bool { return c.find("agent") != nil })
	info := c.Info()
	if info.Attention || info.SwitchTo != nil || c.find("error") != nil {
		t.Fatalf("turn that answered was read as a limit: %+v", info)
	}
	waitFor(t, "signed in", func() bool {
		got, _ := r.accts.Snapshot()
		for _, x := range got["accounts"].([]accounts.Account) {
			if x.ID == "default:fake" {
				return x.Login == accounts.LoginSignedIn
			}
		}
		return false
	})
}

func TestMoveDuringStartEndsWithOneAgentOnTheNewAccount(t *testing.T) {
	r := newRig(t)
	r.withAccounts()
	work := r.newAccount("Work", true)
	for i := 0; i < 5; i++ {
		id := "s" + string(rune('a'+i))
		if _, err := r.run("chat.start", map[string]any{"id": id, "projectId": "p1"}); err != nil {
			t.Fatal(err)
		}
		target := work.ID
		if i%2 == 1 {
			target = "default:fake"
		}
		if _, err := r.acct("project.setAccount", map[string]any{"projectId": "p1", "agent": "fake", "accountId": target}); err != nil {
			t.Fatal(err)
		}
		c := r.m.get(id)
		r.waitState(c, StateIdle)
		if c.Info().Spec.Account.ID != target {
			t.Fatalf("round %d: chat on %s, want %s", i, c.Info().Spec.Account.ID, target)
		}
		r.prompt(c, "hello")
		r.waitState(c, StateIdle)
	}
}
