package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/fakeagent"
	"github.com/ryokusnadi/mooring/internal/fakemcp"
	"github.com/ryokusnadi/mooring/internal/secrets"
	"github.com/ryokusnadi/mooring/internal/store"
)

// The test binary doubles as the stand-in agent and the stdio tool server.
func TestMain(m *testing.M) {
	if os.Getenv("MOORING_FAKE_AGENT") == "1" {
		fakeagent.Run(os.Stdin, os.Stdout)
		return
	}
	if os.Getenv("MOORING_FAKE_MCP") == "1" {
		if err := fakemcp.RunStdio(); err != nil {
			os.Exit(1)
		}
		return
	}
	os.Exit(m.Run())
}

// memSecrets is an in-memory secret store: tests never touch the keychain.
type memSecrets struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemSecrets() *memSecrets { return &memSecrets{m: map[string]string{}} }

func (s *memSecrets) Get(n string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[n]
	if !ok {
		return "", secrets.ErrNotFound
	}
	return v, nil
}

func (s *memSecrets) Set(n, v string) error {
	s.mu.Lock()
	s.m[n] = v
	s.mu.Unlock()
	return nil
}

func (s *memSecrets) Delete(n string) error {
	s.mu.Lock()
	delete(s.m, n)
	s.mu.Unlock()
	return nil
}
func (s *memSecrets) Kind() string { return "memory" }

type pubs struct {
	mu   sync.Mutex
	list []string
}

func (p *pubs) Publish(kind string, data any) {
	b, _ := json.Marshal(data)
	p.mu.Lock()
	p.list = append(p.list, kind+" "+string(b))
	p.mu.Unlock()
}

func (p *pubs) all() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := ""
	for _, l := range p.list {
		out += l + "\n"
	}
	return out
}

type rig struct {
	t    *testing.T
	svc  *Service
	st   *store.Store
	sec  *memSecrets
	pub  *pubs
	web  *httptest.Server // serves the callback route
	fake *fakemcp.HTTP
	fts  *httptest.Server
}

func newRig(t *testing.T) *rig {
	t.Helper()
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	r := &rig{t: t, st: st, sec: newMemSecrets(), pub: &pubs{}}
	var svc *Service
	r.web = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		mux := http.NewServeMux()
		svc.Mount(mux, func(h http.Handler) http.Handler { return h })
		mux.ServeHTTP(w, q)
	}))
	t.Cleanup(r.web.Close)
	svc, err = New(Config{DB: st.DB, Pub: r.pub, Secrets: r.sec, Home: t.TempDir(), RedirectBase: r.web.URL,
		Context: context.Background(), FirstTimeout: 20 * time.Second, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	r.svc = svc
	r.fake = fakemcp.New()
	r.fts = httptest.NewServer(r.fake.Handler())
	r.fake.Base = r.fts.URL
	t.Cleanup(r.fts.Close)
	return r
}

func (r *rig) cmd(name string, args any) (json.RawMessage, error) {
	r.t.Helper()
	raw, _ := json.Marshal(args)
	v, err := r.svc.Run(context.Background(), events.Command{ID: "c", Name: name, Args: raw})
	if err != nil {
		return nil, err
	}
	out, _ := json.Marshal(v)
	return out, nil
}

func (r *rig) save(a SaveArgs) ToolServer {
	r.t.Helper()
	ts, err := r.svc.Save(context.Background(), a)
	if err != nil {
		r.t.Fatalf("save: %v", err)
	}
	return ts
}

func (r *rig) stdioArgs(name string, env map[string]string) SaveArgs {
	vars := []Var{{Name: "MOORING_FAKE_MCP"}}
	vals := map[string]string{"MOORING_FAKE_MCP": "1"}
	for k, v := range env {
		vars = append(vars, Var{Name: k})
		vals[k] = v
	}
	return SaveArgs{Server: ToolServer{Name: name, Kind: "stdio", Command: os.Args[0], Env: vars}, Secrets: &SecretValues{Env: vals}, Allow: true}
}

func (r *rig) remoteArgs(name string) SaveArgs {
	return SaveArgs{Server: ToolServer{Name: name, Kind: "http", URL: r.fts.URL + "/mcp", AllowPrivate: true}, Allow: true}
}
