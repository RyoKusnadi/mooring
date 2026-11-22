package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// A session cookie is "v1.<payload>.<signature>". The payload is the expiry
// (8 bytes, Unix seconds) and 16 random bytes; the signature is HMAC-SHA256
// over "v1.<payload>" with a key derived from the login token. The server
// stores nothing, so a restart keeps every browser signed in.
const (
	sessionLabel = "mooring session v1"
	linkLabel    = "mooring link v1"
	helloLabel   = "mooring hello v1"
	cookieV1     = "v1"
	renewBelow   = 15 * 24 * time.Hour
	payloadSize  = 8 + 16
)

var b64 = base64.RawURLEncoding

// deriveKey is HMAC(token, label). Each purpose has its own label, so a value
// signed for one purpose never verifies for the other.
func deriveKey(token, label string) []byte {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte(label))
	return m.Sum(nil)
}

// seal returns "v1.<payload>.<signature>" for a payload made of a time and random bytes.
func seal(key []byte, at time.Time, unit func(time.Time) int64) (string, error) {
	payload := make([]byte, payloadSize)
	binary.BigEndian.PutUint64(payload, uint64(unit(at)))
	if _, err := rand.Read(payload[8:]); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	body := cookieV1 + "." + b64.EncodeToString(payload)
	return body + "." + b64.EncodeToString(mac(key, body)), nil
}

func mac(key []byte, body string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(body))
	return m.Sum(nil)
}

// unseal checks the format and signature in constant time and returns the
// payload and the time inside it. It does not look at the clock.
func unseal(key []byte, value string) (payload string, stamp int64, ok bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != cookieV1 {
		return "", 0, false
	}
	raw, err := b64.DecodeString(parts[1])
	if err != nil || len(raw) != payloadSize {
		return "", 0, false
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac(key, parts[0]+"."+parts[1])) {
		return "", 0, false
	}
	return parts[1], int64(binary.BigEndian.Uint64(raw)), true
}

func unixSeconds(t time.Time) int64 { return t.Unix() }

// newSession returns a signed cookie value that is good for sessionTTL.
func (a *auth) newSession() (string, error) {
	a.refresh()
	a.mu.Lock()
	k := a.keys
	a.mu.Unlock()
	return seal(k.session, a.now().Add(sessionTTL), unixSeconds)
}

// check reports whether value is a valid, unexpired session cookie, and
// whether it is old enough that the server should send a fresh one.
func (a *auth) check(value string) (ok, renew bool) {
	if value == "" {
		return false, false
	}
	a.refresh()
	a.mu.Lock()
	k := a.keys
	a.mu.Unlock()
	_, stamp, valid := unseal(k.session, value)
	if !valid {
		return false, false
	}
	left := time.Unix(stamp, 0).Sub(a.now())
	if left <= 0 {
		return false, false
	}
	return true, left < renewBelow
}

func (s *server) setSessionCookie(w http.ResponseWriter, r *http.Request, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

// authorize reports whether the request carries a valid session. Near expiry
// it also sends a fresh cookie, so regular use never needs a new sign-in.
func (s *server) authorize(w http.ResponseWriter, r *http.Request) bool {
	ok, renew := s.auth.check(s.sessionID(r))
	if ok && renew {
		if value, err := s.auth.newSession(); err == nil {
			s.setSessionCookie(w, r, value)
		} else {
			s.log.Error("session: renew cookie", "err", err)
		}
	}
	return ok
}
