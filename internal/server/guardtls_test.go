package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGuardAllowsCertificateNames(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := guard([]string{"box.example.ts.net", "100.64.0.5", "fd7a::5", "*.lan.example"}, ok)
	cases := []struct {
		host string
		want int
	}{
		{"box.example.ts.net:7340", 204},
		{"BOX.example.ts.net.:7340", 204},
		{"100.64.0.5:7340", 204},
		{"[fd7a::5]:7340", 204},
		{"127.0.0.1:7340", 204},
		{"other.example.ts.net:7340", 403},
		{"100.64.0.6:7340", 403},
		{"pc.lan.example:7340", 204},
		{"a.pc.lan.example:7340", 403},
		{"lan.example:7340", 403},
	}
	for _, tc := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.Host = tc.host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s: got %d, want %d", tc.host, w.Code, tc.want)
		}
	}
}

func TestHSTSOnlyOverTLSToAName(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := guard([]string{"box.example"}, ok)
	cases := []struct {
		host string
		tls  bool
		want bool
	}{
		{"box.example:7340", true, true},
		{"box.example:7340", false, false},
		// HSTS on localhost would force https on every local port.
		{"localhost:7340", true, false},
	}
	for _, tc := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.Host = tc.host
		if tc.tls {
			r.TLS = &tls.ConnectionState{}
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if got := w.Header().Get("Strict-Transport-Security") != ""; got != tc.want {
			t.Errorf("%s tls=%v: HSTS %v, want %v", tc.host, tc.tls, got, tc.want)
		}
	}
}
