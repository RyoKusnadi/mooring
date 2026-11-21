package terminal

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type sock struct {
	t   *testing.T
	ws  *websocket.Conn
	ctx context.Context
}

func dial(t *testing.T, srv *httptest.Server, id string, hello string) *sock {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/terminals/"+id, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = ws.CloseNow() })
	s := &sock{t: t, ws: ws, ctx: ctx}
	if err := ws.Write(ctx, websocket.MessageText, []byte(hello)); err != nil {
		t.Fatal(err)
	}
	return s
}

// frame returns the next frame: JSON text decoded to a map, or binary as (pos, bytes).
func (s *sock) frame() (map[string]any, uint64, []byte) {
	s.t.Helper()
	typ, data, err := s.ws.Read(s.ctx)
	if err != nil {
		s.t.Fatalf("read: %v", err)
	}
	if typ == websocket.MessageBinary {
		return nil, binary.BigEndian.Uint64(data), data[8:]
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		s.t.Fatalf("frame %q: %v", data, err)
	}
	return m, 0, nil
}

func (s *sock) until(match func(m map[string]any, pos uint64, b []byte) bool) {
	s.t.Helper()
	for {
		m, pos, b := s.frame()
		if match(m, pos, b) {
			return
		}
	}
}

func TestSocketEchoResizeAndSecondClient(t *testing.T) {
	m, _ := newManager(t, Config{Shell: "/bin/cat", Args: []string{}})
	mux := http.NewServeMux()
	mux.Handle("GET /api/terminals/{id}", m.Handler())
	srv := httptest.NewServer(mux)
	defer srv.Close()
	open(t, m, "a")

	a := dial(t, srv, "a", `{"t":"hello","epoch":"","pos":0}`)
	hello, _, _ := a.frame()
	if hello["t"] != "hello" || hello["reset"] != true || hello["cols"] != float64(80) || hello["others"] != float64(0) {
		t.Fatalf("hello %v", hello)
	}
	epoch := hello["epoch"].(string)

	if err := a.ws.Write(a.ctx, websocket.MessageBinary, []byte("ping\n")); err != nil {
		t.Fatal(err)
	}
	var got []byte
	var firstPos uint64
	a.until(func(m map[string]any, pos uint64, b []byte) bool {
		if b != nil {
			if got == nil {
				firstPos = pos
			}
			got = append(got, b...)
		}
		return bytes.Count(got, []byte("ping")) >= 2 // the tty echo and cat's own output
	})
	if firstPos != 0 {
		t.Errorf("first output position %d, want 0", firstPos)
	}

	// A second client resumes at the end, sees one other client, and is told about a resize.
	end := uint64(len(got))
	b := dial(t, srv, "a", `{"t":"hello","epoch":"`+epoch+`","pos":`+itoa(end)+`}`)
	h2, _, _ := b.frame()
	if h2["reset"] != false || h2["others"] != float64(1) || h2["pos"] != float64(end) {
		t.Fatalf("second hello %v", h2)
	}
	if err := a.ws.Write(a.ctx, websocket.MessageText, []byte(`{"t":"resize","cols":100,"rows":30}`)); err != nil {
		t.Fatal(err)
	}
	want := func(m map[string]any, _ uint64, _ []byte) bool {
		return m != nil && m["t"] == "size" && m["cols"] == float64(100) && m["rows"] == float64(30)
	}
	a.until(want)
	b.until(want)

	// The last to resize wins; the other is told, and a repeat of the same size settles.
	if err := b.ws.Write(b.ctx, websocket.MessageText, []byte(`{"t":"resize","cols":90,"rows":20}`)); err != nil {
		t.Fatal(err)
	}
	a.until(func(m map[string]any, _ uint64, _ []byte) bool { return m != nil && m["cols"] == float64(90) })
	if info := m.get("a").info(); info.Cols != 90 || info.Rows != 20 {
		t.Errorf("pty size %dx%d", info.Cols, info.Rows)
	}

	if err := a.ws.Write(a.ctx, websocket.MessageText, []byte(`{"t":"ping"}`)); err != nil {
		t.Fatal(err)
	}
	a.until(func(m map[string]any, _ uint64, _ []byte) bool { return m != nil && m["t"] == "pong" })
}

func TestSocketUnknownTerminalIs404(t *testing.T) {
	m, _ := newManager(t, Config{})
	mux := http.NewServeMux()
	mux.Handle("GET /api/terminals/{id}", m.Handler())
	srv := httptest.NewServer(mux)
	defer srv.Close()
	res, err := http.Get(srv.URL + "/api/terminals/nope")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status %d", res.StatusCode)
	}
}

func TestSocketSilentClientIsDropped(t *testing.T) {
	m, _ := newManager(t, Config{Shell: "/bin/cat", Args: []string{}, Idle: 200 * time.Millisecond})
	mux := http.NewServeMux()
	mux.Handle("GET /api/terminals/{id}", m.Handler())
	srv := httptest.NewServer(mux)
	defer srv.Close()
	term := open(t, m, "a")
	a := dial(t, srv, "a", `{"t":"hello","epoch":"","pos":0}`)
	a.frame()
	waitFor(t, "the silent client to be dropped", func() bool {
		term.mu.Lock()
		defer term.mu.Unlock()
		return len(term.clients) == 0
	})
}

func itoa(n uint64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestSlowRemoteOpenDoesNotBlockOtherTerminals(t *testing.T) {
	release := make(chan struct{})
	m, _ := newManager(t, Config{Remote: func(string, uint16, uint16) (Backend, error) {
		<-release
		return nil, apiErr("NOT_CONNECTED", "gone")
	}})
	done := make(chan error, 1)
	go func() { _, err := m.OpenSSH("p1", "c1", "r1", 80, 24); done <- err }()
	time.Sleep(100 * time.Millisecond)
	opened := make(chan struct{})
	go func() { m.Snapshot(); _, _ = m.Open("p1", "", "l1", 80, 24); close(opened) }()
	select {
	case <-opened:
	case <-time.After(3 * time.Second):
		t.Fatal("a pending remote open froze the manager")
	}
	close(release)
	if err := <-done; err == nil {
		t.Error("expected the remote error")
	}
}
