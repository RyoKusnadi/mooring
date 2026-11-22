package server

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// IsLoopbackHost reports whether host (with or without a port) names this
// machine only: "localhost" or a loopback IP address.
func IsLoopbackHost(host string) bool {
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	name = strings.TrimSuffix(strings.Trim(name, "[]"), ".")
	if strings.EqualFold(name, "localhost") {
		return true
	}
	ip := net.ParseIP(name)
	return ip != nil && ip.IsLoopback()
}

// originAllowed checks a request that may come from another website.
// A browser sends Origin on every cross-site write, so a present Origin must
// name this server. A missing one comes from a non-browser client or a plain
// same-site navigation, and the session cookie is SameSite=Strict.
func originAllowed(r *http.Request) bool {
	origins := r.Header.Values("Origin")
	if len(origins) > 1 {
		return false
	}
	if len(origins) == 1 {
		u, err := url.Parse(origins[0])
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return false
		}
		return strings.EqualFold(u.Host, r.Host)
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
		return true
	}
	return false
}

func isWrite(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// guard rejects requests addressed to an unexpected host name (DNS rebinding)
// and writes sent from another website (cross-site request forgery).
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsLoopbackHost(r.Host) {
			writeError(w, http.StatusForbidden, "host_not_allowed")
			return
		}
		if (isWrite(r.Method) || r.Header.Get("Origin") != "") && !originAllowed(r) {
			writeError(w, http.StatusForbidden, "origin_not_allowed")
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")
		next.ServeHTTP(w, r)
	})
}
