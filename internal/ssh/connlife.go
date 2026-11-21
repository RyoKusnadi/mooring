package ssh

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh/agent"
)

func (c *Conn) keepalive() {
	m := c.m
	t := time.NewTicker(m.KeepaliveEvery)
	defer t.Stop()
	misses := 0
	for {
		select {
		case <-c.closed:
			return
		case <-t.C:
		}
		if c.ping() {
			misses = 0
			continue
		}
		if misses++; misses >= KeepaliveMisses {
			m.log.Warn("ssh: keepalive failed", "conn", c.id)
			c.shutdown(fmt.Sprintf("%s stopped answering.", c.hostLabel()))
			return
		}
	}
}

func (c *Conn) ping() bool {
	res := make(chan error, 1)
	go func() {
		_, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil)
		res <- err
	}()
	select {
	case err := <-res:
		return err == nil
	case <-time.After(c.m.KeepaliveWait):
		return false
	case <-c.closed:
		return true
	}
}

// shutdown closes the connection and everything on it, once.
func (c *Conn) shutdown(reason string) {
	c.once.Do(func() {
		close(c.closed)
		c.cancel()
		m := c.m
		m.mu.Lock()
		if m.conns[c.id] == c {
			delete(m.conns, c.id)
		}
		m.mu.Unlock()
		c.mu.Lock()
		client := c.client
		c.state = "closed"
		fwds := make([]*forward, 0, len(c.fwds))
		for _, f := range c.fwds {
			fwds = append(fwds, f)
		}
		c.mu.Unlock()
		for _, f := range fwds {
			f.stop()
		}
		c.sftpMu.Lock()
		if c.sftp != nil {
			_ = c.sftp.Close()
		}
		c.sftpMu.Unlock()
		if client != nil {
			_ = client.Close()
		}
		c.closeAgent()
		if client != nil {
			m.log.Info("ssh: closed", "conn", c.id, "reason", reason)
			m.Pub.Publish("ssh.closed", map[string]string{"id": c.id, "reason": reason})
		}
	})
}

func (c *Conn) closeAgent() {
	c.mu.Lock()
	a := c.agent
	c.agent = nil
	c.mu.Unlock()
	if a != nil {
		_ = a.Close()
	}
}

// startAgentForward lets shells of this connection use the local agent, when the host allows it.
func (c *Conn) startAgentForward() {
	if !c.host.AgentForward {
		return
	}
	sock := c.m.agentSocket()
	if sock == "" {
		return
	}
	nc, err := net.Dial("unix", sock)
	if err != nil {
		c.m.log.Warn("ssh: agent forwarding needs a running agent", "conn", c.id)
		return
	}
	if err := agent.ForwardToAgent(c.client, agent.NewClient(nc)); err != nil {
		_ = nc.Close()
		c.m.log.Warn("ssh: agent forwarding failed", "conn", c.id, "err", err)
		return
	}
	c.mu.Lock()
	c.agent = nc
	c.mu.Unlock()
}

// conn returns a connected connection.
func (m *Manager) conn(id string) (*Conn, error) {
	m.mu.Lock()
	c := m.conns[id]
	m.mu.Unlock()
	if c == nil || c.info().State != "connected" {
		return nil, apiErr("NOT_CONNECTED", "That connection is closed. Connect again.")
	}
	return c, nil
}

// Disconnect closes a connection, or stops one that is still signing in.
func (m *Manager) Disconnect(id string) error {
	m.mu.Lock()
	c := m.conns[id]
	m.mu.Unlock()
	if c == nil {
		return nil
	}
	if c.info().State != "connected" {
		c.cancel()
		return nil
	}
	c.shutdown("Disconnected.")
	return nil
}
