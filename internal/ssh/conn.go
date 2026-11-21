package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"sync"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Connection is what the browser is told about a connection.
type Connection struct {
	ID     string `json:"id"`
	HostID string `json:"hostId"`
	State  string `json:"state"` // connecting or connected
	Since  int64  `json:"since"` // Unix milliseconds
}

// Conn is one SSH connection and what runs on it.
type Conn struct {
	id   string
	host Host
	m    *Manager

	cancel context.CancelFunc // stops a connect that is still waiting
	closed chan struct{}
	once   sync.Once

	mu      sync.Mutex
	state   string
	since   int64
	client  *gossh.Client
	fatal   error
	agent   net.Conn // the local agent, kept while agent forwarding is on
	fwds    map[string]*forward
	algos   []string // host key algorithms to offer (set after a first try found other key types)
	tried   bool     // the restricted try was made
	pending string   // a password to remember once sign-in works
	usedMem bool     // the remembered password was sent

	fwdPending int      // forwards whose port is being opened
	hsConn     net.Conn // the socket while the handshake runs
	sftpMu     sync.Mutex
	sftp       *sftp.Client
	sftpErr    error
}

func (c *Conn) hostLabel() string {
	return fmt.Sprintf("%s@%s:%d", c.host.User, c.host.Address, c.host.Port)
}

func (c *Conn) info() Connection {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Connection{ID: c.id, HostID: c.host.ID, State: c.state, Since: c.since}
}

// setFatal keeps the first error that must end the connect, since the handshake wraps what a callback returns.
func (c *Conn) setFatal(err error) {
	c.mu.Lock()
	if c.fatal == nil {
		c.fatal = err
	}
	c.mu.Unlock()
}

func (c *Conn) getFatal() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fatal
}

// Connect signs in to a saved host as connection id. Prompts go out as events
// while it waits.
func (m *Manager) Connect(ctx context.Context, id, hostID string) (Connection, error) {
	if !idPattern.MatchString(id) {
		return Connection{}, apiErr("BAD_REQUEST", "A connection id needs 1 to 64 letters, digits, - or _.")
	}
	h, err := m.host(hostID)
	if err != nil {
		return Connection{}, err
	}
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c := &Conn{id: id, host: h, m: m, cancel: cancel, closed: make(chan struct{}), state: "connecting",
		since: m.Now().UnixMilli(), fwds: map[string]*forward{}}
	m.mu.Lock()
	switch {
	case m.closed:
		m.mu.Unlock()
		return Connection{}, apiErr("INTERNAL", "The server is shutting down.")
	case m.conns[id] != nil:
		m.mu.Unlock()
		return Connection{}, apiErr("ALREADY_EXISTS", "That connection already exists.")
	case len(m.conns) >= MaxConnections:
		m.mu.Unlock()
		return Connection{}, apiErr("LIMIT_REACHED", "%d connections are open. Close one to open another.", MaxConnections)
	}
	m.conns[id] = c
	m.mu.Unlock()

	client, err := m.dial(cctx, c)
	if err != nil {
		m.drop(c)
		m.log.Warn("ssh: connect failed", "conn", id, "host", c.hostLabel(), "err", err)
		return Connection{}, err
	}
	c.mu.Lock()
	c.client, c.state, c.since = client, "connected", m.Now().UnixMilli()
	pw := c.pending
	c.pending = ""
	c.mu.Unlock()
	m.remember(c, pw)
	c.startAgentForward()
	go c.keepalive()
	go func() {
		err := client.Wait()
		c.shutdown(lostReason(c, err))
	}()
	m.log.Info("ssh: connected", "conn", id, "host", c.hostLabel())
	info := c.info()
	m.Pub.Publish("ssh.connected", info)
	return info, nil
}

func lostReason(c *Conn, err error) string {
	if err == nil || errors.Is(err, context.Canceled) {
		return fmt.Sprintf("The connection to %s ended.", c.hostLabel())
	}
	return fmt.Sprintf("The connection to %s was lost.", c.hostLabel())
}

// remember stores a password that just worked, or clears the flag of one that was refused.
func (m *Manager) remember(c *Conn, pw string) {
	if m.Secrets == nil {
		return
	}
	switch {
	case pw != "":
		if err := m.Secrets.Set(c.host.ID, pw); err != nil {
			m.log.Warn("ssh: could not remember a password", "host", c.host.ID, "err", err)
			return
		}
		m.setRemembered(c.host.ID, true)
	case c.usedMem:
		m.setRemembered(c.host.ID, true)
	}
}

// drop forgets a connection that never got going.
func (m *Manager) drop(c *Conn) {
	m.mu.Lock()
	if m.conns[c.id] == c {
		delete(m.conns, c.id)
	}
	m.mu.Unlock()
	c.once.Do(func() { close(c.closed) })
	c.closeAgent()
}
