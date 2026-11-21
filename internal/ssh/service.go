// Package ssh makes the SSH connections of the server: saved hosts, host key
// checks, sign-in with prompts, remote shells for terminals, SFTP files and
// local port forwards. Secrets are never logged and never in SQLite.
package ssh

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/secrets"
)

// Limits and times.
const (
	MaxConnections   = 20
	MaxForwards      = 20
	DialTimeout      = 15 * time.Second
	PromptTimeout    = 5 * time.Minute
	KeepaliveEvery   = 15 * time.Second
	KeepaliveMisses  = 3
	shellOpenTimeout = 10 * time.Second
)

// Publisher receives the events SSH produces.
type Publisher interface {
	Publish(kind string, data any)
}

// Config is what a Manager needs. Zero values give the contract's numbers.
type Config struct {
	DB      *sql.DB
	Pub     Publisher
	Ledger  *ledger.Ledger
	Secrets secrets.Store // nil: nothing is remembered
	// Home holds ~/.ssh (config, known_hosts, default keys) and bounds key paths.
	Home string
	// DataDir holds Mooring's own known_hosts. "" keeps accepted keys for this run only.
	DataDir string
	Log     *slog.Logger
	Context context.Context
	Now     func() time.Time
	// AgentSocket gives the SSH agent socket path; nil uses $SSH_AUTH_SOCK.
	AgentSocket func() string
	// Tests shrink these.
	DialTimeout    time.Duration
	PromptTimeout  time.Duration
	KeepaliveEvery time.Duration
	KeepaliveWait  time.Duration
}

// Manager owns hosts, connections, prompts and forwards.
type Manager struct {
	Config
	log *slog.Logger
	ctx context.Context

	mu      sync.Mutex
	conns   map[string]*Conn
	prompts map[string]*prompt
	closed  bool
	hostsMu sync.Mutex // one host change and its event at a time
	keys    *hostKeys
	writes  [64]sync.Mutex
	nextID  uint64
}

// New returns a Manager. Call Shutdown when done.
func New(cfg Config) (*Manager, error) {
	if cfg.DB == nil || cfg.Pub == nil || cfg.Home == "" {
		return nil, fmt.Errorf("ssh needs a database, a publisher and a home folder")
	}
	m := &Manager{Config: cfg, conns: map[string]*Conn{}, prompts: map[string]*prompt{}}
	m.log, m.ctx = cfg.Log, cfg.Context
	if m.log == nil {
		m.log = slog.New(slog.DiscardHandler)
	}
	if m.ctx == nil {
		m.ctx = context.Background()
	}
	if m.Now == nil {
		m.Now = time.Now
	}
	m.DialTimeout = orDefault(m.DialTimeout, DialTimeout)
	m.PromptTimeout = orDefault(m.PromptTimeout, PromptTimeout)
	m.KeepaliveEvery = orDefault(m.KeepaliveEvery, KeepaliveEvery)
	m.KeepaliveWait = orDefault(m.KeepaliveWait, KeepaliveEvery)
	own := ""
	if cfg.DataDir != "" {
		own = filepath.Join(cfg.DataDir, "known_hosts")
	}
	m.keys = &hostKeys{user: filepath.Join(cfg.Home, ".ssh", "known_hosts"), own: own}
	return m, nil
}

func orDefault(v, def time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return def
}

func apiErr(code, format string, a ...any) *ledger.Error {
	return &ledger.Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

func (m *Manager) agentSocket() string {
	if m.AgentSocket != nil {
		return m.AgentSocket()
	}
	return os.Getenv("SSH_AUTH_SOCK")
}

// CanRemember is true when a secret store is present.
func (m *Manager) CanRemember() bool { return m.Secrets != nil }

// Shutdown closes every connection.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.closed = true
	all := make([]*Conn, 0, len(m.conns))
	for _, c := range m.conns {
		all = append(all, c)
	}
	m.mu.Unlock()
	for _, c := range all {
		c.shutdown("The server stopped.")
	}
}

// Snapshot is what the events snapshot adds: sshPrompts, sshConnections, sshForwards.
func (m *Manager) Snapshot() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	prompts, conns, fwds := []*prompt{}, []Connection{}, []Forward{}
	for _, p := range m.prompts {
		prompts = append(prompts, p)
	}
	for _, c := range m.conns {
		if info := c.info(); info.State == "connected" {
			conns = append(conns, info)
			fwds = append(fwds, c.forwardList()...)
		}
	}
	return map[string]any{"sshPrompts": prompts, "sshConnections": conns, "sshForwards": fwds}
}
