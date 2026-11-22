package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

const longChat = 2000

// seedLongChat saves a chat of longChat messages: tool calls with output, and replies.
func seedLongChat(t *testing.T, r *rig) *Chat {
	t.Helper()
	c, err := newChat(r.m, "big", "p1", r.dir, r.dir, Spec{Agent: AgentSlot{ID: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	c.title = "big"
	if err := r.m.insertRow(c, c.Info()); err != nil {
		t.Fatal(err)
	}
	r.m.mu.Lock()
	r.m.chats["big"] = c
	r.m.mu.Unlock()
	c.mu.Lock()
	for i := 0; i < longChat; i++ {
		switch i % 4 {
		case 0:
			c.addLocked(Message{Kind: KindUser, Text: fmt.Sprintf("question %d", i)})
		case 1:
			c.addLocked(Message{Kind: KindTool, ToolID: fmt.Sprint("t", i), Title: "run tests", ToolKind: "execute", Status: "completed",
				Content: json.RawMessage(`[{"type":"content","content":{"type":"text","text":"` + strings.Repeat("ok ", 100) + `"}}]`)})
		default:
			c.addLocked(Message{Kind: KindAgent, Text: strings.Repeat("answer ", 40)})
		}
	}
	c.mu.Unlock()
	c.flush()
	return c
}

func TestLongChatResumeAndResetAreFast(t *testing.T) {
	r := newRig(t)
	c := seedLongChat(t, r)
	head, err := c.attach("", 0)
	if err != nil {
		t.Fatal(err)
	}
	c.hub.Detach(head.Sub)
	epoch, top := head.Epoch, head.Pos

	start := time.Now()
	a, err := c.attach(epoch, top-10)
	if err != nil || a.Snapshot != nil || len(a.Replay) != 10 {
		t.Fatalf("resume near the head: snapshot=%v replay=%d err=%v", a.Snapshot != nil, len(a.Replay), err)
	}
	if d := time.Since(start); d > 20*time.Millisecond {
		t.Fatalf("resume took %v", d)
	}
	start = time.Now()
	a, err = c.attach("other-epoch", 0)
	if err != nil {
		t.Fatal(err)
	}
	body := a.Snapshot.(snapshotBody)
	raw, _ := json.Marshal(body)
	if len(body.Messages) != longChat || a.Pos != top {
		t.Fatalf("reset: %d messages at pos %d", len(body.Messages), a.Pos)
	}
	if d := time.Since(start); d > 250*time.Millisecond {
		t.Fatalf("reset of %d messages took %v (%d bytes)", longChat, d, len(raw))
	}
	t.Logf("reset snapshot: %d messages, %d KiB", longChat, len(raw)>>10)
}

func TestLongChatSurvivesRestartAndStreamsOverSocket(t *testing.T) {
	r := newRig(t)
	seedLongChat(t, r)
	r.m.Shutdown()
	r.boot()
	if got := r.m.get("big").Info().Messages; got != longChat {
		t.Fatalf("saved count %d", got)
	}
	ts := httptest.NewServer(chatMux(r.m))
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dial := func() *websocket.Conn {
		ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/chats/big", nil)
		if err != nil {
			t.Fatal(err)
		}
		ws.SetReadLimit(8 << 20)
		return ws
	}
	read := func(ws *websocket.Conn) map[string]any {
		_, data, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var f map[string]any
		_ = json.Unmarshal(data, &f)
		return f
	}
	ws := dial()
	start := time.Now()
	_ = ws.Write(ctx, websocket.MessageText, []byte(`{"t":"hello","epoch":"","pos":0}`))
	w := read(ws)
	snap, _ := w["snapshot"].(map[string]any)
	if w["t"] != "welcome" || snap == nil || len(snap["messages"].([]any)) != longChat {
		t.Fatalf("welcome: %v", w["t"])
	}
	t.Logf("first load over the socket: %v", time.Since(start))
	_ = ws.CloseNow()

	// A reload with a position resumes without a snapshot and gets what it missed.
	c := r.m.get("big")
	epoch, pos := w["epoch"].(string), uint64(w["pos"].(float64))
	c.mu.Lock()
	c.addLocked(Message{Kind: KindAgent, Text: "late"})
	c.mu.Unlock()
	ws = dial()
	defer func() { _ = ws.CloseNow() }()
	_ = ws.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf(`{"t":"hello","epoch":%q,"pos":%d}`, epoch, pos)))
	w = read(ws)
	if w["snapshot"] != nil || uint64(w["pos"].(float64)) != pos {
		t.Fatalf("resume welcome: %v", w)
	}
	ev := read(ws)
	if ev["t"] != "event" || ev["kind"] != "set" || uint64(ev["pos"].(float64)) != pos+1 {
		t.Fatalf("event: %v", ev)
	}
}

func TestUnknownChatSocketIs404(t *testing.T) {
	r := newRig(t)
	ts := httptest.NewServer(chatMux(r.m))
	defer ts.Close()
	_, resp, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/chats/none", nil)
	if err == nil || resp == nil || resp.StatusCode != 404 {
		t.Fatalf("err=%v resp=%v", err, resp)
	}
}

func chatMux(m *Manager) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/chats/{id}", m.Handler())
	return mux
}
