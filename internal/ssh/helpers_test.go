package ssh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/fakessh"
	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/secrets"
	"github.com/ryokusnadi/mooring/internal/store"
)

const testPassword = "s3cret-pw-4711"

type event struct {
	kind string
	data any
}

type pub struct {
	mu      sync.Mutex
	list    []event
	prompts chan *prompt
}

func (p *pub) Publish(kind string, data any) {
	p.mu.Lock()
	p.list = append(p.list, event{kind, data})
	p.mu.Unlock()
	if pr, ok := data.(*prompt); ok && kind == "ssh.prompt" {
		p.prompts <- pr
	}
}

func (p *pub) count(kind string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, e := range p.list {
		if e.kind == kind {
			n++
		}
	}
	return n
}

// lockedBuf is a log sink safe for concurrent writers.
type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// fakeStore is an in-memory secret store.
type fakeStore struct {
	mu   sync.Mutex
	vals map[string]string
}

func (f *fakeStore) Get(n string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.vals[n]; ok {
		return v, nil
	}
	return "", secrets.ErrNotFound
}

func (f *fakeStore) Set(n, v string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vals[n] = v
	return nil
}

func (f *fakeStore) Delete(n string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.vals, n)
	return nil
}

func (f *fakeStore) Kind() string { return "fake" }

type env struct {
	t     *testing.T
	m     *Manager
	fake  *fakessh.Server
	pub   *pub
	store *fakeStore
	logs  *lockedBuf
	led   *ledger.Ledger
	home  string // the temp HOME with its .ssh
	data  string // Mooring's data folder
	sftp  string // the fake server's home
}

type envOpts struct {
	fake    fakessh.Options
	noStore bool
}

func newEnv(t *testing.T, o envOpts) *env {
	t.Helper()
	e := &env{t: t, pub: &pub{prompts: make(chan *prompt, 64)}, store: &fakeStore{vals: map[string]string{}}, logs: &lockedBuf{}}
	e.home, e.data = t.TempDir(), t.TempDir()
	t.Setenv("HOME", e.home)
	if err := os.MkdirAll(filepath.Join(e.home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	st, err := store.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if e.led, err = ledger.New(ctx, st.DB, nil, log); err != nil {
		t.Fatal(err)
	}
	if o.fake.HostKey == nil {
		if o.fake.HostKey, err = fakessh.GenerateKey(); err != nil {
			t.Fatal(err)
		}
	}
	if o.fake.User == "" {
		o.fake.User = "u"
	}
	if o.fake.Password == "" && len(o.fake.AuthorizedKeys) == 0 && len(o.fake.Questions) == 0 {
		o.fake.Password = testPassword
	}
	e.sftp, _ = filepath.EvalSymlinks(t.TempDir())
	if o.fake.Home == "" {
		o.fake.Home = e.sftp
	}
	if e.fake, err = fakessh.Start(o.fake); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.fake.Close)
	cfg := Config{
		DB: st.DB, Pub: e.pub, Ledger: e.led, Home: e.home, DataDir: e.data, Log: log, Context: ctx,
		AgentSocket:   func() string { return "" },
		PromptTimeout: 5 * time.Second, DialTimeout: 5 * time.Second,
	}
	if !o.noStore {
		cfg.Secrets = e.store
	}
	if e.m, err = New(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.m.Shutdown)
	return e
}

func (e *env) saveHost(auth, keyPath string) Host {
	e.t.Helper()
	h, err := e.m.SaveHost(Host{Name: "fake", Address: "127.0.0.1", Port: e.fake.Port(), User: "u", Auth: auth, KeyPath: keyPath})
	if err != nil {
		e.t.Fatal(err)
	}
	return h
}

// answerer decides how to answer a prompt: the fields of an ssh.answer without promptId.
type answerer func(p *prompt) map[string]any

func standard(password string) answerer {
	return func(p *prompt) map[string]any {
		switch p.Kind {
		case "hostkey":
			return map[string]any{"accept": true}
		case "password":
			return map[string]any{"value": password}
		}
		return map[string]any{"value": ""}
	}
}

// connect runs Connect and answers every prompt with r until it returns.
func (e *env) connect(id string, h Host, r answerer) (Connection, error) {
	e.t.Helper()
	type res struct {
		c   Connection
		err error
	}
	ch := make(chan res, 1)
	go func() {
		c, err := e.m.Connect(context.Background(), id, h.ID)
		ch <- res{c, err}
	}()
	timeout := time.After(20 * time.Second)
	for {
		select {
		case p := <-e.pub.prompts:
			a := r(p)
			if a == nil {
				continue
			}
			a["promptId"] = p.ID
			raw, _ := json.Marshal(a)
			_ = e.m.Answer(raw)
		case r := <-ch:
			return r.c, r.err
		case <-timeout:
			e.t.Fatal("connect did not finish")
		}
	}
}

func (e *env) mustConnect(id string, h Host) {
	e.t.Helper()
	if _, err := e.connect(id, h, standard(testPassword)); err != nil {
		e.t.Fatalf("connect: %v", err)
	}
}

func code(err error) string {
	var le *ledger.Error
	if errors.As(err, &le) {
		return le.Code
	}
	if err == nil {
		return ""
	}
	return fmt.Sprintf("plain error: %v", err)
}

func wait(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if ok() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func itoa(n int) string { return fmt.Sprint(n) }
