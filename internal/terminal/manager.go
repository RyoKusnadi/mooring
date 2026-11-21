// Package terminal runs shells on PTYs for the browser: one output ring per
// terminal, several clients per terminal, one PTY size.
package terminal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Limits and sizes.
const (
	RingSize      = 256 << 10
	FlowWindow    = 1 << 20
	MaxTerminals  = 30
	MaxPerProject = 10
	ExitedKeep    = 10 * time.Minute

	minCols, maxCols, maxRows = 2, 500, 300
	defaultCols, defaultRows  = 80, 24
	killGrace                 = 500 * time.Millisecond
	repaintWait               = 100 * time.Millisecond
	sweepEvery                = 30 * time.Second
	shutdownWait              = 3 * time.Second
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Publisher receives the events terminals produce.
type Publisher interface {
	Publish(kind string, data any)
}

// Config is what a Manager needs. Zero values give the contract's numbers.
type Config struct {
	Pub Publisher
	// Dir maps a project id to the folder its shells start in. Errors should be *ledger.Error.
	Dir func(projectID string) (string, error)
	Log *slog.Logger
	// Remote starts shells on SSH connections; nil refuses them.
	Remote RemoteOpener
	// Shell is the program to run; "" uses $SHELL, then /bin/sh. Args nil means "-l" (login shell).
	Shell string
	Args  []string
	// Context ends every socket when cancelled (server shutdown).
	Context context.Context
	Now     func() time.Time
	// Tests shrink these.
	RingSize     int
	FlowWindow   int
	ExitedKeep   time.Duration
	Idle         time.Duration
	KillGrace    time.Duration
	RepaintAfter time.Duration
}

// Manager owns every terminal.
type Manager struct {
	Config
	log         *slog.Logger
	pub         Publisher
	ringSize    int
	window      int
	keep        time.Duration
	killGrace   time.Duration
	repaintWait time.Duration
	now         func() time.Time
	ctx         context.Context

	mu     sync.Mutex
	terms  map[string]*Terminal
	closed bool
	stop   chan struct{}
}

// New returns a manager and starts its cleanup of old exited terminals. Call Close to end it.
func New(cfg Config) *Manager {
	m := &Manager{Config: cfg, terms: map[string]*Terminal{}, stop: make(chan struct{})}
	m.log, m.pub, m.now, m.ctx = cfg.Log, cfg.Pub, cfg.Now, cfg.Context
	if m.log == nil {
		m.log = slog.New(slog.DiscardHandler)
	}
	if m.now == nil {
		m.now = time.Now
	}
	if m.ctx == nil {
		m.ctx = context.Background()
	}
	m.ringSize = orDefault(cfg.RingSize, RingSize)
	m.window = orDefault(cfg.FlowWindow, FlowWindow)
	m.keep = orDefault(cfg.ExitedKeep, ExitedKeep)
	m.killGrace = orDefault(cfg.KillGrace, killGrace)
	m.repaintWait = orDefault(cfg.RepaintAfter, repaintWait)
	go func() {
		tick := time.NewTicker(sweepEvery)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				m.sweep()
			case <-m.stop:
				return
			}
		}
	}()
	return m
}

func orDefault[T int | time.Duration](v, def T) T {
	if v > 0 {
		return v
	}
	return def
}

// defaultLang is used when the server has no locale, so the shell speaks UTF-8.
func defaultLang() string {
	if runtime.GOOS == "darwin" {
		return "en_US.UTF-8"
	}
	return "C.UTF-8"
}

func (m *Manager) shell() (string, []string) {
	shell := m.Shell
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "/bin/sh"
	}
	if m.Args != nil {
		return shell, m.Args
	}
	return shell, []string{"-l"}
}

func apiErr(code, format string, a ...any) *ledger.Error {
	return &ledger.Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

// Open starts a local shell for a project, in folder (relative to the project, ""
// for the project folder). id is chosen by the browser.
func (m *Manager) Open(projectID, folder, id string, cols, rows int) (Info, error) {
	return m.open(openReq{projectID: projectID, folder: folder, id: id, cols: cols, rows: rows})
}

// OpenWith starts a local shell in the project folder with extra environment
// variables and a line of text shown first. The browser learns of it from the
// terminal.opened event.
func (m *Manager) OpenWith(projectID, id string, env []string, banner string) error {
	_, err := m.open(openReq{projectID: projectID, id: id, env: env, banner: banner})
	return err
}

// OpenSSH starts a login shell on the SSH connection connID for a project's terminal list.
func (m *Manager) OpenSSH(projectID, connID, id string, cols, rows int) (Info, error) {
	if m.Remote == nil {
		return Info{}, apiErr("NOT_CONNECTED", "Remote shells are not available.")
	}
	return m.open(openReq{projectID: projectID, ssh: connID, id: id, cols: cols, rows: rows})
}

func (m *Manager) get(id string) *Terminal {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.terms[id]
}

// Close kills a terminal's process groups and forgets it. An unknown id is not an error.
func (m *Manager) Close(id string) {
	m.mu.Lock()
	t := m.terms[id]
	delete(m.terms, id)
	m.mu.Unlock()
	if t != nil {
		m.discard(t)
	}
}

// CloseProject closes every terminal of a project.
func (m *Manager) CloseProject(projectID string) {
	m.mu.Lock()
	var gone []*Terminal
	for id, t := range m.terms {
		if t.projectID == projectID {
			gone = append(gone, t)
			delete(m.terms, id)
		}
	}
	m.mu.Unlock()
	for _, t := range gone {
		m.discard(t)
	}
}

func (m *Manager) discard(t *Terminal) {
	t.mu.Lock()
	t.removed = true
	t.mu.Unlock()
	t.kill()
	m.pub.Publish("terminal.closed", map[string]string{"id": t.id, "projectId": t.projectID})
}
