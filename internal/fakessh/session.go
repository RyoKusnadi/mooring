package fakessh

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type ptyReq struct {
	Term          string
	Cols, Rows    uint32
	Width, Height uint32
	Modes         string
}

type winch struct{ Cols, Rows, Width, Height uint32 }

// session answers the requests of one session channel.
func (s *Server) session(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer func() { _ = ch.Close() }()
	var p *ptyReq
	var sh *shellRun
	var wg sync.WaitGroup
	for r := range reqs {
		ok := true
		switch r.Type {
		case "pty-req":
			var q ptyReq
			if ssh.Unmarshal(r.Payload, &q) != nil {
				ok = false
				break
			}
			p = &q
			s.setSize(q.Cols, q.Rows)
		case "env":
		case "auth-agent-req@openssh.com":
			s.mu.Lock()
			s.agentReq++
			s.mu.Unlock()
		case "window-change":
			var w winch
			if ssh.Unmarshal(r.Payload, &w) == nil {
				s.setSize(w.Cols, w.Rows)
				if sh != nil {
					sh.resize(w.Cols, w.Rows)
				}
			}
		case "shell", "exec":
			if sh != nil {
				ok = false
				break
			}
			cmd := ""
			if r.Type == "exec" {
				var e struct{ Command string }
				_ = ssh.Unmarshal(r.Payload, &e)
				cmd = e.Command
				s.mu.Lock()
				s.execs = append(s.execs, cmd)
				s.mu.Unlock()
			}
			var err error
			if sh, err = s.startShell(ch, p, cmd); err != nil {
				ok = false
				break
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				sh.run(ch)
			}()
		case "subsystem":
			var sub struct{ Name string }
			_ = ssh.Unmarshal(r.Payload, &sub)
			if sub.Name != "sftp" || s.o.NoSFTP {
				ok = false
				break
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.sftp(ch)
			}()
		default:
			ok = false
		}
		if r.WantReply {
			_ = r.Reply(ok, nil)
		}
	}
	if sh != nil {
		sh.stop()
	}
	wg.Wait()
}

func (s *Server) setSize(cols, rows uint32) {
	s.mu.Lock()
	s.cols, s.rows = cols, rows
	s.mu.Unlock()
}

func (s *Server) sftp(ch ssh.Channel) {
	srv, err := sftp.NewServer(ch, sftp.WithServerWorkingDirectory(s.o.Home))
	if err != nil {
		return
	}
	_ = srv.Serve()
	_ = srv.Close()
}

// shellRun is a started shell: the echo shell, or sh on a PTY.
type shellRun struct {
	s    *Server
	echo bool
	f    *os.File
	cmd  *exec.Cmd
	cols uint32
	rows uint32
	done chan struct{}
	once sync.Once
}

func (s *Server) startShell(ch ssh.Channel, p *ptyReq, command string) (*shellRun, error) {
	sh := &shellRun{s: s, done: make(chan struct{})}
	if p != nil {
		sh.cols, sh.rows = p.Cols, p.Rows
	}
	if s.o.Shell == "" {
		sh.echo = true
		return sh, nil
	}
	args := []string{}
	if command != "" {
		args = []string{"-c", command}
	}
	sh.cmd = exec.Command("/bin/sh", args...)
	sh.cmd.Dir = s.o.Home
	sh.cmd.Env = append(os.Environ(), "SHELL=/bin/sh", "HOME="+s.o.Home, "TERM=xterm-256color")
	ws := &pty.Winsize{Rows: uint16(max(sh.rows, 24)), Cols: uint16(max(sh.cols, 80))}
	f, err := pty.StartWithSize(sh.cmd, ws)
	if err != nil {
		return nil, fmt.Errorf("fakessh: start shell: %w", err)
	}
	sh.f = f
	return sh, nil
}

func (sh *shellRun) resize(cols, rows uint32) {
	sh.cols, sh.rows = cols, rows
	if sh.f != nil {
		_ = pty.Setsize(sh.f, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	}
}

func (sh *shellRun) stop() {
	sh.once.Do(func() {
		close(sh.done)
		if sh.cmd != nil && sh.cmd.Process != nil {
			_ = sh.cmd.Process.Kill()
		}
		if sh.f != nil {
			_ = sh.f.Close()
		}
	})
}

func exitStatus(ch ssh.Channel, code uint32) {
	_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{code}))
}

func (sh *shellRun) run(ch ssh.Channel) {
	defer sh.stop()
	if sh.echo {
		sh.runEcho(ch)
		_ = ch.Close()
		return
	}
	go func() { _, _ = io.Copy(sh.f, ch) }()
	_, _ = io.Copy(ch, sh.f)
	code := uint32(0)
	if err := sh.cmd.Wait(); err != nil {
		code = 1
		if ee, ok := err.(*exec.ExitError); ok {
			code = uint32(max(ee.ExitCode(), 1))
		}
	}
	exitStatus(ch, code)
	_ = ch.Close()
}

// runEcho echoes every key, and answers a typed line: "size" tells the pty
// size, "exit" or "exit N" ends the shell, anything else comes back as "got:<line>".
func (sh *shellRun) runEcho(ch ssh.Channel) {
	_, _ = io.WriteString(ch, "fake$ ")
	br := bufio.NewReader(ch)
	var line []byte
	for {
		b, err := br.ReadByte()
		if err != nil {
			return
		}
		if b != '\r' && b != '\n' {
			line = append(line, b)
			_, _ = ch.Write([]byte{b})
			continue
		}
		text := string(line)
		line = line[:0]
		_, _ = io.WriteString(ch, "\r\n")
		switch {
		case text == "size":
			_, _ = fmt.Fprintf(ch, "size=%dx%d\r\n", sh.cols, sh.rows)
		case text == "exit" || strings.HasPrefix(text, "exit "):
			var n uint32
			_, _ = fmt.Sscanf(strings.TrimPrefix(text, "exit"), "%d", &n)
			exitStatus(ch, n)
			return
		default:
			_, _ = fmt.Fprintf(ch, "got:%s\r\n", text)
		}
		_, _ = io.WriteString(ch, "fake$ ")
	}
}
