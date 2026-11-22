package server

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// hijacker is a ResponseWriter whose Hijack hands out one end of a pipe.
type hijacker struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (h hijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.conn, bufio.NewReadWriter(bufio.NewReader(h.conn), bufio.NewWriter(h.conn)), nil
}

// The library sends its 101 reply inside Hijack, so the client can be connected, and the
// keys can change, before the socket is recorded. Such a socket must still be closed.
func TestASocketUpgradedAfterARotationIsClosed(t *testing.T) {
	s := newSockets()
	r := httptest.NewRequest("GET", "/api/events", nil)
	r.Header.Set("Upgrade", "websocket")
	server, client := net.Pipe()
	defer func() { _ = client.Close() }()
	w := s.wrap(hijacker{httptest.NewRecorder(), server}, r)
	s.closeAll() // the rotation lands between the cookie check and the hijack

	got := make(chan []byte, 1)
	go func() { b, _ := io.ReadAll(client); got <- b }()
	if _, _, err := w.(http.Hijacker).Hijack(); err != nil {
		t.Fatal(err)
	}
	if b := <-got; !bytes.Equal(b, closeFrame) {
		t.Fatalf("the client got %q, want the close frame", b)
	}
	if len(s.conns) != 0 {
		t.Fatalf("%d sockets kept after a late upgrade", len(s.conns))
	}
}
