package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/accounts"
	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/fakeagent"
	"github.com/ryokusnadi/mooring/internal/settings"
	"github.com/ryokusnadi/mooring/internal/store"
)

// The test binary doubles as the stand-in agent when MOORING_FAKE_AGENT=1.
func TestMain(m *testing.M) {
	if os.Getenv("MOORING_FAKE_AGENT") == "1" {
		fakeagent.Run(os.Stdin, os.Stdout)
		return
	}
	os.Exit(m.Run())
}

type recorder struct {
	mu    sync.Mutex
	kinds []string
}

func (r *recorder) Publish(kind string, _ any) {
	r.mu.Lock()
	r.kinds = append(r.kinds, kind)
	r.mu.Unlock()
}

func (r *recorder) count(kind string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, k := range r.kinds {
		if k == kind {
			n++
		}
	}
	return n
}

type rig struct {
	t       *testing.T
	db      *sql.DB
	project string
	dir     string
	env     []string
	rec     *recorder
	m       *Manager
	tools   ToolServers
	accts   *accounts.Service
	data    string
	set     *settings.Service
	staged  func(ctx context.Context, projectID, folder string) (string, string, error)
}

func fakeDef(env ...string) AgentDef {
	return AgentDef{ID: "fake", Name: "Fake", Command: os.Args[0], Env: append([]string{"MOORING_FAKE_AGENT=1"}, env...), Account: &accounts.Adapter{Env: "FAKE_CONFIG", Login: "fake-login", Shared: "projects", Usage: "claude", Home: ".fake"}}
}

// newRig opens a database with one project and a manager whose only agent is the stand-in.
func newRig(t *testing.T, env ...string) *rig {
	t.Helper()
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	r := &rig{t: t, db: st.DB, project: "p1", dir: t.TempDir(), rec: &recorder{}, data: t.TempDir()}
	r.set = settings.New(settings.Config{DB: st.DB})
	r.env = append(env, "FAKE_STATE="+t.TempDir())
	if _, err := st.DB.Exec(`INSERT INTO projects (id, name, path, position, created_at) VALUES ('p1', 'p', ?, 0, 0)`, r.dir); err != nil {
		t.Fatal(err)
	}
	r.boot()
	return r
}

// boot builds a manager on the same database, as a server restart would.
func (r *rig) boot() {
	r.t.Helper()
	m, err := New(Config{
		DB: r.db, Pub: r.rec, Tools: r.tools, Accounts: r.accts, Catalog: []AgentDef{fakeDef(r.env...)}, DefaultAgent: "fake",
		Dir:     func(string) (string, error) { return r.dir, nil },
		DataDir: r.data, Settings: r.set,
		Staged: func(ctx context.Context, p, f string) (string, string, error) {
			if r.staged == nil {
				return "", "", nil
			}
			return r.staged(ctx, p, f)
		},
		KillGrace:  200 * time.Millisecond,
		CancelWait: 2 * time.Second,
	})
	if err != nil {
		r.t.Fatal(err)
	}
	r.m = m
	if r.accts != nil {
		r.accts.SetChats(m)
	}
	r.t.Cleanup(m.Shutdown)
}

func (r *rig) run(name string, a map[string]any) (any, error) {
	r.t.Helper()
	raw, _ := json.Marshal(a)
	return r.m.Run(events.Command{ID: "c", Name: name, Args: raw})
}

func (r *rig) start(id string) *Chat {
	r.t.Helper()
	if _, err := r.run("chat.start", map[string]any{"id": id, "projectId": r.project}); err != nil {
		r.t.Fatalf("start: %v", err)
	}
	c := r.m.get(id)
	r.waitState(c, StateIdle)
	return c
}

func (r *rig) waitState(c *Chat, want string) {
	r.t.Helper()
	waitFor(r.t, "state "+want, func() bool { return c.Info().State == want })
}

func (r *rig) prompt(c *Chat, text string) {
	r.t.Helper()
	if _, err := r.run("chat.prompt", map[string]any{"id": c.id, "text": text}); err != nil {
		r.t.Fatalf("prompt: %v", err)
	}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (c *Chat) messages() []Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Message{}, c.msgs...)
}

func (c *Chat) find(kind string) *Message {
	for _, m := range c.messages() {
		if m.Kind == kind {
			return &m
		}
	}
	return nil
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func has(s, sub string) bool { return strings.Contains(s, sub) }

// stubTools resolves saved tool servers from a fixed table; a missing id is "removed".
type stubTools map[string]ToolServer

func (s stubTools) Usable(id string) error {
	if _, ok := s[id]; !ok {
		return apiErr("NOT_FOUND", "That tool server is not saved.")
	}
	return nil
}

func (s stubTools) Resolve(_ context.Context, id string) (ToolServer, error) {
	t, ok := s[id]
	if !ok {
		return ToolServer{}, errors.New("it was removed")
	}
	return t, nil
}
