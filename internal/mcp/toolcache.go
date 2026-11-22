package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

// defaultToolTTL is how long a tool list stays fresh when the server gives no ttlMs.
// With ttlMs the server decides (MCP 2026-07-28 "Caching",
// https://modelcontextprotocol.io/specification/2026-07-28/server/utilities/caching):
// 0 or less means stale at once, so such a list is never kept.
const defaultToolTTL = 10 * time.Minute

type cachedTools struct {
	key   string
	tools []Tool
	at    time.Time
	ttl   time.Duration
}

// toolCache keeps the last good tool list of each server in memory. An entry serves
// only while its key (the config hash) is the same and the ttl has not run out.
type toolCache struct {
	mu sync.Mutex
	m  map[string]cachedTools
}

func (c *toolCache) get(id, key string, now time.Time) ([]Tool, time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[id]
	if !ok || e.key != key || now.Sub(e.at) >= e.ttl || now.Before(e.at) {
		return nil, time.Time{}, false
	}
	return append([]Tool(nil), e.tools...), e.at, true
}

func (c *toolCache) put(id, key string, tools []Tool, at time.Time, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]cachedTools{}
	}
	if ttl <= 0 {
		delete(c.m, id)
		return
	}
	c.m[id] = cachedTools{key: key, tools: tools, at: at, ttl: ttl}
}

func (c *toolCache) drop(id string) {
	c.mu.Lock()
	delete(c.m, id)
	c.mu.Unlock()
}

// cacheKey hashes what a tool list depends on: the command line or address, the saved
// names and values of env and headers, the private-address switch and the sign-in
// state. Values are hashed, never kept. "" means the key could not be made.
func (s *Service) cacheKey(r rec) string {
	h := sha256.New()
	add := func(p string) { _, _ = fmt.Fprintf(h, "%d:%s;", len(p), p) }
	add(r.sig())
	add(fmt.Sprint(r.AllowPrivate))
	add(r.SignIn)
	for _, g := range []struct {
		vars []Var
		key  func(id, name string) string
	}{{r.Env, envKey}, {r.Headers, headerKey}} {
		vals, _, err := s.values(r.ID, g.vars, g.key)
		if err != nil {
			return ""
		}
		names := make([]string, 0, len(vals))
		for n := range vals {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			add(n)
			add(vals[n])
		}
		add("|")
	}
	return hex.EncodeToString(h.Sum(nil))
}
