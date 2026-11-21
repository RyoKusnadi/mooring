// Package files serves the files of a project to the browser: list, read, write,
// change, search and watch. Every path goes through paths.Resolve with the
// project folder as root.
package files

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/paths"
)

// Limits.
const (
	MaxFile      = 10 << 20
	maxPath      = 4096
	maxHolderSet = 200
	maxWatchDirs = 512
	// Where each entry of a watched folder costs a file handle (macOS kqueue):
	// at most this many per folder, and in all.
	maxFolderHandles = 1000
	maxWatchHandles  = 4000
)

// Publisher receives the events watching produces on the events socket.
type Publisher interface {
	Publish(kind string, data any)
}

// Config is what a Service needs.
type Config struct {
	Pub Publisher
	// Dir maps a project id to its folder. Errors should be *ledger.Error.
	Dir    func(projectID string) (string, error)
	Ledger *ledger.Ledger
	Log    *slog.Logger
	// Context ends watching when cancelled.
	Context context.Context
	// Home is where the macOS trash lives (~/.Trash).
	Home string
	// TrashDir overrides the trash folder; "-" means there is no trash.
	TrashDir string
	// RG is the ripgrep program; "" looks for rg on PATH, "-" never uses it.
	RG string
	// Tests shrink these.
	Debounce time.Duration
	Lease    time.Duration
}

// Service is the files feature.
type Service struct {
	cfg   Config
	log   *slog.Logger
	rg    string
	trash string
	locks [64]sync.Mutex
	srch  chan struct{}
	w     *watcher
	hooks hooks
}

// New returns a Service. Call Close when done.
func New(cfg Config) *Service {
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	if cfg.Debounce == 0 {
		cfg.Debounce = 200 * time.Millisecond
	}
	if cfg.Lease == 0 {
		cfg.Lease = 120 * time.Second
	}
	s := &Service{cfg: cfg, log: cfg.Log, srch: make(chan struct{}, maxSearches)}
	s.rg = findRG(cfg.RG)
	s.trash = TrashPath(cfg.Home, cfg.TrashDir)
	s.w = newWatcher(s)
	return s
}

// Close stops watching.
func (s *Service) Close() { s.w.close() }

func apiErr(code, format string, a ...any) *ledger.Error {
	return &ledger.Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

// root is the real folder of a project.
func (s *Service) root(projectID string) (string, error) {
	dir, err := s.cfg.Dir(projectID)
	if err != nil {
		return "", err
	}
	real, err := paths.Root(dir)
	if err != nil {
		return "", apiErr("NOT_FOUND", "That project folder cannot be opened.")
	}
	return real, nil
}

// clean checks a browser path and returns it in slash form without "." parts.
func clean(rel string) (string, error) {
	if len(rel) > maxPath || strings.ContainsRune(rel, 0) || strings.HasPrefix(rel, "/") {
		return "", apiErr("BAD_REQUEST", "That path is not valid. Paths are relative to the project folder.")
	}
	c := path.Clean(rel)
	if c == "." {
		return "", nil
	}
	return c, nil
}

// follow resolves a browser path through links to a real path inside root.
func (s *Service) follow(root, rel string) (string, error) {
	c, err := clean(rel)
	if err != nil {
		return "", err
	}
	real, err := paths.Resolve(root, filepath.FromSlash(c))
	if err != nil {
		return "", pathErr(err, rel)
	}
	return real, nil
}

// entry resolves the folder that holds rel, and keeps the last name as it is,
// so a link is acted on and not its target.
func (s *Service) entry(root, rel string) (full string, err error) {
	c, err := clean(rel)
	if err != nil {
		return "", err
	}
	if c == "" {
		return "", apiErr("PROTECTED", "The project folder itself cannot be changed here.")
	}
	base := path.Base(c)
	if base == ".." {
		return "", apiErr("BAD_REQUEST", "That path is not valid.")
	}
	parent, err := paths.Resolve(root, filepath.FromSlash(path.Dir(c)))
	if err != nil {
		return "", pathErr(err, rel)
	}
	info, err := os.Stat(parent)
	if err != nil {
		return "", pathErr(err, rel)
	}
	if !info.IsDir() {
		return "", apiErr("NOT_A_FOLDER", "A part of that path is a file, not a folder.")
	}
	return filepath.Join(parent, base), nil
}

// pathErr turns a failure on a path into a message for the browser. With rel, the message
// names the file ("src/main.go does not exist."), as the user typed or clicked it.
func pathErr(err error, rel ...string) error {
	var le *ledger.Error
	name, lower := "That file or folder", "that path"
	if len(rel) > 0 && rel[0] != "" && len(rel[0]) <= 200 {
		name, lower = rel[0], rel[0]
	}
	switch {
	case errors.As(err, &le):
		return le
	case errors.Is(err, paths.ErrOutsideRoot):
		return apiErr("OUTSIDE_PROJECT", "That path leaves the project folder.")
	case errors.Is(err, os.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		return apiErr("NOT_FOUND", "%s does not exist.", name)
	case errors.Is(err, ErrNotRegular):
		return apiErr("NOT_A_FILE", "That is a pipe, socket or device, not a file.")
	case errors.Is(err, os.ErrPermission):
		return apiErr("BAD_REQUEST", "The server is not allowed to use %s.", lower)
	}
	return err
}

// lock serialises writes to one real path.
func (s *Service) lock(real string) func() {
	h := uint32(2166136261)
	for i := 0; i < len(real); i++ {
		h = (h ^ uint32(real[i])) * 16777619
	}
	m := &s.locks[h%uint32(len(s.locks))]
	m.Lock()
	return m.Unlock
}
