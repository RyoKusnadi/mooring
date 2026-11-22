// Package lsp relays language servers to the browser. A server is started on
// the first socket for a project, folder and server id and is shared by every
// socket for that triple. The relay passes JSON-RPC messages through without
// understanding the language.
package lsp

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ryokusnadi/mooring/internal/files"
	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Publisher receives the events the relay produces on the events socket.
type Publisher interface {
	Publish(kind string, data any)
}

// Watcher is the part of files.Service the relay uses to learn about changes
// on disk. nil turns off didChangeWatchedFiles.
type Watcher interface {
	Watch(a files.WatchArgs) (*files.WatchResult, error)
	Subscribe(fn func(files.FSChange)) (cancel func())
}

// Config is what a Service needs.
type Config struct {
	DB  *sql.DB
	Pub Publisher
	// Dir maps a project id to its folder. Errors should be *ledger.Error.
	Dir   func(projectID string) (string, error)
	Files Watcher
	// DataDir is Mooring's data folder; servers are installed into <DataDir>/lsp.
	// "" turns off both that folder and installing.
	DataDir string
	Log     *slog.Logger
	Context context.Context
	Now     func() time.Time
	// Idle is how long a server without sockets stays up; 0 means 10 minutes.
	Idle time.Duration
	// MemoryMB is the limit for one server's process group; 0 means 2048.
	MemoryMB int

	// PathDirs replaces the PATH; ExtraDirs replaces GOPATH/bin and ~/.cargo/bin
	// (an empty, non-nil slice means none). Tests set them.
	PathDirs  []string
	ExtraDirs []string
	// Tests shrink these.
	MemoryEvery   time.Duration
	StopGrace     time.Duration
	WatchDebounce time.Duration
	WatchRenew    time.Duration
}

// Service is the language server feature.
type Service struct {
	cfg Config
	log *slog.Logger
	db  *sql.DB
	pub Publisher
	ctx context.Context

	extraOnce sync.Once
	extra     []string

	mu         sync.Mutex
	insts      map[instKey]*inst
	installing map[string]bool
	closed     bool
	wg         sync.WaitGroup
	unsub      func()
	holders    int
	quit       chan struct{}
}

// New returns a Service. Call Shutdown when done.
func New(cfg Config) (*Service, error) {
	if cfg.DB == nil || cfg.Dir == nil {
		return nil, fmt.Errorf("lsp needs a database and a project folder lookup")
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Idle <= 0 {
		cfg.Idle = 10 * time.Minute
	}
	if cfg.MemoryMB <= 0 {
		cfg.MemoryMB = 2048
	}
	if cfg.MemoryEvery <= 0 {
		cfg.MemoryEvery = 5 * time.Second
	}
	if cfg.StopGrace <= 0 {
		cfg.StopGrace = 2 * time.Second
	}
	if cfg.WatchDebounce <= 0 {
		cfg.WatchDebounce = 100 * time.Millisecond
	}
	if cfg.WatchRenew <= 0 {
		cfg.WatchRenew = 30 * time.Second
	}
	s := &Service{cfg: cfg, log: cfg.Log, db: cfg.DB, pub: cfg.Pub, ctx: cfg.Context,
		insts: map[instKey]*inst{}, installing: map[string]bool{}, quit: make(chan struct{})}
	if cfg.Files != nil {
		s.unsub = cfg.Files.Subscribe(s.fsChange)
	}
	s.wg.Add(1)
	go s.background()
	return s, nil
}

func apiErr(code, format string, a ...any) *ledger.Error {
	return &ledger.Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

func (s *Service) publish(kind string, data any) {
	if s.pub != nil {
		s.pub.Publish(kind, data)
	}
}
