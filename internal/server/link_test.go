package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// link opens /login?code=... the way `mooring open` does: a navigation with no Origin.
func link(h http.Handler, code, site string) *httptest.ResponseRecorder {
	return do(h, req{method: "GET", path: "/login?code=" + url.QueryEscape(code), site: site})
}

// started returns a server and a clock one second after its start, so codes made now are newer.
func started(t *testing.T) (http.Handler, *clock) {
	h, c := newTestServer(t)
	c.t = c.t.Add(time.Second)
	return h, c
}

func newCode(t *testing.T, token string, at time.Time) string {
	t.Helper()
	code, err := NewLinkCode(token, at)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestCodeWorksOnceThenIsRefused(t *testing.T) {
	h, c := started(t)
	code := newCode(t, testToken, c.t)
	w := link(h, code, "none")
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("first use: %d to %q", w.Code, w.Header().Get("Location"))
	}
	cookie := sessionCookieOf(t, w)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie flags: %+v", cookie)
	}
	if got := status(h, cookie); got != http.StatusNoContent {
		t.Fatalf("session from link: got %d", got)
	}
	w = link(h, code, "none")
	if w.Header().Get("Location") != "/?link=used" || len(w.Result().Cookies()) != 0 {
		t.Fatalf("second use: %d to %q with cookies %v", w.Code, w.Header().Get("Location"), w.Result().Cookies())
	}
}

func TestCodeExpiresAfterFiveMinutes(t *testing.T) {
	h, c := started(t)
	code := newCode(t, testToken, c.t)
	c.t = c.t.Add(linkTTL)
	if w := link(h, code, ""); w.Header().Get("Location") != "/?link=used" {
		t.Fatalf("expired code: redirected to %q", w.Header().Get("Location"))
	}
	fresh := newCode(t, testToken, c.t)
	c.t = c.t.Add(linkTTL - time.Second)
	if w := link(h, fresh, ""); w.Header().Get("Location") != "/" {
		t.Fatalf("code just inside 5 minutes: redirected to %q", w.Header().Get("Location"))
	}
}

func TestCodeMadeBeforeServerStartIsRefused(t *testing.T) {
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	old := newCode(t, testToken, c.t.Add(-time.Minute))
	h := startServer(t, testToken, "", c, nil)
	if w := link(h, old, ""); w.Header().Get("Location") != "/?link=used" {
		t.Fatalf("code from before start: redirected to %q", w.Header().Get("Location"))
	}
}

func TestBadCodesCountTowardTheLimit(t *testing.T) {
	h, c := started(t)
	wrongKey := newCode(t, "another-token", c.t)
	for i, code := range []string{wrongKey, "nonsense", "v1.AAAA.BBBB", "v1..", strings.Repeat("x", 5000)} {
		if w := link(h, code, ""); w.Header().Get("Location") != "/?link=invalid" {
			t.Fatalf("bad code %d: %d to %q", i, w.Code, w.Header().Get("Location"))
		}
	}
	good := newCode(t, testToken, c.t)
	if w := link(h, good, ""); w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/?link=wait" {
		t.Fatalf("after %d bad codes: got %d", maxFailures, w.Code)
	}
	if w := login(h, testToken); w.Code != http.StatusTooManyRequests {
		t.Fatalf("token login shares the limit: got %d", w.Code)
	}
	c.t = c.t.Add(failureWindow + time.Second)
	if w := link(h, newCode(t, testToken, c.t), ""); w.Header().Get("Location") != "/" {
		t.Fatalf("after the window: redirected to %q", w.Header().Get("Location"))
	}
}

func TestLinkFromAnotherSiteIsRefused(t *testing.T) {
	h, c := started(t)
	code := newCode(t, testToken, c.t)
	if w := link(h, code, "cross-site"); w.Code != http.StatusForbidden || len(w.Result().Cookies()) != 0 {
		t.Fatalf("cross-site: got %d", w.Code)
	}
	w := do(h, req{method: "GET", path: "/login?code=" + code, origin: "http://evil.example"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("foreign Origin: got %d", w.Code)
	}
	// Refused requests must not burn the code.
	if w := link(h, code, "none"); w.Header().Get("Location") != "/" {
		t.Fatalf("code after refused attempts: redirected to %q", w.Header().Get("Location"))
	}
	if w := do(h, req{method: "GET", path: "/login?code=x", host: "evil.example"}); w.Code != http.StatusForbidden {
		t.Fatalf("foreign Host: got %d", w.Code)
	}
}

func TestUsedCodeMemoryIsBounded(t *testing.T) {
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	a := newAuth("t", "", c.now)
	c.t = c.t.Add(time.Second)
	for range maxUsedCode {
		if got := a.redeem(newCode(t, "t", c.t)); got != linkOK {
			t.Fatalf("redeem: got %v", got)
		}
	}
	if got := a.redeem(newCode(t, "t", c.t)); got != linkBusy {
		t.Fatalf("full table must refuse new codes, got %v", got)
	}
	if len(a.used) != maxUsedCode {
		t.Fatalf("used codes: %d", len(a.used))
	}
	c.t = c.t.Add(linkTTL + time.Second)
	if got := a.redeem(newCode(t, "t", c.t)); got != linkOK || len(a.used) != 1 {
		t.Fatalf("after expiry: result %v, %d remembered", got, len(a.used))
	}
}

func TestHelloProvesTheServerHoldsTheToken(t *testing.T) {
	h, _ := started(t)
	w := do(h, req{method: "GET", path: "/api/hello?nonce=abc123"})
	if w.Code != http.StatusOK || w.Body.String() != HelloProof(testToken, "abc123", "") {
		t.Fatalf("hello: %d %q", w.Code, w.Body.String())
	}
	if w.Body.String() == HelloProof("another-token", "abc123", "") || w.Body.String() == HelloProof(testToken, "abc124", "") {
		t.Fatal("proof must depend on the token and the nonce")
	}
	if HelloProof(testToken, "abc123", "127.0.0.1:1") == HelloProof(testToken, "abc123", "127.0.0.1:2") {
		t.Fatal("proof must depend on the listen address")
	}
	if w := do(h, req{method: "GET", path: "/api/hello"}); w.Code != http.StatusBadRequest {
		t.Fatalf("no nonce: %d", w.Code)
	}
}
