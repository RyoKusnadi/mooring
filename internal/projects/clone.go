package projects

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ryokusnadi/mooring/internal/events"
)

// Progress lines arrive many times a second; the stream keeps 1024 events.
const progressEvery = 250 * time.Millisecond

var credentials = regexp.MustCompile(`://[^/@\s]+@`)

func redact(s string) string { return credentials.ReplaceAllString(s, "://") }

// repoName guesses a folder name from a clone URL: "https://h/x/repo.git" -> "repo".
func repoName(url string) string {
	u := strings.TrimRight(url, "/")
	if i := strings.LastIndexAny(u, "/:"); i >= 0 {
		u = u[i+1:]
	}
	return strings.TrimSuffix(u, ".git")
}

func (s *Service) projectClone(ctx context.Context, cmd events.Command) (any, error) {
	var a struct{ URL, Parent, Name string }
	if err := decode(cmd.Args, &a); err != nil {
		return nil, err
	}
	url := strings.TrimSpace(a.URL)
	if url == "" || strings.HasPrefix(url, "-") {
		return nil, apiErr("BAD_REQUEST", "Enter the address of a repository to clone.")
	}
	name := a.Name
	if name == "" {
		name = repoName(url)
	}
	folder, err := folderName(name)
	if err != nil {
		return nil, err
	}
	parent, err := s.resolveDir(a.Parent)
	if err != nil {
		return nil, err
	}
	dest, err := s.resolve(filepath.Join(parent, folder))
	if err != nil {
		return nil, err
	}
	// Creating the folder first claims it: git clones into an empty folder, and a
	// failed clone then removes only a folder this command made.
	if err := os.Mkdir(dest, 0o755); errors.Is(err, os.ErrExist) {
		return nil, apiErr("ALREADY_EXISTS", "%q already exists there.", folder)
	} else if err != nil {
		return nil, fmt.Errorf("create clone folder: %w", err)
	}

	if err := s.runClone(ctx, cmd.ID, url, dest); err != nil {
		if rmErr := os.RemoveAll(dest); rmErr != nil {
			s.log.Error("projects: remove failed clone", "err", rmErr)
		}
		return nil, err
	}
	p, err := s.insertProject(dest, folder)
	if err != nil {
		_ = os.RemoveAll(dest)
		return nil, err
	}
	return p, nil
}

func (s *Service) runClone(ctx context.Context, cmdID, url, dest string) error {
	ctx, cancel := context.WithTimeout(ctx, s.cloneTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, s.git, "clone", "--progress", "--", url, dest)
	c.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		// Refuse "ext::" and other transports that run commands.
		"GIT_ALLOW_PROTOCOL=file:git:http:https:ssh",
	)
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		c.Env = append(c.Env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	}
	c.WaitDelay = 5 * time.Second
	stderr, err := c.StderrPipe()
	if err != nil {
		return fmt.Errorf("clone stderr: %w", err)
	}
	if err := c.Start(); err != nil {
		s.log.Error("projects: start git", "err", err)
		return apiErr("CLONE_FAILED", "Git could not be started. Is it installed?")
	}

	var last string
	var lastSent time.Time
	sc := bufio.NewScanner(stderr)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	sc.Split(splitLines)
	for sc.Scan() {
		line := redact(strings.TrimSpace(sc.Text()))
		if line == "" {
			continue
		}
		last = line
		if now := time.Now(); now.Sub(lastSent) >= progressEvery {
			lastSent = now
			s.pub.Publish("clone.progress", map[string]string{"cmd": cmdID, "line": line})
		}
	}
	waitErr := c.Wait()
	switch {
	case waitErr == nil:
		if last != "" {
			s.pub.Publish("clone.progress", map[string]string{"cmd": cmdID, "line": last})
		}
		return nil
	case ctx.Err() != nil && errors.Is(context.Cause(ctx), context.DeadlineExceeded):
		return apiErr("CLONE_FAILED", "The clone took longer than %d minutes and was stopped.", int(s.cloneTimeout/time.Minute))
	case ctx.Err() != nil:
		return ctx.Err()
	}
	s.log.Warn("projects: clone failed", "cmd", cmdID, "err", waitErr)
	msg := "The clone failed."
	if last != "" {
		msg = "The clone failed: " + last
	}
	return apiErr("CLONE_FAILED", "%s", msg)
}

// splitLines splits on \n and on \r, because git redraws progress with \r.
func splitLines(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
