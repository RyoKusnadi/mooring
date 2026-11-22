package events

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/store"
)

// fakeCommands has "count" (returns how many times it has run) and "block"
// (starts, then waits for release).
type fakeCommands struct {
	runs    atomic.Int32
	started chan struct{}
	release chan struct{}
}

func newFakeCommands() *fakeCommands {
	return &fakeCommands{started: make(chan struct{}, 8), release: make(chan struct{})}
}

func (f *fakeCommands) Secret(string) bool { return false }

func (f *fakeCommands) Run(ctx context.Context, cmd Command) (any, error) {
	n := f.runs.Add(1)
	if cmd.Name == "block" {
		f.started <- struct{}{}
		select {
		case <-f.release:
		case <-ctx.Done():
		}
	}
	return map[string]int32{"n": n}, nil
}

type rig struct {
	hub *Hub
	led *ledger.Ledger
	srv *httptest.Server
}

func newRig(t *testing.T, st *store.Store, cmds Commands) *rig {
	t.Helper()
	hub, err := NewHub(nil)
	if err != nil {
		t.Fatal(err)
	}
	led, err := ledger.New(context.Background(), st.DB, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := Handler(Config{Hub: hub, Ledger: led, Commands: cmds, Snapshot: func() (any, error) {
		return map[string]string{"state": "full"}, nil
	}})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &rig{hub: hub, led: led, srv: srv}
}

func openStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	st, err := store.Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

type client struct {
	t  *testing.T
	ws *websocket.Conn
}

func (r *rig) dial(t *testing.T, epoch string, pos uint64) (*client, map[string]any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(r.srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.CloseNow() })
	c := &client{t: t, ws: ws}
	c.send(map[string]any{"t": "hello", "epoch": epoch, "pos": pos})
	return c, c.next()
}

func (c *client) send(v any) {
	c.t.Helper()
	raw, _ := json.Marshal(v)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, raw); err != nil {
		c.t.Fatal(err)
	}
}

func (c *client) next() map[string]any {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := c.ws.Read(ctx)
	if err != nil {
		c.t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		c.t.Fatal(err)
	}
	return m
}

func TestCommandSurvivesDroppedSocketAndRunsOnce(t *testing.T) {
	cmds := newFakeCommands()
	r := newRig(t, openStore(t, t.TempDir()), cmds)

	c1, welcome := r.dial(t, "", 0)
	if welcome["t"] != "welcome" || welcome["snapshot"] == nil {
		t.Fatalf("first welcome: %v", welcome)
	}
	epoch, pos := welcome["epoch"].(string), uint64(welcome["pos"].(float64))

	c1.send(map[string]any{"t": "cmd", "id": "cmd-1", "name": "block", "args": map[string]int{"x": 1}})
	<-cmds.started
	_ = c1.ws.CloseNow() // connection lost before the result

	r.hub.Publish("project.added", map[string]string{"id": "p1"})
	r.hub.Publish("project.added", map[string]string{"id": "p2"})
	close(cmds.release)
	r.led.Wait()

	c2, welcome2 := r.dial(t, epoch, pos)
	if welcome2["snapshot"] != nil || welcome2["epoch"] != epoch || uint64(welcome2["pos"].(float64)) != pos {
		t.Fatalf("resume welcome: %v", welcome2)
	}
	for i, want := range []string{"p1", "p2"} {
		ev := c2.next()
		data := ev["data"].(map[string]any)
		if ev["t"] != "event" || data["id"] != want || uint64(ev["pos"].(float64)) != pos+uint64(i)+1 {
			t.Fatalf("missed event %d: %v", i, ev)
		}
	}
	c2.send(map[string]any{"t": "cmd", "id": "cmd-1", "name": "block", "args": map[string]int{"x": 1}})
	res := c2.next()
	if res["t"] != "result" || res["id"] != "cmd-1" || res["ok"] != true || res["value"].(map[string]any)["n"] != float64(1) {
		t.Fatalf("result: %v", res)
	}
	if cmds.runs.Load() != 1 {
		t.Fatalf("command ran %d times", cmds.runs.Load())
	}
}

func TestOutcomeUnknownAfterRestart(t *testing.T) {
	dir := t.TempDir()
	cmds := newFakeCommands()
	r1 := newRig(t, openStore(t, dir), cmds)
	c1, w1 := r1.dial(t, "", 0)
	c1.send(map[string]any{"t": "cmd", "id": "cmd-9", "name": "block"})
	<-cmds.started
	_ = c1.ws.CloseNow()
	// The server dies while the command is recorded as running. A second
	// rig on the same database stands in for the restarted process.
	cmds2 := newFakeCommands()
	r2 := newRig(t, openStore(t, dir), cmds2)
	t.Cleanup(func() { close(cmds.release) })

	c2, w2 := r2.dial(t, w1["epoch"].(string), uint64(w1["pos"].(float64)))
	if w2["snapshot"] == nil || w2["epoch"] == w1["epoch"] {
		t.Fatalf("restart must force a snapshot: %v", w2)
	}
	c2.send(map[string]any{"t": "cmd", "id": "cmd-9", "name": "block"})
	res := c2.next()
	errObj, _ := res["error"].(map[string]any)
	if res["ok"] != false || errObj["code"] != "OUTCOME_UNKNOWN" {
		t.Fatalf("result: %v", res)
	}
	if cmds2.runs.Load() != 0 {
		t.Fatal("command ran again after restart")
	}
}

func TestPendingFrameThenResult(t *testing.T) {
	cmds := newFakeCommands()
	st := openStore(t, t.TempDir())
	hub, _ := NewHub(nil)
	led, _ := ledger.New(context.Background(), st.DB, nil, nil)
	srv := httptest.NewServer(Handler(Config{Hub: hub, Ledger: led, Commands: cmds, PendingAfter: 30 * time.Millisecond,
		Snapshot: func() (any, error) { return map[string]int{}, nil }}))
	defer srv.Close()
	r := &rig{hub: hub, led: led, srv: srv}
	c, _ := r.dial(t, "", 0)
	c.send(map[string]any{"t": "cmd", "id": "slow", "name": "block"})
	if f := c.next(); f["t"] != "pending" || f["id"] != "slow" {
		t.Fatalf("got %v", f)
	}
	close(cmds.release)
	if f := c.next(); f["t"] != "result" || f["ok"] != true {
		t.Fatalf("got %v", f)
	}
}

func TestPingVisibilityAndBadInput(t *testing.T) {
	r := newRig(t, openStore(t, t.TempDir()), newFakeCommands())
	c, _ := r.dial(t, "", 0)
	c.send(map[string]any{"t": "visibility", "hidden": true})
	c.send(map[string]any{"t": "ping"})
	if f := c.next(); f["t"] != "pong" {
		t.Fatalf("got %v", f)
	}
	c.send(map[string]any{"t": "cmd", "id": "bad id!", "name": "count"})
	f := c.next()
	if f["ok"] != false || f["error"].(map[string]any)["code"] != "BAD_REQUEST" {
		t.Fatalf("got %v", f)
	}
}

func TestSilentClientIsDropped(t *testing.T) {
	st := openStore(t, t.TempDir())
	hub, _ := NewHub(nil)
	led, _ := ledger.New(context.Background(), st.DB, nil, nil)
	srv := httptest.NewServer(Handler(Config{Hub: hub, Ledger: led, Commands: newFakeCommands(), Idle: 100 * time.Millisecond,
		Snapshot: func() (any, error) { return 1, nil }}))
	defer srv.Close()
	c, _ := (&rig{hub: hub, led: led, srv: srv}).dial(t, "", 0)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, _, err := c.ws.Read(ctx); ctx.Err() != nil || err == nil {
		t.Fatalf("server should close a silent socket, got %v", err)
	}
}

func TestForeignOriginRefused(t *testing.T) {
	r := newRig(t, openStore(t, t.TempDir()), newFakeCommands())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(r.srv.URL, "http"), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {"https://evil.example"}},
	})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("err=%v resp=%v", err, resp)
	}
}

func TestSlowSubscriberDroppedOthersUnaffected(t *testing.T) {
	hub, _ := NewHub(nil)
	snap := func() (any, error) { return 1, nil }
	slow, _ := hub.Attach("", 0, snap)
	fast, _ := hub.Attach("", 0, snap)
	for i := 0; i < QueueSize+5; i++ {
		hub.Publish("k", i)
		select {
		case <-fast.Sub.Queue:
		default:
			t.Fatal("fast subscriber got nothing")
		}
	}
	select {
	case <-slow.Sub.Dropped:
	default:
		t.Fatal("slow subscriber was not dropped")
	}
	select {
	case <-fast.Sub.Dropped:
		t.Fatal("fast subscriber was dropped")
	default:
	}
}
