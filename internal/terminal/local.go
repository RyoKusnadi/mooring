package terminal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// localBackend is a login shell on a PTY, in a process group of its own.
type localBackend struct {
	f      *os.File
	cmd    *exec.Cmd
	reaped atomic.Bool
}

func startLocal(shell string, args []string, dir string, env []string, cols, rows uint16) (*localBackend, error) {
	cmd := exec.Command(shell, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), env...), "TERM=xterm-256color", "COLORTERM=truecolor")
	if os.Getenv("LANG") == "" && os.Getenv("LC_ALL") == "" {
		cmd.Env = append(cmd.Env, "LANG="+defaultLang())
	}
	// pty.Start gives the child its own session, so its process group is its pid.
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		return nil, fmt.Errorf("start %s in %s: %w", shell, dir, err)
	}
	return &localBackend{f: f, cmd: cmd}, nil
}

func (b *localBackend) Read(p []byte) (int, error)  { return b.f.Read(p) }
func (b *localBackend) Write(p []byte) (int, error) { return b.f.Write(p) }
func (b *localBackend) Close() error                { return b.f.Close() }
func (b *localBackend) pid() int                    { return b.cmd.Process.Pid }

func (b *localBackend) Wait() int {
	err := b.cmd.Wait()
	b.reaped.Store(true)
	return exitCode(err)
}

func exitCode(err error) int {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return 0
	}
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ee.ExitCode()
}

func (b *localBackend) Hangup() { b.signalGroups(syscall.SIGHUP) }
func (b *localBackend) Kill()   { b.signalGroups(syscall.SIGKILL) }

// signalGroups signals the shell's group and the foreground group (a program
// the shell started with job control has a group of its own). Once the shell is
// reaped, only a kill goes on: members of its group may still hold the PTY.
func (b *localBackend) signalGroups(sig syscall.Signal) {
	if b.reaped.Load() && sig != syscall.SIGKILL {
		return
	}
	pg := b.pid()
	_ = syscall.Kill(-pg, sig)
	if fg, err := b.fgGroup(); err == nil && fg > 0 && fg != pg {
		_ = syscall.Kill(-fg, sig)
	}
}

// The ioctls go through SyscallConn so the PTY file stays non-blocking (File.Fd would change that).

func (b *localBackend) fgGroup() (int, error) {
	rc, err := b.f.SyscallConn()
	if err != nil {
		return 0, err
	}
	var pg int
	var ioErr error
	if err := rc.Control(func(fd uintptr) { pg, ioErr = unix.IoctlGetInt(int(fd), unix.TIOCGPGRP) }); err != nil {
		return 0, err
	}
	return pg, ioErr
}

func (b *localBackend) Resize(cols, rows uint16) error {
	rc, err := b.f.SyscallConn()
	if err != nil {
		return err
	}
	var ioErr error
	ws := &unix.Winsize{Row: rows, Col: cols}
	if err := rc.Control(func(fd uintptr) { ioErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, ws) }); err != nil {
		return err
	}
	return ioErr
}

func (b *localBackend) Winch() {
	if pg, err := b.fgGroup(); err == nil && pg > 0 {
		_ = syscall.Kill(-pg, syscall.SIGWINCH)
	}
}
