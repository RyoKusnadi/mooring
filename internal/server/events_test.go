package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func liveServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	h, _ := newTestServer(t)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts, "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/events"
}

func sessionCookieHeader(t *testing.T, ts *httptest.Server) http.Header {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/login", "application/json", strings.NewReader(`{"token":"`+testToken+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie {
			return http.Header{"Cookie": {c.String()}}
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func TestSocketWithoutSessionIsRefusedWith401(t *testing.T) {
	_, url := liveServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, url, nil)
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("err=%v resp=%v", err, resp)
	}
}

func TestSocketWithForeignOriginIsRefused(t *testing.T) {
	ts, url := liveServer(t)
	hdr := sessionCookieHeader(t, ts)
	hdr.Set("Origin", "https://evil.example")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: hdr})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("err=%v resp=%v", err, resp)
	}
}

func TestSocketRunsProjectCommandAndPublishesEvent(t *testing.T) {
	ts, url := liveServer(t)
	hdr := sessionCookieHeader(t, ts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.CloseNow() }()
	send := func(v string) {
		if err := ws.Write(ctx, websocket.MessageText, []byte(v)); err != nil {
			t.Fatal(err)
		}
	}
	next := func() map[string]any {
		_, data, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		return m
	}
	send(`{"t":"hello","epoch":"","pos":0}`)
	w := next()
	home := w["snapshot"].(map[string]any)["home"].(string)
	send(`{"t":"cmd","id":"a1","name":"project.create","args":{"parent":"` + home + `","name":"demo"}}`)
	var gotEvent, gotResult bool
	for i := 0; i < 2; i++ {
		switch f := next(); f["t"] {
		case "event":
			gotEvent = f["kind"] == "project.added" && f["pos"] == float64(1)
		case "result":
			gotResult = f["ok"] == true && f["id"] == "a1"
		}
	}
	if !gotEvent || !gotResult {
		t.Fatalf("event=%v result=%v", gotEvent, gotResult)
	}
}

func TestTerminalSocketNeedsSession(t *testing.T) {
	ts, _ := liveServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/terminals/abc"
	_, resp, err := websocket.Dial(ctx, url, nil)
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no session: err=%v resp=%v", err, resp)
	}
	hdr := sessionCookieHeader(t, ts)
	_, resp, err = websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: hdr})
	if err == nil || resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown terminal: err=%v resp=%v", err, resp)
	}
	hdr.Set("Origin", "https://evil.example")
	_, resp, err = websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: hdr})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin: err=%v resp=%v", err, resp)
	}
}
