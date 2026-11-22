package lsp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const stderrKeep = 8 << 10

// proc is a language server in its own process group. Its pipes are plain files
// so Wait returns when the server exits even if a grandchild still holds them.
type proc struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    *bufio.Reader
	outF   io.ReadCloser
	errOut io.ReadCloser
	exited chan struct{}
	errEnd chan struct{} // closed when stderr reaches its end

	mu     sync.Mutex
	stderr []byte // last 8 KiB; never logged whole
}

func startProc(argv []string, dir string, env []string) (*proc, error) {
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
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = env
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
		return nil, fmt.Errorf("start %s: %w", argv[0], err)
	}
	p := &proc{cmd: cmd, in: inW, out: bufio.NewReaderSize(outR, 64<<10), outF: outR, errOut: errR, exited: make(chan struct{}), errEnd: make(chan struct{})}
	go p.drainStderr()
	go func() {
		_ = cmd.Wait()
		close(p.exited)
		// A grandchild may still hold the pipe; give buffered output a moment, then stop reading.
		if f, ok := p.outF.(*os.File); ok {
			_ = f.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		}
	}()
	return p, nil
}

// killGroup kills the server and everything it started, without waiting.
func (p *proc) killGroup() { _ = syscall.Kill(-p.pid(), syscall.SIGKILL) }

func (p *proc) pid() int { return p.cmd.Process.Pid }

func (p *proc) drainStderr() {
	defer close(p.errEnd)
	buf := make([]byte, 4096)
	for {
		n, err := p.errOut.Read(buf)
		if n > 0 {
			p.mu.Lock()
			p.stderr = append(p.stderr, buf[:n]...)
			if len(p.stderr) > stderrKeep {
				p.stderr = p.stderr[len(p.stderr)-stderrKeep:]
			}
			p.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// waitStderr gives the reader up to d to take the last of stderr from a server that
// ended, so its final words make it into the tail. A grandchild may hold the pipe open.
func (p *proc) waitStderr(d time.Duration) {
	select {
	case <-p.errEnd:
	case <-time.After(d):
	}
}

// stderrTail is the last n bytes of stderr, with credentials removed.
func (p *proc) stderrTail(n int) string {
	p.mu.Lock()
	b := p.stderr
	if len(b) > n {
		b = b[len(b)-n:]
	}
	s := string(b)
	p.mu.Unlock()
	return strings.TrimSpace(scrub(strings.ToValidUTF8(s, "")))
}

// kill kills the whole group at once and waits for the server to be reaped.
func (p *proc) kill() {
	_ = syscall.Kill(-p.pid(), syscall.SIGKILL)
	<-p.exited
	p.closePipes()
}

// finish waits up to grace for a server that was asked to exit, then kills the
// group (always, so grandchildren die too).
func (p *proc) finish(grace time.Duration) {
	select {
	case <-p.exited:
	case <-time.After(grace):
	}
	p.kill()
}

func (p *proc) closePipes() {
	_ = p.in.Close()
	_ = p.outF.Close()
	_ = p.errOut.Close()
}
