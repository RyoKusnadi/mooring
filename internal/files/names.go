package files

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// MaxNames caps the file names one names request returns.
	MaxNames     = 30000
	namesTimeout = 10 * time.Second
)

// NamesResult is the value of GET /api/files/names.
type NamesResult struct {
	Paths     []string `json:"paths"`
	Truncated bool     `json:"truncated"`
	Engine    string   `json:"engine"`
}

// Names lists the project's file paths for "open file by name", with the same
// rules as search except that hidden files are kept (.github, .env.example):
// ignored folders and links are skipped.
func (s *Service) Names(ctx context.Context, projectID string) (*NamesResult, error) {
	root, err := s.root(projectID)
	if err != nil {
		return nil, err
	}
	select {
	case s.srch <- struct{}{}:
		defer func() { <-s.srch }()
	default:
		return nil, apiErr("SEARCH_BUSY", "Four searches are running. Try again in a moment.")
	}
	ctx, cancel := context.WithTimeout(ctx, namesTimeout)
	defer cancel()
	res := &NamesResult{Paths: []string{}, Engine: "walk"}
	if s.rg == "" || !s.namesRG(ctx, root, res) {
		res.Paths, res.Truncated, res.Engine = res.Paths[:0], false, "walk"
		s.namesWalk(ctx, root, "", ignoreFor(root, ""), res)
	}
	if ctx.Err() != nil {
		res.Truncated = true
	}
	sort.Strings(res.Paths)
	return res, nil
}

// namesRG runs rg --files. It reports false when rg could not run.
func (s *Service) namesRG(ctx context.Context, root string, res *NamesResult) bool {
	args := []string{"--files", "--hidden", "--no-config", "--no-messages", "--no-require-git"}
	for n := range alwaysIgnored {
		args = append(args, "--glob", "!"+n+"/")
	}
	cmd := exec.CommandContext(ctx, s.rg, append(args, "--", ".")...)
	cmd.Dir = root
	out, err := cmd.StdoutPipe()
	if err != nil {
		return false
	}
	if err := cmd.Start(); err != nil {
		s.log.Warn("files: start rg for names", "err", err)
		return false
	}
	res.Engine = "rg"
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		if len(res.Paths) >= MaxNames {
			res.Truncated = true
			break
		}
		res.Paths = append(res.Paths, path.Clean(strings.TrimPrefix(filepath.ToSlash(sc.Text()), "./")))
	}
	if res.Truncated || sc.Err() != nil {
		_ = cmd.Process.Kill()
	}
	_, _ = io.Copy(io.Discard, out)
	_ = cmd.Wait() // exit 1 means no files; a kill or timeout is reported through Truncated
	return true
}

func (s *Service) namesWalk(ctx context.Context, root, rel string, ign ignoreSet, res *NamesResult) {
	des, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return
	}
	for _, de := range des {
		if res.Truncated || ctx.Err() != nil {
			return
		}
		name := de.Name()
		if de.Type()&os.ModeSymlink != 0 {
			continue
		}
		child := path.Join(rel, name)
		switch {
		case de.IsDir():
			if !ign.ignored(child, true) {
				s.namesWalk(ctx, root, child, ign.child(root, child), res)
			}
		case de.Type().IsRegular() && !ign.ignored(child, false):
			if len(res.Paths) >= MaxNames {
				res.Truncated = true
				return
			}
			res.Paths = append(res.Paths, child)
		}
	}
}
