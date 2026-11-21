package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ryokusnadi/mooring/internal/store"
)

// startServer builds a server for token. dataDir may be "" (no token file to watch).
func startServer(t *testing.T, token, dataDir string, c *clock, logger *slog.Logger) http.Handler {
	t.Helper()
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	static := fstest.MapFS{"index.html": {Data: []byte("<title>Mooring</title>")}}
	srv, err := New(Config{Token: token, Static: static, Now: c.now, Store: st, Home: t.TempDir(), DataDir: dataDir, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close(); _ = st.Close() })
	return srv
}

func status(h http.Handler, c *http.Cookie) int {
	return do(h, req{method: "GET", path: "/api/session", cookie: c}).Code
}

func TestTamperedOrMalformedCookieIsRefused(t *testing.T) {
	h, _ := newTestServer(t)
	good := sessionCookieOf(t, login(h, testToken))
	parts := strings.Split(good.Value, ".")
	flip := func(s string) string {
		b := []byte(s)
		if b[0] == 'A' {
			b[0] = 'B'
		} else {
			b[0] = 'A'
		}
		return string(b)
	}
	for name, value := range map[string]string{
		"payload":   parts[0] + "." + flip(parts[1]) + "." + parts[2],
		"signature": parts[0] + "." + parts[1] + "." + flip(parts[2]),
		"version":   "v2." + parts[1] + "." + parts[2],
		"short":     parts[0] + "." + parts[1],
		"empty sig": parts[0] + "." + parts[1] + ".",
		"old style": strings.Repeat("a", 64),
	} {
		if got := status(h, &http.Cookie{Name: sessionCookie, Value: value}); got != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, got)
		}
	}
	other := startServer(t, "another-token", "", &clock{t: time.Unix(1_800_000_000, 0)}, nil)
	if got := status(other, good); got != http.StatusUnauthorized {
		t.Errorf("cookie signed with another token: got %d, want 401", got)
	}
}

func TestCookieSurvivesServerRestart(t *testing.T) {
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	first := startServer(t, testToken, "", c, nil)
	cookie := sessionCookieOf(t, login(first, testToken))
	second := startServer(t, testToken, "", c, nil)
	if got := status(second, cookie); got != http.StatusNoContent {
		t.Fatalf("after restart: got %d, want 204", got)
	}
}

func TestCookieIsRenewedNearExpiry(t *testing.T) {
	h, c := newTestServer(t)
	cookie := sessionCookieOf(t, login(h, testToken))
	if w := do(h, req{method: "GET", path: "/api/session", cookie: cookie}); len(w.Result().Cookies()) != 0 {
		t.Fatal("a fresh cookie must not be renewed")
	}
	c.t = c.t.Add(sessionTTL - renewBelow + time.Hour)
	w := do(h, req{method: "GET", path: "/api/session", cookie: cookie})
	if w.Code != http.StatusNoContent {
		t.Fatalf("near expiry: got %d, want 204", w.Code)
	}
	renewed := sessionCookieOf(t, w)
	if !renewed.HttpOnly || renewed.SameSite != http.SameSiteStrictMode || renewed.MaxAge != int(sessionTTL.Seconds()) {
		t.Fatalf("renewed cookie flags: %+v", renewed)
	}
	// The old cookie ends at its own expiry; the new one runs a full term from now.
	c.t = c.t.Add(renewBelow)
	if got := status(h, cookie); got != http.StatusUnauthorized {
		t.Fatalf("old cookie after its expiry: got %d, want 401", got)
	}
	if got := status(h, renewed); got != http.StatusNoContent {
		t.Fatalf("renewed cookie: got %d, want 204", got)
	}
}

func TestTokenNewSignsEveryoneOut(t *testing.T) {
	dir := t.TempDir()
	token, err := LoadOrCreateToken(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	h := startServer(t, token, dir, c, nil)
	cookie := sessionCookieOf(t, login(h, token))
	if got := status(h, cookie); got != http.StatusNoContent {
		t.Fatalf("before token -new: got %d", got)
	}
	fresh, err := ReplaceToken(dir)
	if err != nil || fresh == token {
		t.Fatalf("ReplaceToken: %q, %v", fresh, err)
	}
	c.t = c.t.Add(2 * time.Second)
	if got := status(h, cookie); got != http.StatusUnauthorized {
		t.Fatalf("old cookie after token -new: got %d, want 401", got)
	}
	if w := login(h, token); w.Code != http.StatusUnauthorized {
		t.Fatalf("old token after token -new: got %d, want 401", w.Code)
	}
	if w := login(h, fresh); w.Code != http.StatusNoContent {
		t.Fatalf("new token: got %d, want 204", w.Code)
	}
	// A restarted server also refuses the old cookie.
	if got := status(startServer(t, fresh, dir, c, nil), cookie); got != http.StatusUnauthorized {
		t.Fatalf("old cookie after restart: got %d, want 401", got)
	}
}

func TestSessionAndLinkKeysDiffer(t *testing.T) {
	if string(deriveKey("t", sessionLabel)) == string(deriveKey("t", linkLabel)) {
		t.Fatal("keys must differ per purpose")
	}
	// A sign-in code must not work as a cookie, nor the other way round.
	h, c := newTestServer(t)
	code, err := NewLinkCode(testToken, c.t)
	if err != nil {
		t.Fatal(err)
	}
	if got := status(h, &http.Cookie{Name: sessionCookie, Value: code}); got != http.StatusUnauthorized {
		t.Fatalf("code as cookie: got %d, want 401", got)
	}
	cookie := sessionCookieOf(t, login(h, testToken))
	c.t = c.t.Add(time.Second)
	if w := link(h, cookie.Value, ""); w.Header().Get("Location") != "/?link=invalid" {
		t.Fatalf("cookie as code: redirected to %q", w.Header().Get("Location"))
	}
}

func TestCodeNeverReachesTheLog(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	h := startServer(t, testToken, "", c, logger)
	c.t = c.t.Add(time.Second)
	good, _ := NewLinkCode(testToken, c.t)
	bad, _ := NewLinkCode("other", c.t)
	for _, code := range []string{good, good, bad} {
		link(h, code, "")
	}
	login(h, "wrong-token-value")
	if buf.Len() == 0 {
		t.Fatal("expected some log lines")
	}
	for _, secret := range []string{good, bad, strings.Split(good, ".")[1], "wrong-token-value", testToken} {
		if strings.Contains(buf.String(), secret) {
			t.Fatalf("log contains a secret: %s", buf.String())
		}
	}
}

func TestTokenReplacedWithSameSizeAndTimeIsStillNoticed(t *testing.T) {
	dir := t.TempDir()
	token, _ := LoadOrCreateToken(dir)
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	h := startServer(t, token, dir, c, nil)
	cookie := sessionCookieOf(t, login(h, token))
	c.t = c.t.Add(2 * time.Second)
	status(h, cookie) // reads the file once and remembers it
	path := filepath.Join(dir, tokenFileName)
	old, _ := os.Stat(path)
	if _, err := ReplaceToken(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, old.ModTime(), old.ModTime()); err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(2 * time.Second)
	if got := status(h, cookie); got != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", got)
	}
}
