package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// topTTL is how long the repository of a folder is remembered. Two checkpoints run per turn,
// and a chat in a plain folder would otherwise start git twice per turn just to hear "no".
// A folder made a repository (or deleted) shows up within this time.
const topTTL = 30 * time.Second

type topEntry struct {
	top, gitDir string
	repo        bool
	at          time.Time
}

type topCache struct {
	mu sync.Mutex
	m  map[string]topEntry
}

func (c *topCache) get(dir string, now time.Time) (topEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[dir]
	if !ok || now.Sub(e.at) >= topTTL {
		return topEntry{}, false
	}
	return e, true
}

func (c *topCache) put(dir string, e topEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || len(c.m) > 256 {
		c.m = make(map[string]topEntry)
	}
	c.m[dir] = e
}

// topOf finds the work tree and git directory of dir. repo is false outside a repository.
func (s *Service) topOf(ctx context.Context, dir string) (top, gitDir string, repo bool, err error) {
	if e, ok := s.tops.get(dir, time.Now()); ok {
		return e.top, e.gitDir, e.repo, nil
	}
	top, gitDir, repo, err = s.readTop(ctx, dir)
	if err == nil {
		s.tops.put(dir, topEntry{top: top, gitDir: gitDir, repo: repo, at: time.Now()})
	}
	return top, gitDir, repo, err
}

func (s *Service) readTop(ctx context.Context, dir string) (top, gitDir string, repo bool, err error) {
	out, err := s.run(ctx, dir, false, nil, "rev-parse", "--show-toplevel", "--absolute-git-dir")
	if err != nil {
		var re *runError
		if asRun(err, &re) && re.notRepo() {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	lines := strings.Split(strings.TrimRight(string(out), "\r\n"), "\n")
	if len(lines) < 2 {
		return "", "", false, fmt.Errorf("git rev-parse gave %d lines", len(lines))
	}
	top, err = filepath.EvalSymlinks(lines[0])
	if err != nil {
		return "", "", false, err
	}
	return top, lines[1], true, nil
}
