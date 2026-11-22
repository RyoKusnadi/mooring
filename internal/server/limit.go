package server

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// maxLimitKeys caps how many addresses the login limit remembers. When it is
// full of addresses with recent failures, a new address waits too.
const maxLimitKeys = 4096

// limiter counts failed logins per client address, so someone else on the
// network cannot lock the owner out by failing on purpose.
type limiter struct {
	now func() time.Time

	mu    sync.Mutex
	byKey map[string][]time.Time
}

func newLimiter(now func() time.Time) *limiter {
	return &limiter{now: now, byKey: map[string][]time.Time{}}
}

// clientKey names the client a request came from: its IP, an IPv6 address by
// its /48 (one network or provider customer usually holds the whole block, so
// it cannot fill the table alone), and every loopback
// address as one. Forwarding headers are not trusted.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	switch {
	case ip == nil:
		return host
	case ip.IsLoopback():
		return "loopback"
	case ip.To4() != nil:
		return ip.To4().String()
	}
	return ip.Mask(net.CIDRMask(48, 128)).String() + "/48"
}

// recentLocked drops the failures of key older than the window and returns the rest.
func (l *limiter) recentLocked(key string) []time.Time {
	cutoff := l.now().Add(-failureWindow)
	old := l.byKey[key]
	kept := old[:0]
	for _, t := range old {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.byKey, key)
		return nil
	}
	l.byKey[key] = kept
	return kept
}

// waitLocked is how long key must wait before its next attempt; zero means now.
func (l *limiter) waitLocked(key string) time.Duration {
	if f := l.recentLocked(key); len(f) >= maxFailures {
		return f[0].Add(failureWindow).Sub(l.now())
	}
	if l.fullFor(key) {
		for k := range l.byKey {
			l.recentLocked(k)
		}
		if len(l.byKey) >= maxLimitKeys {
			return failureWindow
		}
	}
	return 0
}

// retryAfter reports how long logins from key stay blocked.
func (l *limiter) retryAfter(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.waitLocked(key)
}

// reserve counts an attempt before it is checked, so requests that arrive at
// the same time cannot all slip under the limit. When key is blocked it counts
// nothing and returns the wait. release takes the attempt back after a success.
func (l *limiter) reserve(key string) (release func(), wait time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if w := l.waitLocked(key); w > 0 {
		return nil, w
	}
	at := l.now()
	l.byKey[key] = append(l.byKey[key], at)
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		f := l.byKey[key]
		for i := len(f) - 1; i >= 0; i-- {
			if f[i].Equal(at) {
				l.byKey[key] = append(f[:i], f[i+1:]...)
				return
			}
		}
	}, 0
}

// fail counts a failed attempt from key.
func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fullFor(key) {
		return // the table is full; a new address waits anyway
	}
	l.byKey[key] = append(l.byKey[key], l.now())
}

// fullFor reports whether key would need a new row in a full table. This
// machine and signed-in browsers always get one, so strangers filling the
// table cannot lock the owner out.
func (l *limiter) fullFor(key string) bool {
	if key == "loopback" || key == "signed-in" {
		return false
	}
	_, known := l.byKey[key]
	return !known && len(l.byKey) >= maxLimitKeys
}
