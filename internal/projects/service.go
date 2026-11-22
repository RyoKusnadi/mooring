// Package projects runs the folder, project and layout commands of phase 2.
package projects

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/paths"
)

// Publisher receives the events commands produce.
type Publisher interface {
	Publish(kind string, data any)
}

// Service owns projects and layouts. Every path from the browser is resolved
// against Root (the user's home folder) by internal/paths.
type Service struct {
	db   *sql.DB
	pub  Publisher
	root string
	log  *slog.Logger
	now  func() time.Time

	// writeMu makes each store write and the event announcing it one step, so
	// two commands cannot write in one order and announce in the other.
	writeMu sync.Mutex

	// git is the git program to run; tests may change it.
	git string
	// CloneTimeout limits one clone.
	cloneTimeout time.Duration
}

// New returns a service. root is the folder every path must stay inside.
func New(db *sql.DB, pub Publisher, root string, log *slog.Logger) (*Service, error) {
	real, err := paths.Root(root)
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{db: db, pub: pub, root: real, log: log, now: time.Now, git: "git", cloneTimeout: 10 * time.Minute}, nil
}

// Home is the root folder, with symlinks resolved.
func (s *Service) Home() string { return s.root }

func apiErr(code, format string, a ...any) *ledger.Error {
	return &ledger.Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

// Secret reports whether a command's args must not be stored. None are yet.
func (s *Service) Secret(string) bool { return false }

// Run runs one command by name.
func (s *Service) Run(ctx context.Context, cmd events.Command) (any, error) {
	switch cmd.Name {
	case "fs.list":
		return s.fsList(cmd.Args)
	case "project.add":
		return s.projectAdd(cmd.Args)
	case "project.create":
		return s.projectCreate(cmd.Args)
	case "project.clone":
		return s.projectClone(ctx, cmd)
	case "project.rename":
		return s.projectRename(cmd.Args)
	case "project.reorder":
		return s.projectReorder(cmd.Args)
	case "project.remove":
		return s.projectRemove(cmd.Args)
	case "layout.save":
		return s.layoutSave(cmd.Args)
	}
	return nil, apiErr("BAD_REQUEST", "Unknown command %q.", cmd.Name)
}

func decode(raw json.RawMessage, into any) error {
	if err := json.Unmarshal(raw, into); err != nil {
		return apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	return nil
}

// resolve checks a browser path and maps its failures to contract errors.
func (s *Service) resolve(p string) (string, error) {
	got, err := paths.Resolve(s.root, p)
	switch {
	case errors.Is(err, paths.ErrOutsideRoot):
		return "", apiErr("OUTSIDE_ROOT", "That folder is outside your home folder.")
	case err != nil:
		s.log.Warn("projects: resolve path", "err", err)
		return "", apiErr("BAD_REQUEST", "That path cannot be used.")
	}
	return got, nil
}

// resolveDir is resolve plus "must be an existing folder".
func (s *Service) resolveDir(p string) (string, error) {
	got, err := s.resolve(p)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(got)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", apiErr("NOT_FOUND", "That folder does not exist.")
	case err != nil:
		return "", fmt.Errorf("stat %s: %w", got, err)
	case !info.IsDir():
		return "", apiErr("BAD_REQUEST", "That path is a file, not a folder.")
	}
	return got, nil
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("make id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
