package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"syscall"
)

// Forward is a local port that carries connections to an address on the remote side.
type Forward struct {
	ID         string `json:"id"`
	Conn       string `json:"conn"`
	LocalPort  int    `json:"localPort"`
	RemoteHost string `json:"remoteHost"`
	RemotePort int    `json:"remotePort"`
}

type forward struct {
	Forward
	c    *Conn
	ln   net.Listener
	once sync.Once

	mu    sync.Mutex
	done  bool
	pairs map[net.Conn]struct{}
}

func (c *Conn) forwardList() []Forward {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Forward, 0, len(c.fwds))
	for _, f := range c.fwds {
		out = append(out, f.Forward)
	}
	return out
}

// Forward listens on bind (a loopback address) and port, and carries each
// connection to remoteHost:remotePort as the remote side sees it.
func (m *Manager) Forward(connID, bind string, localPort int, remoteHost string, remotePort int) (Forward, error) {
	c, err := m.conn(connID)
	if err != nil {
		return Forward{}, err
	}
	if ip := net.ParseIP(bind); ip == nil || !ip.IsLoopback() {
		return Forward{}, apiErr("BAD_REQUEST", "A forward listens on this computer's loopback address (127.0.0.1) only.")
	}
	if localPort < 0 || localPort > 65535 || remotePort < 1 || remotePort > 65535 || remoteHost == "" || !plain(remoteHost) || len(remoteHost) > 253 {
		return Forward{}, apiErr("BAD_REQUEST", "The ports and the remote host are not valid.")
	}
	c.mu.Lock()
	if len(c.fwds)+c.fwdPending >= MaxForwards {
		c.mu.Unlock()
		return Forward{}, apiErr("LIMIT_REACHED", "%d forwards are open on this connection. Stop one to open another.", MaxForwards)
	}
	c.fwdPending++ // a place is kept while the port opens
	c.mu.Unlock()
	ln, err := net.Listen("tcp", net.JoinHostPort(bind, strconv.Itoa(localPort)))
	if err != nil {
		c.mu.Lock()
		c.fwdPending--
		c.mu.Unlock()
		if errors.Is(err, syscall.EADDRINUSE) {
			return Forward{}, apiErr("ALREADY_EXISTS", "Port %d on this computer is already in use.", localPort)
		}
		m.log.Warn("ssh: forward listen", "err", err)
		return Forward{}, apiErr("INTERNAL", "The port could not be opened.")
	}
	m.mu.Lock()
	m.nextID++
	id := fmt.Sprintf("f%d", m.nextID)
	m.mu.Unlock()
	f := &forward{c: c, ln: ln, pairs: map[net.Conn]struct{}{}}
	f.Forward = Forward{ID: id, Conn: connID, LocalPort: ln.Addr().(*net.TCPAddr).Port, RemoteHost: remoteHost, RemotePort: remotePort}
	c.mu.Lock()
	c.fwdPending--
	select {
	case <-c.closed:
		c.mu.Unlock()
		_ = ln.Close()
		return Forward{}, apiErr("NOT_CONNECTED", "That connection is closed. Connect again.")
	default:
	}
	c.fwds[id] = f
	c.mu.Unlock()
	go f.serve()
	m.log.Info("ssh: forward", "conn", connID, "local", f.LocalPort, "remote", net.JoinHostPort(remoteHost, strconv.Itoa(remotePort)))
	m.Pub.Publish("ssh.forwarded", f.Forward)
	return f.Forward, nil
}

// StopForward closes a forward and the connections it carries.
func (m *Manager) StopForward(id string) {
	m.mu.Lock()
	conns := make([]*Conn, 0, len(m.conns))
	for _, c := range m.conns {
		conns = append(conns, c)
	}
	m.mu.Unlock()
	for _, c := range conns {
		c.mu.Lock()
		f := c.fwds[id]
		c.mu.Unlock()
		if f != nil {
			f.stop()
			return
		}
	}
}

func (f *forward) stop() {
	f.once.Do(func() {
		_ = f.ln.Close()
		f.mu.Lock()
		f.done = true
		for p := range f.pairs {
			_ = p.Close()
		}
		f.mu.Unlock()
		f.c.mu.Lock()
		delete(f.c.fwds, f.ID)
		f.c.mu.Unlock()
		f.c.m.Pub.Publish("ssh.forwardStopped", map[string]string{"id": f.ID})
	})
}

func (f *forward) serve() {
	for {
		nc, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.carry(nc)
	}
}

func (f *forward) carry(local net.Conn) {
	remote, err := f.c.client.Dial("tcp", net.JoinHostPort(f.RemoteHost, strconv.Itoa(f.RemotePort)))
	if err != nil {
		f.c.m.log.Info("ssh: forward target refused", "forward", f.ID, "err", err)
		_ = local.Close()
		return
	}
	f.mu.Lock()
	if f.done { // stopped while dialling
		f.mu.Unlock()
		_ = local.Close()
		_ = remote.Close()
		return
	}
	f.pairs[local], f.pairs[remote] = struct{}{}, struct{}{}
	f.mu.Unlock()
	var wg sync.WaitGroup
	wg.Add(2)
	pipe := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		} else {
			_ = dst.Close()
		}
	}
	go pipe(remote, local)
	go pipe(local, remote)
	wg.Wait()
	_ = local.Close()
	_ = remote.Close()
	f.mu.Lock()
	delete(f.pairs, local)
	delete(f.pairs, remote)
	f.mu.Unlock()
}
