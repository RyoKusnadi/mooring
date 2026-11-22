package server

import (
	"encoding/hex"
	"net/http"
	"time"
)

// A sign-in code has the same shape as a cookie, signed with the link key,
// and carries its issue time in milliseconds. It works once, for linkTTL, and
// only if it was made after this server started.
const (
	linkTTL     = 5 * time.Minute
	linkSkew    = time.Minute // a code stamped this far ahead of our clock is still accepted
	maxUsedCode = 1024        // bound on remembered codes; a full table refuses new ones
)

// NewLinkCode makes a sign-in code from the login token. The CLI calls it
// without contacting the server; the server verifies with the same key.
func NewLinkCode(token string, now time.Time) (string, error) {
	return seal(deriveKey(token, linkLabel), now, func(t time.Time) int64 { return t.UnixMilli() })
}

type linkResult int

const (
	linkOK    linkResult = iota
	linkBad              // wrong format or wrong key: counts as a failed login
	linkStale            // signed correctly but used, expired, or from before this start
	linkBusy             // the used-code table is full
)

// redeem checks a code and, when it is good, marks it used.
func (a *auth) redeem(code string) linkResult {
	a.refresh()
	a.mu.Lock()
	defer a.mu.Unlock()
	k := a.keys
	payload, stamp, ok := unseal(k.link, code)
	if !ok {
		return linkBad
	}
	now := a.now()
	issued := time.UnixMilli(stamp)
	if issued.Before(a.start) || !now.Before(issued.Add(linkTTL)) || issued.After(now.Add(linkSkew)) {
		return linkStale
	}
	for p, until := range a.used {
		if !now.Before(until) {
			delete(a.used, p)
		}
	}
	if _, seen := a.used[payload]; seen {
		return linkStale
	}
	if len(a.used) >= maxUsedCode {
		return linkBusy
	}
	a.used[payload] = issued.Add(linkTTL)
	return linkOK
}

// handleLink is GET /login?code=...: it turns a sign-in code into a session
// cookie and sends the browser to the app, so the code leaves the address bar.
func (s *server) handleLink(w http.ResponseWriter, r *http.Request) {
	if !originAllowed(r) {
		writeError(w, http.StatusForbidden, "origin_not_allowed")
		return
	}
	// The code is in the URL: keep it out of caches and referrers (Referrer-Policy is set by guard).
	w.Header().Set("Cache-Control", "no-store")
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if wait := s.limit.retryAfter(clientKey(r)); wait > 0 {
		http.Redirect(w, r, "/?link=wait", http.StatusSeeOther)
		return
	}
	switch s.auth.redeem(code) {
	case linkBad:
		s.limit.fail(clientKey(r))
		s.log.Warn("sign-in link: refused", "remote", r.RemoteAddr)
		http.Redirect(w, r, "/?link=invalid", http.StatusSeeOther)
		return
	case linkBusy:
		s.log.Warn("sign-in link: too many codes in use")
		http.Redirect(w, r, "/?link=busy", http.StatusSeeOther)
		return
	case linkStale:
		s.log.Info("sign-in link: used or expired", "remote", r.RemoteAddr)
		http.Redirect(w, r, "/?link=used", http.StatusSeeOther)
		return
	}
	value, err := s.auth.newSession()
	if err != nil {
		s.log.Error("sign-in link: create session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.setSessionCookie(w, r, value)
	s.log.Info("sign-in link: session opened", "remote", r.RemoteAddr)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HelloProof is what a Mooring server answers to GET /api/hello?nonce=...:
// HMAC-SHA256 of the nonce and the address the server listens on, under a key
// only the token holder can derive. Binding the address stops a program on
// another port from relaying the nonce to the real server.
// `mooring open` checks it before it makes a sign-in code, so a stale address
// file cannot send a live code to another program.
func HelloProof(token, nonce, listenAddr string) string {
	return hex.EncodeToString(mac(deriveKey(token, helloLabel), nonce+"\n"+listenAddr))
}

func (s *server) handleHello(w http.ResponseWriter, r *http.Request) {
	nonce := r.URL.Query().Get("nonce")
	if nonce == "" || len(nonce) > 128 {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	s.auth.refresh()
	s.auth.mu.Lock()
	k := s.auth.keys
	s.auth.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(hex.EncodeToString(mac(k.hello, nonce+"\n"+s.listenAddr))))
}
