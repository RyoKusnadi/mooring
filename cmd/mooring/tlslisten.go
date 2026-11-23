package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"
)

// handshakeListener finishes each TLS handshake before Accept hands the
// connection to the HTTP server, within a time limit, so a client that connects
// and stays silent is dropped. net/http limits a handshake only through
// ReadTimeout, which Mooring cannot set because its sockets stay open.
type handshakeListener struct {
	net.Listener
	config  *tls.Config
	timeout time.Duration
	conns   chan net.Conn
	errs    chan error
	done    chan struct{}
	once    sync.Once
}

func newHandshakeListener(l net.Listener, cfg *tls.Config, timeout time.Duration) *handshakeListener {
	h := &handshakeListener{Listener: l, config: cfg, timeout: timeout,
		conns: make(chan net.Conn), errs: make(chan error), done: make(chan struct{})}
	go h.loop()
	return h
}

func (h *handshakeListener) loop() {
	for {
		c, err := h.Listener.Accept()
		if err != nil {
			// The HTTP server decides: it retries a temporary error and stops on others.
			select {
			case h.errs <- err:
			case <-h.done:
				return
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		go h.handshake(c)
	}
}

func (h *handshakeListener) handshake(c net.Conn) {
	tc := tls.Server(c, h.config)
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()
	if err := tc.HandshakeContext(ctx); err != nil {
		_ = c.Close()
		return
	}
	select {
	case h.conns <- tc:
	case <-h.done:
		_ = tc.Close()
	}
}

// Accept returns the next connection whose handshake has finished.
func (h *handshakeListener) Accept() (net.Conn, error) {
	select {
	case c := <-h.conns:
		return c, nil
	case err := <-h.errs:
		return nil, err
	case <-h.done:
		return nil, net.ErrClosed
	}
}

func (h *handshakeListener) Close() error {
	h.once.Do(func() { close(h.done) })
	return h.Listener.Close()
}
