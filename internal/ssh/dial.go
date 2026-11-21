package ssh

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

// dial opens the TCP connection and signs in.
func (m *Manager) dial(ctx context.Context, c *Conn) (*gossh.Client, error) {
	addr := net.JoinHostPort(c.host.Address, strconv.Itoa(c.host.Port))
	d := net.Dialer{Timeout: m.DialTimeout}
	nc, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		if ctx.Err() != nil {
			return nil, apiErr("CANCELLED", "The connection was stopped.")
		}
		return nil, apiErr("HOST_UNREACHABLE", "Could not reach %s: %s.", addr, dialReason(err))
	}
	auth, closeAuth := m.authMethods(ctx, c)
	defer closeAuth()
	cfg := &gossh.ClientConfig{
		User: c.host.User, Auth: auth,
		HostKeyCallback:   m.hostKeyCallback(ctx, c),
		HostKeyAlgorithms: c.hostAlgos(),
	}
	stop := context.AfterFunc(ctx, func() { _ = nc.Close() })
	c.mu.Lock()
	c.hsConn = nc
	c.mu.Unlock()
	c.handshakeClock(true)
	sc, chans, reqs, err := gossh.NewClientConn(nc, addr, cfg)
	stop()
	c.mu.Lock()
	c.hsConn = nil
	c.mu.Unlock()
	if err == nil {
		_ = nc.SetDeadline(time.Time{})
	}
	if err != nil {
		_ = nc.Close()
		if c.retryNeeded(err) {
			return m.dial(ctx, c)
		}
		return nil, m.dialError(ctx, c, addr, err)
	}
	return gossh.NewClient(sc, chans, reqs), nil
}

// tryKnownTypes asks for a second handshake that offers only the key types the
// known_hosts files hold for this host. It is false when that was already tried.
func (c *Conn) tryKnownTypes(types []string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tried {
		return false
	}
	c.tried = true
	for _, t := range types {
		if t == "ssh-rsa" {
			c.algos = append(c.algos, "rsa-sha2-512", "rsa-sha2-256")
		} else {
			c.algos = append(c.algos, t)
		}
	}
	return true
}

func (c *Conn) hostAlgos() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.algos
}

// retryNeeded is true once, when the handshake stopped to offer other key types
// (or, in the restricted try, the server has none of them: then it is asked again freely).
func (c *Conn) retryNeeded(err error) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if errors.Is(c.fatal, errRetry) {
		c.fatal = nil
		return true
	}
	if c.fatal == nil && len(c.algos) > 0 && strings.Contains(err.Error(), "no common algorithm") {
		c.algos = nil
		return true
	}
	return false
}

func dialReason(err error) string {
	var dns *net.DNSError
	var ne net.Error
	switch {
	case errors.As(err, &dns):
		return "the name was not found"
	case errors.As(err, &ne) && ne.Timeout():
		return "it did not answer in time"
	}
	return "the connection was refused or unreachable"
}

var attempted = regexp.MustCompile(`attempted methods \[([^\]]*)\]`)

func (m *Manager) dialError(ctx context.Context, c *Conn, addr string, err error) error {
	if f := c.getFatal(); f != nil {
		return f
	}
	if ctx.Err() != nil {
		return apiErr("CANCELLED", "The connection was stopped.")
	}
	if strings.Contains(err.Error(), "unable to authenticate") {
		tried := []string{}
		if sm := attempted.FindStringSubmatch(err.Error()); sm != nil {
			for _, name := range strings.Fields(sm[1]) {
				if name != "none" {
					tried = append(tried, name)
				}
			}
		}
		list := "none that apply"
		if len(tried) > 0 {
			list = strings.Join(tried, ", ")
		}
		return apiErr("AUTH_FAILED", "%s refused the sign-in for %s. Tried: %s.", addr, c.host.User, list)
	}
	return apiErr("HOST_UNREACHABLE", "The connection to %s failed while signing in. The server may not be an SSH server.", addr)
}

// handshakeClock starts a fresh handshake deadline (on) or stops it (off, while a prompt waits for the user).
func (c *Conn) handshakeClock(on bool) {
	c.mu.Lock()
	nc := c.hsConn
	c.mu.Unlock()
	if nc == nil {
		return
	}
	if on {
		_ = nc.SetDeadline(time.Now().Add(c.m.DialTimeout))
	} else {
		_ = nc.SetDeadline(time.Time{})
	}
}
