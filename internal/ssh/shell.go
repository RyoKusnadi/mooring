package ssh

import (
	"fmt"
	"io"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Shell is a login shell on a connection. It is a terminal.Backend.
type Shell struct {
	sess *gossh.Session
	in   io.WriteCloser
	out  io.Reader
}

// OpenShell starts a login shell with a PTY on the connection, in the host's
// root folder when it has one.
func (m *Manager) OpenShell(connID string, cols, rows uint16) (*Shell, error) {
	c, err := m.conn(connID)
	if err != nil {
		return nil, err
	}
	ch := make(chan shellResult, 1)
	go func() {
		sh, err := c.startShell(cols, rows)
		ch <- shellResult{sh, err}
	}()
	select {
	case r := <-ch:
		return r.sh, r.err
	case <-time.After(shellOpenTimeout):
		go reap(ch)
		return nil, apiErr("NOT_CONNECTED", "The server did not open a shell in time.")
	case <-c.closed:
		go reap(ch)
		return nil, apiErr("NOT_CONNECTED", "That connection is closed. Connect again.")
	}
}

func (c *Conn) startShell(cols, rows uint16) (*Shell, error) {
	sess, err := c.client.NewSession()
	if err != nil {
		return nil, apiErr("NOT_CONNECTED", "The connection could not open a session: %v.", err)
	}
	fail := func(err error) (*Shell, error) {
		_ = sess.Close()
		c.m.log.Warn("ssh: shell", "conn", c.id, "err", err)
		return nil, apiErr("INTERNAL", "The remote shell could not be started.")
	}
	c.mu.Lock()
	forwarded := c.agent != nil
	c.mu.Unlock()
	if forwarded {
		if err := agent.RequestAgentForwarding(sess); err != nil {
			c.m.log.Warn("ssh: agent forwarding refused", "conn", c.id, "err", err)
		}
	}
	modes := gossh.TerminalModes{gossh.ECHO: 1, gossh.TTY_OP_ISPEED: 38400, gossh.TTY_OP_OSPEED: 38400}
	if err := sess.RequestPty("xterm-256color", int(rows), int(cols), modes); err != nil {
		return fail(err)
	}
	in, err := sess.StdinPipe()
	if err != nil {
		return fail(err)
	}
	out, err := sess.StdoutPipe()
	if err != nil {
		return fail(err)
	}
	if c.host.Root == "" {
		err = sess.Shell()
	} else {
		err = sess.Start(fmt.Sprintf(`cd -- %s && exec "${SHELL:-/bin/sh}" -l`, quote(c.host.Root)))
	}
	if err != nil {
		return fail(err)
	}
	return &Shell{sess: sess, in: in, out: out}, nil
}

// quote makes s one word for a POSIX shell.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func (s *Shell) Read(p []byte) (int, error)  { return s.out.Read(p) }
func (s *Shell) Write(p []byte) (int, error) { return s.in.Write(p) }

// Wait returns the shell's exit status, or 255 when the channel closed without
// one (the connection ended).
func (s *Shell) Wait() int {
	err := s.sess.Wait()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*gossh.ExitError); ok {
		if ee.Signal() != "" {
			return 255
		}
		return ee.ExitStatus()
	}
	return 255
}

func (s *Shell) Resize(cols, rows uint16) error { return s.sess.WindowChange(int(rows), int(cols)) }

// Winch does nothing: a window-change already tells the remote program.
func (s *Shell) Winch() {}

func (s *Shell) Hangup() { _ = s.sess.Close() }
func (s *Shell) Kill()   { _ = s.sess.Close() }

// Close releases the session.
func (s *Shell) Close() error {
	_ = s.in.Close()
	return s.sess.Close()
}

type shellResult struct {
	sh  *Shell
	err error
}

// reap closes a shell that finished opening after nobody was waiting for it.
func reap(ch <-chan shellResult) {
	if r := <-ch; r.sh != nil {
		_ = r.sh.Close()
	}
}
