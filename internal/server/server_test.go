package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ryokusnadi/mooring/internal/store"
)

const testToken = "correct-token"

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newTestServer(t *testing.T) (http.Handler, *clock) {
	t.Helper()
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	static := fstest.MapFS{
		"index.html":    {Data: []byte("<title>Mooring</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Token: testToken, Static: static, Now: c.now, Store: st, Home: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close(); _ = st.Close() })
	return srv, c
}

type req struct {
	method, path, body, host, origin, site string
	cookie                                 *http.Cookie
}

func do(h http.Handler, q req) *httptest.ResponseRecorder {
	r := httptest.NewRequest(q.method, q.path, strings.NewReader(q.body))
	r.Host = "127.0.0.1:7340"
	if q.host != "" {
		r.Host = q.host
	}
	if q.origin != "" {
		r.Header.Set("Origin", q.origin)
	}
	if q.site != "" {
		r.Header.Set("Sec-Fetch-Site", q.site)
	}
	if q.cookie != nil {
		r.AddCookie(q.cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func login(h http.Handler, token string) *httptest.ResponseRecorder {
	return do(h, req{method: "POST", path: "/api/login", body: `{"token":"` + token + `"}`,
		origin: "http://127.0.0.1:7340"})
}

func sessionCookieOf(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("no session cookie in response")
	return nil
}

func TestLoginOpensSessionAndLogoutClosesIt(t *testing.T) {
	h, _ := newTestServer(t)

	if w := do(h, req{method: "GET", path: "/api/session"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("session before login: got %d, want 401", w.Code)
	}
	w := login(h, testToken)
	if w.Code != http.StatusNoContent {
		t.Fatalf("login: got %d, want 204", w.Code)
	}
	c := sessionCookieOf(t, w)
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie must be HttpOnly and SameSite=Strict, got %+v", c)
	}
	if w := do(h, req{method: "GET", path: "/api/session", cookie: c}); w.Code != http.StatusNoContent {
		t.Fatalf("session after login: got %d, want 204", w.Code)
	}
	if w := do(h, req{method: "POST", path: "/api/logout", cookie: c}); w.Code != http.StatusNoContent {
		t.Fatalf("logout: got %d, want 204", w.Code)
	}
	// Cookies are signed, not stored: logout clears this browser's cookie.
	w = do(h, req{method: "POST", path: "/api/logout", cookie: c})
	if cleared := sessionCookieOf(t, w); cleared.MaxAge >= 0 || cleared.Value != "" {
		t.Fatalf("logout must clear the cookie, got %+v", cleared)
	}
}

func TestSessionExpires(t *testing.T) {
	h, c := newTestServer(t)
	cookie := sessionCookieOf(t, login(h, testToken))
	c.t = c.t.Add(sessionTTL)
	if w := do(h, req{method: "GET", path: "/api/session", cookie: cookie}); w.Code != http.StatusUnauthorized {
		t.Fatalf("expired session: got %d, want 401", w.Code)
	}
}

func TestWrongTokenIsRejectedThenRateLimited(t *testing.T) {
	h, c := newTestServer(t)
	for i := range maxFailures {
		if w := login(h, "wrong"); w.Code != http.StatusUnauthorized {
			t.Fatalf("wrong token attempt %d: got %d, want 401", i+1, w.Code)
		}
	}
	w := login(h, testToken)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("after %d failures: got %d, want 429", maxFailures, w.Code)
	}
	if w.Header().Get("Retry-After") != "60" {
		t.Fatalf("Retry-After: got %q, want 60", w.Header().Get("Retry-After"))
	}
	c.t = c.t.Add(failureWindow + time.Second)
	if w := login(h, testToken); w.Code != http.StatusNoContent {
		t.Fatalf("after the window: got %d, want 204", w.Code)
	}
}

func TestGuardRejectsForeignHostAndOrigin(t *testing.T) {
	h, _ := newTestServer(t)
	cases := []struct {
		name string
		q    req
		want int
	}{
		{"rebinding host", req{method: "GET", path: "/", host: "evil.example"}, 403},
		{"lan address", req{method: "GET", path: "/", host: "192.168.1.5:7340"}, 403},
		{"localhost", req{method: "GET", path: "/", host: "localhost:7340"}, 200},
		{"ipv6 loopback", req{method: "GET", path: "/", host: "[::1]:7340"}, 200},
		{"cross-site write", req{method: "POST", path: "/api/logout", origin: "https://evil.example"}, 403},
		{"cross-site read", req{method: "GET", path: "/api/session", origin: "https://evil.example"}, 403},
		{"null origin", req{method: "POST", path: "/api/logout", origin: "null"}, 403},
		{"other port", req{method: "POST", path: "/api/logout", origin: "http://127.0.0.1:9999"}, 403},
		{"same origin", req{method: "POST", path: "/api/logout", origin: "http://127.0.0.1:7340"}, 204},
		{"no origin", req{method: "POST", path: "/api/logout"}, 204},
	}
	for _, tc := range cases {
		if w := do(h, tc.q); w.Code != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, w.Code, tc.want)
		}
	}
}

func TestStaticServesAppShellAndFallsBack(t *testing.T) {
	h, _ := newTestServer(t)
	cases := []struct {
		path string
		want int
		body string
	}{
		{"/", 200, "Mooring"},
		{"/projects/demo", 200, "Mooring"},
		{"/assets/app.js", 200, "console.log"},
		{"/assets/missing.js", 404, ""},
		{"/favicon.ico", 204, ""},
		{"/api/unknown", 404, "not_found"},
	}
	for _, tc := range cases {
		w := do(h, req{method: "GET", path: tc.path})
		if w.Code != tc.want || !strings.Contains(w.Body.String(), tc.body) {
			t.Errorf("%s: got %d %q, want %d containing %q", tc.path, w.Code, w.Body.String(), tc.want, tc.body)
		}
	}
	if w := do(h, req{method: "GET", path: "/assets/app.js"}); !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("hashed assets must be cached as immutable")
	}
}

func TestStaticWithoutBuildSaysHowToBuild(t *testing.T) {
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	h, err := New(Config{Token: testToken, Static: fstest.MapFS{}, Store: st, Home: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if w := do(h, req{method: "GET", path: "/"}); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", w.Code)
	}
}

func TestLoadOrCreateTokenIsStableAndPrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	first, err := LoadOrCreateToken(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != randomIDLength*2 {
		t.Fatalf("token length: got %d, want %d", len(first), randomIDLength*2)
	}
	second, err := LoadOrCreateToken(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("token changed between starts")
	}
	info, err := os.Stat(filepath.Join(dir, tokenFileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("token file mode: got %o, want 600", perm)
	}
}
