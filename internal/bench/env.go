package bench

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/ryokusnadi/mooring/internal/accounts"
	"github.com/ryokusnadi/mooring/internal/chat"
	"github.com/ryokusnadi/mooring/internal/git"
	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/settings"
	"github.com/ryokusnadi/mooring/internal/store"
)

type nopPub struct{}

func (nopPub) Publish(string, any) {}

// env is the chat manager of one run, without HTTP. Its chats live in a database of
// their own inside the run folder, so the user's chats and settings are not touched.
type env struct {
	m        *chat.Manager
	db       *sql.DB
	set      *settings.Service
	agent    string
	smallOn  bool // a small model is set for the agent
	close    func()
	mu       sync.Mutex
	dirs     map[string]string // project id to folder
	jobTotal int64
}

// standIn is the agent definition for -dry: o.Exe, a test binary that runs as the stand-in agent.
func standIn(o Options, runDir string) (chat.AgentDef, error) {
	exe := o.Exe
	if exe == "" {
		return chat.AgentDef{}, errors.New("no stand-in agent program given")
	}
	benchDir, err := filepath.Abs(o.BenchDir)
	if err != nil {
		return chat.AgentDef{}, fmt.Errorf("find the bench folder: %w", err)
	}
	return chat.AgentDef{ID: o.Agent, Name: "Stand-in", Command: exe, Env: []string{
		"MOORING_FAKE_AGENT=1", "MOORING_BENCH_DIR=" + benchDir, "FAKE_EMBED=1", "FAKE_CLOSE=close",
		"FAKE_STATE=" + filepath.Join(runDir, "fake-state"),
	}}, nil
}

func newEnv(ctx context.Context, o Options, runDir string, log *slog.Logger) (*env, error) {
	var def chat.AgentDef
	if o.Dry {
		var err error
		if def, err = standIn(o, runDir); err != nil {
			return nil, err
		}
	} else {
		found := false
		for _, d := range chat.DefaultCatalog() {
			if d.ID == o.Agent {
				def, found = d, true
			}
		}
		if !found {
			return nil, fmt.Errorf("no agent %q in the catalog", o.Agent)
		}
		if in := def.Info(); !in.Installed {
			return nil, fmt.Errorf("%s needs %q, which was not found on this machine", def.Name, in.Missing)
		}
	}
	st, err := store.Open(runDir, log)
	if err != nil {
		return nil, fmt.Errorf("open the bench database: %w", err)
	}
	e := &env{agent: o.Agent, db: st.DB, dirs: map[string]string{}}
	closers := []func(){func() { _ = st.Close() }}
	e.close = func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}
	fail := func(err error) (*env, error) { e.close(); return nil, err }

	e.set = settings.New(settings.Config{DB: st.DB, Pub: nopPub{}, KnownAgent: func(id string) bool { return id == o.Agent }, Log: log})
	small, err := e.smallModel(o, log)
	if err != nil {
		return fail(err)
	}
	e.smallOn = small != ""
	patch, _ := json.Marshal(map[string]any{"smallModel": map[string]string{o.Agent: small}})
	if small == "" {
		patch = []byte(`{"smallModel":{}}`)
	}
	if _, err := e.set.Save(patch); err != nil {
		return fail(fmt.Errorf("set the small model: %w", err))
	}

	var acct *accounts.Service
	if !o.Dry {
		home, err := os.UserHomeDir()
		if err != nil {
			return fail(fmt.Errorf("find the home folder: %w", err))
		}
		main, err := store.Open(o.DataDir, log)
		if err != nil {
			return fail(fmt.Errorf("open the accounts database: %w", err))
		}
		closers = append(closers, func() { _ = main.Close() })
		adapters, names := chat.Adapters([]chat.AgentDef{def})
		if acct, err = accounts.New(accounts.Config{DB: main.DB, Pub: nopPub{}, DataDir: o.DataDir, Home: home, Adapters: adapters, Agents: names, Log: log}); err != nil {
			return fail(err)
		}
	}
	svc := git.New(git.Config{Pub: nopPub{}, Dir: e.dir, Log: log, Home: runDir})
	e.m, err = chat.New(chat.Config{
		DB: st.DB, Pub: nopPub{}, Dir: e.dir, Catalog: []chat.AgentDef{def}, DefaultAgent: o.Agent, Accounts: acct,
		DataDir: runDir, Settings: e.set, Staged: svc.Staged, Log: log, Context: ctx,
		JobHook: func(r chat.JobRecord) {
			e.mu.Lock()
			e.jobTotal += r.Tokens
			e.mu.Unlock()
		},
	})
	if err != nil {
		return fail(err)
	}
	closers = append(closers, e.m.Shutdown)
	return e, nil
}

// smallModel is the small model of the agent: the stand-in's, or the one the user set.
func (e *env) smallModel(o Options, log *slog.Logger) (string, error) {
	if o.Dry {
		return "fast", nil
	}
	main, err := store.Open(o.DataDir, log)
	if err != nil {
		return "", fmt.Errorf("open the settings database: %w", err)
	}
	defer func() { _ = main.Close() }()
	return settings.New(settings.Config{DB: main.DB, Pub: nopPub{}}).Get().SmallModel[o.Agent], nil
}

func (e *env) dir(projectID string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if d, ok := e.dirs[projectID]; ok {
		return d, nil
	}
	return "", &ledger.Error{Code: "NOT_FOUND", Message: "That project does not exist."}
}

func (e *env) jobs() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.jobTotal
}

// useJobs turns the small-model jobs on or off for the next task.
func (e *env) useJobs(on bool) error {
	_, err := e.set.Save([]byte(fmt.Sprintf(`{"titleJobs":%t,"commitJobs":%t}`, on, on)))
	return err
}
