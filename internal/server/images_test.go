package server

import "testing"

func TestChatImageRoutesNeedSessionHostAndOrigin(t *testing.T) {
	h, _ := newTestServer(t)
	cookie := sessionCookieOf(t, login(h, testToken))
	for _, rt := range []struct{ method, path string }{
		{"POST", "/api/chats/c1/images?name=a.png"},
		{"GET", "/api/chats/c1/images/i1"},
	} {
		if w := do(h, req{method: rt.method, path: rt.path}); w.Code != 401 {
			t.Fatalf("%s no session: %d", rt.method, w.Code)
		}
		if w := do(h, req{method: rt.method, path: rt.path, cookie: cookie}); w.Code != 404 {
			t.Fatalf("%s unknown chat: %d", rt.method, w.Code)
		}
		if w := do(h, req{method: rt.method, path: rt.path, cookie: cookie, host: "evil.example"}); w.Code != 403 {
			t.Fatalf("%s foreign host: %d", rt.method, w.Code)
		}
		if w := do(h, req{method: rt.method, path: rt.path, cookie: cookie, origin: "http://evil.example"}); w.Code != 403 {
			t.Fatalf("%s foreign origin: %d", rt.method, w.Code)
		}
	}
	// A write with no Origin from another site is refused too.
	if w := do(h, req{method: "POST", path: "/api/chats/c1/images", cookie: cookie, site: "cross-site"}); w.Code != 403 {
		t.Fatalf("cross-site upload: %d", w.Code)
	}
}
