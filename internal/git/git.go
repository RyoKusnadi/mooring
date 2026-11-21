// Package git serves the git features of a project to the browser: status,
// diff, stage, discard, commit, push and worktrees. It runs the git program.
// Every path goes through paths.Resolve, scoped to the place a request names.
package git

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/ryokusnadi/mooring/internal/files"
	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/paths"
)

// Publisher receives the git.changed events.
type Publisher interface {
	Publish(kind string, data any)
}

// ChatUse is a saved chat that works in a folder.
type ChatUse struct {
	ID   string
	Live bool // its agent is running
}

// Chats tells which chats work in which folders.
type Chats interface {
	// FolderUse maps the real working folder of each chat of a project to it.
	FolderUse(projectID string) map[string]ChatUse
}

// Config is what a Service needs.
type Config struct {
	Pub Publisher
	// Dir maps a project id to its folder. Errors should be *ledger.Error.
	Dir func(projectID string) (string, error)
	Log *slog.Logger
	// Home and TrashDir pick the trash like files.Config.
	Home     string
	TrashDir string
	// Env is added to git's environment (tests point HOME at an empty folder).
	Env []string
}

// Service is the git feature.
type Service struct {
	cfg   Config
	log   *slog.Logger
	trash string

	mu    sync.Mutex
	chats Chats
	terms Terminals
	locks map[string]*sync.Mutex

	lc logCache

	verOnce sync.Once
	verErr  error
}

// New returns a Service.
func New(cfg Config) *Service {
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{cfg: cfg, log: cfg.Log, trash: files.TrashPath(cfg.Home, cfg.TrashDir), locks: map[string]*sync.Mutex{}}
}

// SetChats says where to ask about chats. Call it before serving.
func (s *Service) SetChats(c Chats) {
	s.mu.Lock()
	s.chats = c
	s.mu.Unlock()
}

func (s *Service) chatUse(projectID string) map[string]ChatUse {
	s.mu.Lock()
	c := s.chats
	s.mu.Unlock()
	if c == nil {
		return nil
	}
	return c.FolderUse(projectID)
}

func apiErr(code, format string, a ...any) *ledger.Error {
	return &ledger.Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

// repoLock serialises changes to one repository.
func (s *Service) repoLock(top string) func() {
	s.mu.Lock()
	m := s.locks[top]
	if m == nil {
		m = &sync.Mutex{}
		s.locks[top] = m
	}
	s.mu.Unlock()
	m.Lock()
	return m.Unlock
}

// place is a folder a request works in, inside a repository.
type place struct {
	project string // project id
	root    string // real project folder
	dir     string // real place folder
	top     string // real top of the repository (the work tree)
	prefix  string // place inside top, "" or slash-separated
}

// scope is the pathspec that limits git to the place: "" means the whole repository.
func (p *place) scope() string { return p.prefix }

// resolve finds the place of a request. When the folder is not in a
// repository, repo is false and the place has only root and dir.
func (s *Service) resolve(projectID, folder string) (pl *place, repo bool, err error) {
	dir, err := s.cfg.Dir(projectID)
	if err != nil {
		return nil, false, err
	}
	root, err := paths.Root(dir)
	if err != nil {
		return nil, false, apiErr("NOT_FOUND", "That project folder cannot be opened.")
	}
	if len(folder) > 4096 || strings.ContainsRune(folder, 0) || filepath.IsAbs(folder) {
		return nil, false, apiErr("BAD_REQUEST", "The folder must be a path inside the project.")
	}
	if folder = path.Clean(folder); folder == "." {
		folder = ""
	}
	real, err := paths.Resolve(root, filepath.FromSlash(folder))
	if err != nil {
		return nil, false, mapPathErr(err, folder)
	}
	if fi, err := os.Stat(real); err != nil || !fi.IsDir() {
		return nil, false, apiErr("NOT_FOUND", "That folder does not exist.")
	}
	pl = &place{project: projectID, root: root, dir: real}
	out, err := s.run(ctxBG(), real, false, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		var re *runError
		if asRun(err, &re) && re.notRepo() {
			return pl, false, nil
		}
		return nil, false, s.failure(err)
	}
	top, err := filepath.EvalSymlinks(strings.TrimRight(string(out), "\r\n"))
	if err != nil || !paths.Inside(top, real) {
		return pl, false, nil
	}
	pl.top = top
	rel, _ := filepath.Rel(top, real)
	if rel != "." {
		pl.prefix = filepath.ToSlash(rel)
	}
	return pl, true, nil
}

// need is resolve for requests that cannot work outside a repository.
func (s *Service) need(projectID, folder string) (*place, error) {
	pl, repo, err := s.resolve(projectID, folder)
	if err != nil {
		return nil, err
	}
	if !repo {
		return nil, apiErr("NOT_A_REPO", "This folder is not in a git repository.")
	}
	return pl, nil
}

// mapPathErr words a path failure for the browser; `rel` names the file or folder when known.
func mapPathErr(err error, rel ...string) error {
	name, lower := "That path", "that path"
	if len(rel) > 0 && rel[0] != "" && len(rel[0]) <= 200 {
		name, lower = rel[0], rel[0]
	}
	switch {
	case isOutside(err):
		return apiErr("OUTSIDE_PROJECT", "%s is outside the project folder.", name)
	case os.IsNotExist(err):
		return apiErr("NOT_FOUND", "%s does not exist.", name)
	}
	return apiErr("BAD_REQUEST", "The server is not allowed to use %s.", lower)
}

// relPath checks a path of a request against the place and returns it as git
// wants it: relative to the repository top. The last name is kept as it is, so
// a link is acted on and not its target. "" is the place itself.
func (p *place) relPath(rel string) (string, error) {
	if len(rel) > 4096 || strings.ContainsRune(rel, 0) || filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", apiErr("BAD_REQUEST", "That path is not valid. Paths are relative to the folder.")
	}
	c := path.Clean(rel)
	if c == "." {
		return p.prefix, nil
	}
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", apiErr("OUTSIDE_PROJECT", "That path leaves the project folder.")
	}
	parent, err := paths.Resolve(p.dir, filepath.FromSlash(path.Dir(c)))
	if err != nil {
		return "", mapPathErr(err, rel)
	}
	full := filepath.Join(parent, path.Base(c))
	r, err := filepath.Rel(p.top, full)
	if err != nil || r == ".." || strings.HasPrefix(r, "../") {
		return "", apiErr("OUTSIDE_PROJECT", "That path leaves the project folder.")
	}
	return filepath.ToSlash(r), nil
}

// inPlace turns a repository path into one relative to the place; ok is false
// when it lies outside.
func (p *place) inPlace(top string) (string, bool) {
	if p.prefix == "" {
		return top, true
	}
	if top == p.prefix {
		return "", true
	}
	if strings.HasPrefix(top, p.prefix+"/") {
		return top[len(p.prefix)+1:], true
	}
	return top, false
}

var credURL = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/@\s]+@`)

// clean keeps git's message to 4 KiB and removes credentials from URLs in it.
func clean(msg string) string {
	msg = credURL.ReplaceAllString(msg, "$1")
	msg = strings.TrimSpace(msg)
	if len(msg) > 4096 {
		msg = strings.ToValidUTF8(msg[:4096], "")
	}
	return msg
}
