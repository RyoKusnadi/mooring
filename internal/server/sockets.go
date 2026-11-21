package server

import (
	"bufio"
	"net"
	"net/http"
	"strings"
	"sync"
)

// sockets remembers the WebSocket connections opened with a session, so that
// when the session keys change (sign out other browsers, `mooring token -new`)
// every one of them can be closed. A cookie that no longer verifies must not
// keep a terminal or an agent chat alive.
type sockets struct {
	mu    sync.Mutex
	conns map[*trackedConn]struct{}
}

func newSockets() *sockets { return &sockets{conns: make(map[*trackedConn]struct{})} }

const closeReason = "Signed out. Sign in again."

// closeAll sends a close frame (code 1008, policy violation) and drops every connection.
func (s *sockets) closeAll() {
	s.mu.Lock()
	all := make([]*trackedConn, 0, len(s.conns))
	for c := range s.conns {
		all = append(all, c)
	}
	s.mu.Unlock()
	frame := append([]byte{0x88, byte(2 + len(closeReason)), 0x03, 0xF0}, closeReason...)
	for _, c := range all {
		_, _ = c.Write(frame) // unmasked, as a server sends it; the connection ends right after
		_ = c.Close()
	}
}

type trackedConn struct {
	net.Conn
	s *sockets
}

func (c *trackedConn) Close() error {
	c.s.mu.Lock()
	delete(c.s.conns, c)
	c.s.mu.Unlock()
	return c.Conn.Close()
}

// trackedWriter lets the WebSocket library hijack the connection, and records it.
type trackedWriter struct {
	http.ResponseWriter
	s *sockets
}

func (w trackedWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	conn, rw, err := h.Hijack()
	if err != nil {
		return nil, nil, err
	}
	tc := &trackedConn{Conn: conn, s: w.s}
	w.s.mu.Lock()
	w.s.conns[tc] = struct{}{}
	w.s.mu.Unlock()
	return tc, rw, nil
}

func (s *sockets) wrap(w http.ResponseWriter, r *http.Request) http.ResponseWriter {
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return trackedWriter{w, s}
	}
	return w
}
