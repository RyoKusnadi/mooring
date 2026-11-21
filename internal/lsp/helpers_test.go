package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/fakelsp"
	"github.com/ryokusnadi/mooring/internal/files"
	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/store"
)

// The test binary doubles as the stand-in language server when MOORING_FAKE_LSP=1.
func TestMain(m *testing.M) {
	if os.Getenv("MOORING_FAKE_LSP") == "1" {
		fakelsp.Run(os.Stdin, os.Stdout)
		return
	}
	os.Exit(m.Run())
}

type published struct {
	Kind string
	Data map[string]any
}

type recorder struct {
	mu  sync.Mutex
	got []published
}

func (r *recorder) Publish(kind string, data any) {
	raw, _ := json.Marshal(data)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	r.mu.Lock()
	r.got = append(r.got, published{kind, m})
	r.mu.Unlock()
}

func (r *recorder) wait(t *testing.T, kind string, pred func(map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		for _, p := range r.got {
			if p.Kind == kind && (pred == nil || pred(p.Data)) {
				r.mu.Unlock()
				return p.Data
			}
		}
		r.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no %s event matched; got %v", kind, r.got)
	return nil
}

type env struct {
	t       *testing.T
	svc     *Service
	pub     *recorder
	srv     *httptest.Server
	root    string // the project folder, behind a link on macOS
	bin     string // wrapper programs found on the PATH
	data    string
	watcher *files.Service
}

type option func(*Config)

// newEnv starts a Service on an empty database with project "p1" in a temp folder.
// Programs named in wrappers exist on its PATH and run the stand-in server.
func newEnv(t *testing.T, wrappers []string, opts ...option) *env {
	t.Helper()
	t.Setenv("MOORING_FAKE_LSP", "1")
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	e := &env{t: t, pub: &recorder{}, root: t.TempDir(), bin: t.TempDir(), data: t.TempDir()}
	for _, name := range wrappers {
		script := fmt.Sprintf("#!/bin/sh\nexec %q \"$@\"\n", os.Args[0])
		if err := os.WriteFile(filepath.Join(e.bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dir := func(id string) (string, error) {
		if id != "p1" {
			return "", &ledger.Error{Code: "NOT_FOUND", Message: "no project"}
		}
		return e.root, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e.watcher = files.New(files.Config{Pub: e.pub, Dir: dir, Context: ctx, Debounce: 20 * time.Millisecond, Lease: time.Minute})
	t.Cleanup(e.watcher.Close)
	cfg := Config{DB: st.DB, Pub: e.pub, Dir: dir, Files: e.watcher, DataDir: e.data, Context: ctx,
		PathDirs: []string{e.bin}, ExtraDirs: []string{}, StopGrace: time.Second, WatchDebounce: 30 * time.Millisecond,
		MemoryEvery: time.Hour, Idle: time.Hour}
	for _, o := range opts {
		o(&cfg)
	}
	e.svc, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.svc.Shutdown)
	mux := http.NewServeMux()
	mux.Handle("GET /api/lsp/{id}", e.svc.Handler())
	e.srv = httptest.NewServer(mux)
	t.Cleanup(e.srv.Close)
	return e
}

// allow records consent for a server the way the user would.
func (e *env) allow(id string) []string {
	e.t.Helper()
	d, ok, err := e.svc.def(id)
	if err != nil || !ok {
		e.t.Fatalf("server %s: %v", id, err)
	}
	r := e.svc.resolve(d)
	if r.path == "" {
		e.t.Fatalf("server %s not found", id)
	}
	if _, err := e.svc.Allow(id, r.argv); err != nil {
		e.t.Fatal(err)
	}
	return r.argv
}

func (e *env) write(rel, text string) string {
	e.t.Helper()
	p := filepath.Join(e.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		e.t.Fatal(err)
	}
	return p
}

func uriOf(path string) string { return fileURI(path) }
