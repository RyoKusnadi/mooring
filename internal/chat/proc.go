package chat

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const killGrace = 500 * time.Millisecond

// proc is a running agent in its own process group. Its pipes are plain files
// so Wait returns when the agent exits even if a grandchild still holds them
// (killing npx leaves such grandchildren).
type proc struct {
	cmd     *exec.Cmd
	in      io.WriteCloser
	out     io.ReadCloser
	errOut  io.ReadCloser
	exited  chan struct{}
	drained chan struct{} // closed once stderr reaches end of file

	mu      sync.Mutex
	stderr  []byte // last bytes only; never logged in full
	reaped  bool
	exitErr error
}

type launch struct {
	command string
	args    []string
	dir     string
	env     []string
}

func startProc(l launch) (*proc, error) {
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("make pipe: %w", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("make pipe: %w", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("make pipe: %w", err)
	}
	cmd := exec.Command(l.command, l.args...)
	cmd.Dir = l.dir
	cmd.Env = append(os.Environ(), l.env...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = inR, outW, errW
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err = cmd.Start()
	_ = inR.Close()
	_ = outW.Close()
	_ = errW.Close()
	if err != nil {
		_ = inW.Close()
		_ = outR.Close()
		_ = errR.Close()
		return nil, fmt.Errorf("start %s: %w", l.command, err)
	}
	p := &proc{cmd: cmd, in: inW, out: outR, errOut: errR, exited: make(chan struct{}), drained: make(chan struct{})}
	go p.drainStderr()
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.exitErr, p.reaped = err, true
		p.mu.Unlock()
		close(p.exited)
	}()
	return p, nil
}

// why says how the process ended, from its exit status and the last line it wrote to stderr.
func (p *proc) why() string {
	<-p.exited
	// Wait can return before the last stderr bytes are read. A grandchild that keeps the pipe
	// open must not hold up the message, so the wait is short.
	select {
	case <-p.drained:
	case <-time.After(250 * time.Millisecond):
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	out := "it exited"
	var ee *exec.ExitError
	if errors.As(p.exitErr, &ee) {
		out = ee.String()
	}
	if lines := strings.Split(strings.TrimSpace(string(p.stderr)), "\n"); lines[len(lines)-1] != "" {
		out += ": " + cleanLine(lines[len(lines)-1], 160)
	}
	return out
}

// startCause is the plain reason a program did not start, without the path noise.
func startCause(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	var xe *exec.Error
	if errors.As(err, &xe) {
		err = xe.Err
	}
	return err.Error()
}

func (p *proc) drainStderr() {
	defer close(p.drained)
	buf := make([]byte, 4096)
	for {
		n, err := p.errOut.Read(buf)
		if n > 0 {
			p.mu.Lock()
			p.stderr = append(p.stderr, buf[:n]...)
			if len(p.stderr) > 2048 {
				p.stderr = p.stderr[len(p.stderr)-2048:]
			}
			p.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// stop sends SIGTERM to the whole group, then SIGKILL after the grace period
// (always, so grandchildren die even when the agent itself exited), closes the
// pipes and returns once the agent has been reaped or the group is gone.
func (p *proc) stop(grace time.Duration) {
	pg := p.cmd.Process.Pid
	_ = syscall.Kill(-pg, syscall.SIGTERM)
	select {
	case <-p.exited:
	case <-time.After(grace):
	}
	_ = syscall.Kill(-pg, syscall.SIGKILL)
	<-p.exited
	_ = p.in.Close()
	_ = p.out.Close()
	_ = p.errOut.Close()
}

func (p *proc) running() bool {
	select {
	case <-p.exited:
		return false
	default:
		return true
	}
}
