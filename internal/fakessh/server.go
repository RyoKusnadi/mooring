// Package fakessh is a stand-in SSH server for tests: password, public key and
// keyboard-interactive sign-in, a shell, SFTP and direct-tcpip. It never
// touches the real ~/.ssh. Tests run it in the same process on 127.0.0.1:0;
// the e2e starts it as a program through the package's test binary (see Run).
package fakessh

import (
	"bytes"
	"fmt"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

// Question is one keyboard-interactive prompt and the answer the server wants.
type Question struct {
	Text   string
	Echo   bool
	Answer string
}

// Options sets what the server accepts. A method is on when its option is set.
type Options struct {
	Addr           string // "" means 127.0.0.1:0
	HostKey        ssh.Signer
	User           string
	Password       string
	AuthorizedKeys []ssh.PublicKey
	Questions      []Question
	// Home is the SFTP working directory and where a real shell starts.
	Home string
	// Shell is "" for a line-echo shell, or "sh" for /bin/sh on a PTY.
	Shell  string
	NoSFTP bool
}

// Server is a running fake server.
type Server struct {
	o   Options
	ln  net.Listener
	cfg *ssh.ServerConfig

	mu       sync.Mutex
	conns    map[net.Conn]struct{} // from Accept, so a drop also reaches one still in its handshake
	attempts []string
	agentReq int
	execs    []string
	cols     uint32
	rows     uint32
	paused   chan struct{} // closed when running; a global request waits on it
	closed   bool
	wg       sync.WaitGroup
}

// Start listens and serves in the background.
func Start(o Options) (*Server, error) {
	if o.Addr == "" {
		o.Addr = "127.0.0.1:0"
	}
	if o.HostKey == nil {
		k, err := GenerateKey()
		if err != nil {
			return nil, err
		}
		o.HostKey = k
	}
	ln, err := net.Listen("tcp", o.Addr)
	if err != nil {
		return nil, fmt.Errorf("fakessh: listen: %w", err)
	}
	s := &Server{o: o, ln: ln, conns: map[net.Conn]struct{}{}, paused: make(chan struct{})}
	close(s.paused)
	s.cfg = s.serverConfig()
	s.wg.Add(1)
	go s.accept()
	return s, nil
}

func (s *Server) serverConfig() *ssh.ServerConfig {
	cfg := &ssh.ServerConfig{}
	cfg.AddHostKey(s.o.HostKey)
	if s.o.Password != "" {
		cfg.PasswordCallback = func(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
			s.note("password")
			if c.User() == s.o.User && string(pw) == s.o.Password {
				return nil, nil
			}
			return nil, fmt.Errorf("wrong password")
		}
	}
	if len(s.o.AuthorizedKeys) > 0 {
		cfg.PublicKeyCallback = func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			s.note("publickey")
			for _, k := range s.o.AuthorizedKeys {
				if c.User() == s.o.User && bytes.Equal(k.Marshal(), key.Marshal()) {
					return nil, nil
				}
			}
			return nil, fmt.Errorf("key not authorized")
		}
	}
	if len(s.o.Questions) > 0 {
		cfg.KeyboardInteractiveCallback = func(c ssh.ConnMetadata, ch ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			s.note("keyboard-interactive")
			for _, q := range s.o.Questions {
				ans, err := ch(c.User(), "", []string{q.Text}, []bool{q.Echo})
				if err != nil {
					return nil, err
				}
				if len(ans) != 1 || ans[0] != q.Answer || c.User() != s.o.User {
					return nil, fmt.Errorf("wrong answer")
				}
			}
			return nil, nil
		}
	}
	return cfg
}

func (s *Server) note(method string) {
	s.mu.Lock()
	s.attempts = append(s.attempts, method)
	s.mu.Unlock()
}

// Addr is the address the server listens on.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Port is the TCP port.
func (s *Server) Port() int { return s.ln.Addr().(*net.TCPAddr).Port }

// HostKey is the server's public key.
func (s *Server) HostKey() ssh.PublicKey { return s.o.HostKey.PublicKey() }

// Attempts lists the sign-in methods clients tried, in order.
func (s *Server) Attempts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.attempts...)
}

// AgentRequests counts agent forwarding requests on shells.
func (s *Server) AgentRequests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agentReq
}

// Execs lists the commands of exec requests.
func (s *Server) Execs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.execs...)
}

// Size is the last pty size a client asked for.
func (s *Server) Size() (cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int(s.cols), int(s.rows)
}

// Pause stops answering global requests (keepalives) until Resume.
func (s *Server) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.paused:
		s.paused = make(chan struct{})
	default:
	}
}

// Resume answers global requests again.
func (s *Server) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.paused:
	default:
		close(s.paused)
	}
}

// DropConnections closes every client connection but keeps listening.
func (s *Server) DropConnections() {
	s.mu.Lock()
	all := make([]net.Conn, 0, len(s.conns))
	for c := range s.conns {
		all = append(all, c)
	}
	s.mu.Unlock()
	for _, c := range all {
		_ = c.Close()
	}
}

// Close stops the server and its connections.
func (s *Server) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	_ = s.ln.Close()
	s.Resume()
	s.DropConnections()
	s.wg.Wait()
}

func (s *Server) accept() {
	defer s.wg.Done()
	for {
		nc, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.serve(nc)
		}()
	}
}

func (s *Server) serve(nc net.Conn) {
	s.mu.Lock()
	s.conns[nc] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.conns, nc)
		s.mu.Unlock()
		_ = nc.Close()
	}()
	sc, chans, reqs, err := ssh.NewServerConn(nc, s.cfg)
	if err != nil {
		return
	}
	defer func() { _ = sc.Close() }()
	go func() {
		for r := range reqs {
			s.mu.Lock()
			gate := s.paused
			s.mu.Unlock()
			<-gate
			if r.WantReply {
				_ = r.Reply(false, nil)
			}
		}
	}()
	var wg sync.WaitGroup
	for nch := range chans {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.channel(nch)
		}()
	}
	wg.Wait()
}

func (s *Server) channel(nch ssh.NewChannel) {
	switch nch.ChannelType() {
	case "session":
		ch, reqs, err := nch.Accept()
		if err != nil {
			return
		}
		s.session(ch, reqs)
	case "direct-tcpip":
		s.directTCP(nch)
	default:
		_ = nch.Reject(ssh.UnknownChannelType, "not supported")
	}
}
