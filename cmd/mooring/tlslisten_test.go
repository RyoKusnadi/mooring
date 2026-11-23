package main

import (
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"
)

func TestSilentTLSClientIsDropped(t *testing.T) {
	cert, key := writeCert(t, "localhost")
	s, err := loadTLS(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h := newHandshakeListener(l, s.config, 200*time.Millisecond)
	defer func() { _ = h.Close() }()

	silent, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = silent.Close() }()
	_ = silent.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := silent.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("a silent client must be closed, read gave %v", err)
	}

	// A client that does the handshake is accepted.
	go func() {
		c, err := tls.Dial("tcp", l.Addr().String(), &tls.Config{ServerName: "localhost", InsecureSkipVerify: true}) //nolint:gosec // test certificate
		if err == nil {
			_ = c.Close()
		}
	}()
	c, err := h.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.(*tls.Conn); !ok {
		t.Fatalf("Accept gave %T, want *tls.Conn so the server sees TLS", c)
	}
	_ = c.Close()
	_ = h.Close()
	if _, err := h.Accept(); err == nil {
		t.Fatal("Accept after Close must fail")
	}
}
