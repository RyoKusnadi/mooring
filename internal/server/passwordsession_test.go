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

	"github.com/ryokusnadi/mooring/internal/password"
)

func TestSignOutOthersKeepsThisBrowserOnly(t *testing.T) {
	h, _ := newPasswordServer(t, true)
	mine := sessionCookieOf(t, passLogin(h, testUser, testPass))
	other := sessionCookieOf(t, login(h, testToken))
	post := func(c *http.Cookie, body string) *httptest.ResponseRecorder {
		return do(h, req{method: "POST", path: "/api/password/sign-out-others", body: body, origin: "http://127.0.0.1:7340", cookie: c})
	}
	if w := post(nil, `{"current":"`+testPass+`"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("no session: %d", w.Code)
	}
	if w := post(mine, `{"current":"wrong password"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", w.Code)
	}
	if w := do(h, req{method: "POST", path: "/api/password/sign-out-others", body: `{"current":"` + testPass + `"}`, site: "cross-site", cookie: mine}); w.Code != http.StatusForbidden {
		t.Fatalf("cross-site: %d", w.Code)
	}
	w := post(mine, `{"current":"`+testPass+`"}`)
	if w.Code != http.StatusNoContent {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
	fresh := sessionCookieOf(t, w)
	if do(h, req{method: "GET", path: "/api/session", cookie: fresh}).Code != http.StatusNoContent {
		t.Fatal("this browser was signed out")
	}
	for name, c := range map[string]*http.Cookie{"old cookie": mine, "token cookie": other} {
		if do(h, req{method: "GET", path: "/api/session", cookie: c}).Code != http.StatusUnauthorized {
			t.Fatalf("%s still works", name)
		}
	}
	if w := login(h, testToken); w.Code != http.StatusUnauthorized {
		t.Fatal("the old token still works")
	}
}

func TestPasswordStatusNeedsASession(t *testing.T) {
	h, _ := newPasswordServer(t, true)
	if w := do(h, req{method: "GET", path: "/api/password"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("no session: %d", w.Code)
	}
	c := sessionCookieOf(t, login(h, testToken))
	w := do(h, req{method: "GET", path: "/api/password", cookie: c})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"username":"ada"`) || strings.Contains(w.Body.String(), testPass) {
		t.Fatalf("with session: %d %s", w.Code, w.Body)
	}
}

// Settings commands go over the events socket, which needs a session; the
// run-once ledger keeps neither the arguments nor the answer of a secret command.
func TestSetChangeRemoveOverTheSocket(t *testing.T) {
	h, dir := newPasswordServer(t, false)
	ts := httptest.NewServer(h)
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/events"
	if _, resp, err := websocket.Dial(ctx, wsURL, nil); err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no session: %v", err)
	}
	hdr := sessionCookieHeader(t, ts)
	ws, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.CloseNow() }()
	read := func() map[string]any {
		_, data, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		return m
	}
	if err := ws.Write(ctx, websocket.MessageText, []byte(`{"t":"hello","epoch":"","pos":0}`)); err != nil {
		t.Fatal(err)
	}
	read()
	cmd := func(id, name, args string) map[string]any {
		if err := ws.Write(ctx, websocket.MessageText, []byte(`{"t":"cmd","id":"`+id+`","name":"`+name+`","args":`+args+`}`)); err != nil {
			t.Fatal(err)
		}
		for {
			if m := read(); m["t"] == "result" && m["id"] == id {
				return m
			}
		}
	}
	if r := cmd("p1", "password.set", `{"username":"ada","password":"first password"}`); r["ok"] != true {
		t.Fatalf("set: %v", r)
	}
	if w := passLogin(h, "ada", "first password"); w.Code != http.StatusNoContent {
		t.Fatalf("login with the new password: %d", w.Code)
	}
	r := cmd("p2", "password.set", `{"username":"ada","password":"second password"}`)
	if r["ok"] != false || r["error"].(map[string]any)["code"] != "WRONG_PASSWORD" {
		t.Fatalf("change without current: %v", r)
	}
	if r := cmd("p3", "password.set", `{"current":"first password","username":"bob","password":"second password"}`); r["ok"] != true {
		t.Fatalf("change: %v", r)
	}
	if w := passLogin(h, "bob", "second password"); w.Code != http.StatusNoContent {
		t.Fatalf("login after change: %d", w.Code)
	}
	if r := cmd("p4", "password.remove", `{"current":"second password"}`); r["ok"] != true {
		t.Fatalf("remove: %v", r)
	}
	if rec, _ := password.Load(dir); rec != nil {
		t.Fatal("file still there")
	}
}

func TestRotatingTheKeysClosesOpenSockets(t *testing.T) {
	h, _ := newPasswordServer(t, true)
	ts := httptest.NewServer(h)
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/events"
	cookieFor := func(body string) *http.Cookie {
		resp, err := http.Post(ts.URL+"/api/login", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		for _, c := range resp.Cookies() {
			if c.Name == sessionCookie {
				return c
			}
		}
		t.Fatal("no cookie")
		return nil
	}
	dial := func(c *http.Cookie) (*websocket.Conn, error) {
		ws, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Cookie": {c.String()}}})
		return ws, err
	}
	mine := cookieFor(`{"username":"ada","password":"` + testPass + `"}`)
	other := cookieFor(`{"token":"` + testToken + `"}`)
	wsMine, err := dial(mine)
	if err != nil {
		t.Fatal(err)
	}
	wsOther, err := dial(other)
	if err != nil {
		t.Fatal(err)
	}

	r, _ := http.NewRequestWithContext(ctx, "POST", ts.URL+"/api/password/sign-out-others", strings.NewReader(`{"current":"`+testPass+`"}`))
	r.AddCookie(mine)
	resp, err := http.DefaultClient.Do(r)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("rotate: %v %v", err, resp)
	}
	_ = resp.Body.Close()
	var fresh *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie {
			fresh = c
		}
	}
	if fresh == nil {
		t.Fatal("no fresh cookie")
	}

	for name, ws := range map[string]*websocket.Conn{"rotating browser": wsMine, "other browser": wsOther} {
		for {
			if _, _, err := ws.Read(ctx); err != nil {
				if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
					t.Fatalf("%s: closed with %v, want a policy close", name, err)
				}
				break
			}
		}
	}
	ws, err := dial(fresh)
	if err != nil {
		t.Fatalf("reconnect with the fresh cookie: %v", err)
	}
	_ = ws.CloseNow()
	if _, err := dial(other); err == nil {
		t.Fatal("the other browser reconnected with its old cookie")
	}
}
