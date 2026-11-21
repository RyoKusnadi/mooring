package terminal

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// The tail of the output must reach the ring even when the active client has
// not acked and the shell exits while the reader is paused.
func TestExitWhilePausedKeepsTail(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("on BSD a shell cannot exit until its output is read, so the tail is never at risk")
	}
	cfg := script("head -c 2000 /dev/zero | tr '\\0' x; sleep 0.4; echo YYY; sleep 0.3; echo ENDMARK")
	cfg.FlowWindow = 1024
	m, _ := newManager(t, cfg)
	term := open(t, m, "a")
	waitFor(t, "first output", func() bool { return term.pos() >= 2000 })
	term.attach(term.stream.Epoch(), 0) // never acks: the reader pauses after the next read
	select {
	case <-term.done:
	case <-time.After(5 * time.Second):
		t.Fatal("never exited")
	}
	if !strings.Contains(term.ringText(), "ENDMARK") {
		t.Errorf("the tail was lost: %d bytes in the ring", len(term.ringText()))
	}
}

// Reading acks must never wait on the PTY: a big paste into a shell that is
// not reading must not stop the socket from answering.
func TestPasteDoesNotStallSocket(t *testing.T) {
	m, _ := newManager(t, script("sleep 30"))
	mux := http.NewServeMux()
	mux.Handle("GET /api/terminals/{id}", m.Handler())
	srv := httptest.NewServer(mux)
	defer srv.Close()
	open(t, m, "a")
	s := dial(t, srv, "a", `{"t":"hello","epoch":"","pos":0}`)
	s.frame()
	chunk := make([]byte, 64<<10)
	for range 16 {
		if err := s.ws.Write(s.ctx, websocket.MessageBinary, chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ws.Write(s.ctx, websocket.MessageText, []byte(`{"t":"ping"}`)); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		s.until(func(m map[string]any, _ uint64, _ []byte) bool { return m != nil && m["t"] == "pong" })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("no pong while the paste was pending")
	}
}

// A window that just attached and has not acked must not pause the shell of
// the window that is typing.
func TestFlowGateIsTheTypist(t *testing.T) {
	cfg := script("sleep 0.3; yes")
	cfg.FlowWindow = 64 << 10
	m, _ := newManager(t, cfg)
	term := open(t, m, "a")
	a, _ := term.attach(term.stream.Epoch(), 0)
	term.input(a, nil)
	term.attach(term.stream.Epoch(), 0) // b: never acks
	deadline := time.Now().Add(5 * time.Second)
	for term.pos() < 1<<20 {
		if time.Now().After(deadline) {
			t.Fatalf("stalled at %d bytes behind a client that is not typing", term.pos())
		}
		drain(a)
		term.ack(a, a.cursor)
		time.Sleep(5 * time.Millisecond)
	}
}

// A terminal opened while its project is being removed must not outlive it.
func TestOpenRacesCloseProject(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	dir := t.TempDir()
	cfg := script("sleep 30")
	cfg.Dir = func(string) (string, error) {
		close(entered)
		<-release
		return dir, nil
	}
	m, _ := newManager(t, cfg)
	m.Dir = cfg.Dir
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = m.Open("p1", "", "a", 80, 24) }()
	<-entered
	go func() { defer wg.Done(); m.CloseProject("p1") }()
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()
	if n := len(m.Snapshot()); n != 0 {
		t.Errorf("%d terminals left in a removed project", n)
	}
}
