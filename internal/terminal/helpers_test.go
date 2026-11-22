package terminal

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type recorder struct {
	mu   sync.Mutex
	kind []string
}

func (r *recorder) Publish(kind string, _ any) {
	r.mu.Lock()
	r.kind = append(r.kind, kind)
	r.mu.Unlock()
}

func (r *recorder) has(kind string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, k := range r.kind {
		if k == kind {
			return true
		}
	}
	return false
}

// newManager runs scripts under /bin/sh -c; the manager and its processes end with the test.
func newManager(t *testing.T, cfg Config) (*Manager, *recorder) {
	t.Helper()
	rec := &recorder{}
	dir := t.TempDir()
	cfg.Pub = rec
	cfg.Dir = func(string) (string, error) { return dir, nil }
	if cfg.Shell == "" {
		cfg.Shell = "/bin/sh"
	}
	m := New(cfg)
	t.Cleanup(m.Shutdown)
	return m, rec
}

func script(s string) Config { return Config{Args: []string{"-c", s}} }

func open(t *testing.T, m *Manager, id string) *Terminal {
	t.Helper()
	if _, err := m.Open("p1", "", id, 80, 24); err != nil {
		t.Fatalf("open %s: %v", id, err)
	}
	return m.get(id)
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (t *Terminal) pos() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stream.Pos()
}

func (t *Terminal) ringText() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	b, _ := t.stream.Since(t.stream.Oldest(), 1<<30)
	return string(b)
}

// drain pulls everything the pump would send and returns the data and last frames.
func drain(c *client) (data string, state *stateFrame, exit *exitFrame) {
	for {
		w := c.next()
		switch {
		case w.wait:
			return
		case w.state != nil:
			state = w.state
		case w.data != nil:
			data += string(w.data)
		case w.exit != nil:
			exit = w.exit
		}
	}
}

func groupGone(pg int) bool { return syscall.Kill(-pg, 0) == syscall.ESRCH }

func fmtLines(n int) string {
	return fmt.Sprintf("i=0; while [ $i -lt %d ]; do echo line-$i-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx; i=$((i+1)); done", n)
}

func contains(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
