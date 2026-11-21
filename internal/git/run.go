package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ryokusnadi/mooring/internal/paths"
)

// Limits.
const (
	readTimeout   = 60 * time.Second
	changeTimeout = 180 * time.Second
	maxDiff       = 2 << 20
)

// pinned is user configuration that would change the format of git's output
// (and so the diff the browser got and the patch built from it).
var pinned = []string{
	"-c", "diff.suppressBlankEmpty=false", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
	"-c", "diff.relative=false", "-c", "diff.algorithm=default", "-c", "diff.indentHeuristic=true",
	"-c", "diff.interHunkContext=0", "-c", "diff.renames=true", "-c", "diff.wordRegex=", "-c", "color.ui=false",
	"-c", "color.diff=false", "-c", "core.quotePath=true", "-c", "log.showSignature=false",
}

// looseKey marks a context whose git run must not read pathspecs literally.
// `git stash --include-untracked` leaves the untracked files in place under
// GIT_LITERAL_PATHSPECS, so stash save turns it off and names its own pathspec
// with :(literal).
type looseKey struct{}

// runError is a git run that did not succeed.
type runError struct {
	args     []string
	exit     int // -1 when git did not run to its end
	stderr   string
	stdout   string
	timedOut bool
	tooLarge bool
	partial  []byte // the output read before tooLarge stopped it
	missing  bool
}

func (e *runError) Error() string {
	return fmt.Sprintf("git %s: exit %d: %s", strings.Join(e.args[:min(len(e.args), 2)], " "), e.exit, e.stderr)
}

func (e *runError) notRepo() bool {
	return strings.Contains(e.stderr, "not a git repository") || strings.Contains(e.stderr, "must be run in a work tree")
}

func osEnviron() []string { return os.Environ() }

func asRun(err error, re **runError) bool { return errors.As(err, re) }
func isOutside(err error) bool            { return errors.Is(err, paths.ErrOutsideRoot) }
func ctxBG() context.Context              { return context.Background() }

// version checks once that git is installed and new enough.
func (s *Service) version() error {
	s.verOnce.Do(func() {
		out, err := exec.Command("git", "--version").Output()
		if err != nil {
			s.verErr = apiErr("GIT_NOT_INSTALLED", "Git was not found on this machine. Install git 2.20 or newer.")
			return
		}
		var maj, min int
		f := strings.Fields(string(out))
		if len(f) >= 3 {
			parts := strings.SplitN(f[2], ".", 3)
			if len(parts) >= 2 {
				maj, _ = strconv.Atoi(parts[0])
				min, _ = strconv.Atoi(parts[1])
			}
		}
		if maj < 2 || (maj == 2 && min < 20) {
			s.verErr = apiErr("GIT_NOT_INSTALLED", "Git %s is too old. Install git 2.20 or newer.", strings.Join(f[2:], " "))
		}
	})
	return s.verErr
}

func (s *Service) env(write bool) []string {
	env := append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", "GIT_LITERAL_PATHSPECS=1", "LC_ALL=C", "GIT_PAGER=cat", "PAGER=cat",
		"GIT_EDITOR=true", "GIT_ASKPASS=", "SSH_ASKPASS=",
	)
	if !write {
		env = append(env, "GIT_OPTIONAL_LOCKS=0")
	}
	return append(env, s.cfg.Env...)
}

// limitWriter stops a run that prints more than max bytes.
type limitWriter struct {
	buf    bytes.Buffer
	max    int
	cancel context.CancelFunc
	over   bool
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.max > 0 && w.buf.Len()+len(p) > w.max {
		w.buf.Write(p[:max(w.max-w.buf.Len(), 0)]) // keep what fits, for a partial list
		w.over = true
		w.cancel()
		return len(p), nil
	}
	return w.buf.Write(p)
}

// run runs git in dir and returns its output. Exit code 0 is success.
func (s *Service) run(ctx context.Context, dir string, write bool, stdin []byte, args ...string) ([]byte, error) {
	out, _, err := s.runX(ctx, dir, write, stdin, 0, nil, args...)
	return out, err
}

// runX is run with an output limit (0 for none) and the exit codes that also
// count as success. It returns stderr as well.
func (s *Service) runX(ctx context.Context, dir string, write bool, stdin []byte, limit int, okExit []int, args ...string) ([]byte, string, error) {
	if err := s.version(); err != nil {
		return nil, "", err
	}
	timeout := readTimeout
	if write {
		timeout = changeTimeout
	}
	ctx, stop := context.WithTimeout(ctx, timeout)
	defer stop()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "git", append(append([]string{"--no-pager"}, pinned...), args...)...)
	cmd.Dir = dir
	cmd.Env = s.env(write)
	if v, ok := ctx.Value(sshKey{}).(string); ok {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND="+v)
	}
	if ctx.Value(looseKey{}) != nil {
		cmd.Env = append(cmd.Env, "GIT_LITERAL_PATHSPECS=0")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	stdout := &limitWriter{max: limit, cancel: cancel}
	stderr := &limitWriter{max: 64 << 10, cancel: func() {}}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	re := &runError{args: args, exit: -1, stderr: stderr.buf.String(), stdout: stdout.buf.String()}
	switch {
	case stdout.over:
		re.tooLarge, re.partial = true, stdout.buf.Bytes()
		return nil, stderr.buf.String(), re
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		re.timedOut = true
		return nil, stderr.buf.String(), re
	case ctx.Err() != nil:
		return nil, stderr.buf.String(), ctx.Err()
	case err == nil:
		return stdout.buf.Bytes(), stderr.buf.String(), nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		re.exit = ee.ExitCode()
		for _, c := range okExit {
			if c == re.exit {
				return stdout.buf.Bytes(), stderr.buf.String(), nil
			}
		}
		return nil, stderr.buf.String(), re
	}
	if errors.Is(err, exec.ErrNotFound) {
		re.missing = true
	}
	re.stderr = err.Error()
	return nil, stderr.buf.String(), re
}

// failure turns a failed run into the error the browser gets.
func (s *Service) failure(err error) error {
	var re *runError
	if !asRun(err, &re) {
		return err
	}
	msg := clean(re.stderr)
	if msg == "" {
		msg = clean(re.stdout)
	}
	switch {
	case re.timedOut:
		return apiErr("TIMEOUT", "Git took too long and was stopped. A change may still have happened.")
	case re.missing:
		return apiErr("GIT_NOT_INSTALLED", "Git was not found on this machine. Install git 2.20 or newer.")
	case re.notRepo():
		return apiErr("NOT_A_REPO", "This folder is not in a git repository.")
	case authFailed(re.stderr):
		return apiErr("AUTH_FAILED", "The remote wants a login that git cannot ask for here. %s", msg)
	}
	s.log.Warn("git: command failed", "args", re.args[:min(len(re.args), 2)], "exit", re.exit)
	return apiErr("GIT_FAILED", "%s", msg)
}

func authFailed(stderr string) bool {
	for _, m := range []string{"Authentication failed", "could not read Username", "could not read Password", "terminal prompts disabled", "Permission denied (publickey", "Host key verification failed"} {
		if strings.Contains(stderr, m) {
			return true
		}
	}
	return false
}
