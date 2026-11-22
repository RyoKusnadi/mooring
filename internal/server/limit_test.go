package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func loginFrom(h http.Handler, remote, token string) int {
	r := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"token":"`+token+`"}`))
	r.Host = "127.0.0.1:7340"
	r.Header.Set("Origin", "http://127.0.0.1:7340")
	r.RemoteAddr = remote
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

func TestLoginLimitIsPerAddress(t *testing.T) {
	h, _ := newTestServer(t)
	for i := 0; i < maxFailures; i++ {
		if code := loginFrom(h, "198.51.100.7:1000", "wrong"); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i, code)
		}
	}
	if code := loginFrom(h, "198.51.100.7:2000", testToken); code != http.StatusTooManyRequests {
		t.Fatalf("same address after %d failures: got %d, want 429", maxFailures, code)
	}
	// Someone else on the network cannot lock the owner out.
	if code := loginFrom(h, "198.51.100.8:1000", testToken); code != http.StatusNoContent {
		t.Fatalf("another address: got %d, want 204", code)
	}
	if code := loginFrom(h, "127.0.0.1:1000", testToken); code != http.StatusNoContent {
		t.Fatalf("this machine: got %d, want 204", code)
	}
}

func TestClientKey(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:5":             "loopback",
		"[::1]:5":                 "loopback",
		"192.0.2.4:5":             "192.0.2.4",
		"[::ffff:192.0.2.4]:5":    "192.0.2.4",
		"[2001:db8:1:2:3::9]:5":   "2001:db8:1::/48",
		"[2001:db8:1:7:ffff::]:5": "2001:db8:1::/48",
		"[2001:db8:2::1]:5":       "2001:db8:2::/48",
	}
	for remote, want := range cases {
		if got := clientKey(&http.Request{RemoteAddr: remote}); got != want {
			t.Errorf("%s: got %q, want %q", remote, got, want)
		}
	}
}

func TestFullLimitTableStillLetsThisMachineIn(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	l := newLimiter(func() time.Time { return now })
	for i := 0; i < maxLimitKeys; i++ {
		l.fail(fmt.Sprintf("10.%d.%d.1", i/256, i%256))
	}
	if w := l.retryAfter("203.0.113.1"); w == 0 {
		t.Fatal("a new address must wait while the table is full")
	}
	if w := l.retryAfter("loopback"); w != 0 {
		t.Fatalf("this machine must not wait, got %v", w)
	}
	now = now.Add(failureWindow + time.Second)
	if w := l.retryAfter("203.0.113.1"); w != 0 {
		t.Fatalf("after the window the table empties, got %v", w)
	}
}
