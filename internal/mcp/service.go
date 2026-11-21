// Package mcp keeps the tool servers (MCP servers) that chats can use: the
// catalog, saved servers with their consent, health checks, sign-in for remote
// servers, and the lookup that fills a chat's servers in at start. Secret
// values and tokens live in the secret store only; they are never in SQLite,
// logs, events or the ledger, and never go to the browser.
package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/secrets"
)

// Limits and times.
const (
	MaxServers        = 100
	maxSecretBytes    = 16 << 10
	signInLife        = 10 * time.Minute
	maxPending        = 32
	refreshAhead      = 60 * time.Second
	firstCheckTimeout = 120 * time.Second
	checkTimeout      = 30 * time.Second
	maxStderr         = 4 << 10
	maxTools          = 200
	catalogPage       = 50
	maxChecks         = 4
	defaultRegistry   = "https://registry.modelcontextprotocol.io/v0/servers"
)

// Publisher receives the events tool servers produce.
type Publisher interface {
	Publish(kind string, data any)
}

// Config is what a Service needs.
type Config struct {
	DB      *sql.DB
	Pub     Publisher
	Secrets secrets.Store // nil: no secret can be saved
	Home    string        // the folder stdio servers start in
	DataDir string        // holds the downloaded catalog; "" keeps it in memory
	// RedirectBase is where the sign-in site sends the browser back to, such as
	// http://127.0.0.1:7340 (the address Mooring listens on).
	RedirectBase string
	Log          *slog.Logger
	Context      context.Context
	Now          func() time.Time
	// Tests change these.
	RegistryURL    string
	RegistryClient *http.Client
	LookPath       func(string) (string, error)
	FirstTimeout   time.Duration
	Timeout        time.Duration
}

// Service owns saved tool servers.
type Service struct {
	Config
	db  *sql.DB
	log *slog.Logger
	ctx context.Context
	reg *registryHolder

	mu      sync.Mutex // one change of a saved row at a time
	locks   sync.Map   // per-server token refresh locks
	pending *pendingSignIns
	cache   toolCache // last good tool list per server, in memory only
	notes   sync.Map  // server id to why it needs a sign-in
	// Both guarded by mu: edits per server (a running check drops its result when the
	// number moved) and the servers being checked now.
	gen      map[string]uint64
	checking map[string]bool
}

// New returns a Service.
func New(cfg Config) (*Service, error) {
	if cfg.DB == nil || cfg.Pub == nil {
		return nil, fmt.Errorf("tool servers need a database and a publisher")
	}
	s := &Service{Config: cfg, db: cfg.DB, log: cfg.Log, ctx: cfg.Context, pending: &pendingSignIns{m: map[string]*pendingSignIn{}},
		gen: map[string]uint64{}, checking: map[string]bool{}}
	if s.log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}
	if s.ctx == nil {
		s.ctx = context.Background()
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.LookPath == nil {
		s.LookPath = exec.LookPath
	}
	if s.RegistryURL == "" {
		s.RegistryURL = defaultRegistry
	}
	if s.FirstTimeout == 0 {
		s.FirstTimeout = firstCheckTimeout
	}
	if s.Timeout == 0 {
		s.Timeout = checkTimeout
	}
	file := ""
	if cfg.DataDir != "" {
		file = filepath.Join(cfg.DataDir, "mcp-registry.json")
	}
	s.reg = &registryHolder{file: file}
	return s, nil
}

func apiErr(code, format string, a ...any) *ledger.Error {
	return &ledger.Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

// CanSaveSecrets is true when a secret store is present.
func (s *Service) CanSaveSecrets() bool { return s.Secrets != nil }

func (s *Service) changed(r rec) { s.Pub.Publish("toolServer.changed", r.ToolServer) }

// update changes one saved row under the lock, writes it and publishes it.
func (s *Service) update(id string, f func(*rec)) (rec, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.get(id)
	if err != nil {
		return rec{}, err
	}
	f(&r)
	if err := s.putKeep(r); err != nil {
		return rec{}, err
	}
	r.Allowed = r.allowedSig != "" && r.allowedSig == r.sig()
	s.changed(r)
	return r, nil
}

// putKeep writes a row without moving it.
func (s *Service) putKeep(r rec) error {
	var pos int
	if err := s.db.QueryRow(`SELECT position FROM tool_servers WHERE id = ?`, r.ID).Scan(&pos); err != nil {
		return fmt.Errorf("read tool server position: %w", err)
	}
	return s.put(r, pos)
}

// List returns every saved server.
func (s *Service) List() ([]ToolServer, error) {
	rs, err := s.list()
	if err != nil {
		return nil, err
	}
	out := make([]ToolServer, len(rs))
	for i, r := range rs {
		out[i] = r.ToolServer
	}
	return out, nil
}

// Shutdown forgets sign-ins that are still waiting. Check processes end with the
// service context, which the server cancels first.
func (s *Service) Shutdown() {
	s.pending.mu.Lock()
	s.pending.m = map[string]*pendingSignIn{}
	s.pending.mu.Unlock()
}
