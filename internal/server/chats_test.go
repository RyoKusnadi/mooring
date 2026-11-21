package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestSnapshotListsChatsAndAgents(t *testing.T) {
	ts, url := liveServer(t)
	hdr := sessionCookieHeader(t, ts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.CloseNow() }()
	if err := ws.Write(ctx, websocket.MessageText, []byte(`{"t":"hello","epoch":"","pos":0}`)); err != nil {
		t.Fatal(err)
	}
	_, data, err := ws.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var w struct {
		Snapshot struct {
			Chats  map[string][]json.RawMessage `json:"chats"`
			Agents []struct {
				ID     string `json:"id"`
				Tested bool   `json:"tested"`
			} `json:"agents"`
		} `json:"snapshot"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		t.Fatal(err)
	}
	if w.Snapshot.Chats == nil || len(w.Snapshot.Agents) != 5 || w.Snapshot.Agents[0].ID != "claude" || !w.Snapshot.Agents[0].Tested {
		t.Fatalf("snapshot: %s", data)
	}
}

func TestChatOutputRouteNeedsSessionHostAndOrigin(t *testing.T) {
	h, _ := newTestServer(t)
	path := "/api/chats/c1/outputs/o1"
	if w := do(h, req{method: "GET", path: path}); w.Code != 401 {
		t.Fatalf("no session: %d", w.Code)
	}
	cookie := sessionCookieOf(t, login(h, testToken))
	if w := do(h, req{method: "GET", path: path, cookie: cookie}); w.Code != 404 {
		t.Fatalf("unknown chat: %d", w.Code)
	}
	if w := do(h, req{method: "GET", path: path, cookie: cookie, host: "evil.example"}); w.Code != 403 {
		t.Fatalf("foreign host: %d", w.Code)
	}
	if w := do(h, req{method: "GET", path: path, cookie: cookie, origin: "http://evil.example"}); w.Code != 403 {
		t.Fatalf("foreign origin: %d", w.Code)
	}
}

func TestSettingsRouteNeedsSessionAndGivesDefaults(t *testing.T) {
	h, _ := newTestServer(t)
	if w := do(h, req{method: "GET", path: "/api/settings"}); w.Code != 401 {
		t.Fatalf("no session: %d", w.Code)
	}
	cookie := sessionCookieOf(t, login(h, testToken))
	w := do(h, req{method: "GET", path: "/api/settings", cookie: cookie})
	want := `{"compactDefault":false,"smallModel":{},"titleJobs":false,"commitJobs":false,"jobTokensPerDay":200000}`
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != want {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := do(h, req{method: "GET", path: "/api/settings", cookie: cookie, origin: "http://evil.example"}); w.Code != 403 {
		t.Fatalf("foreign origin: %d", w.Code)
	}
}
