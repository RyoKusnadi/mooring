package mcp

import (
	"sort"
	"strings"
	"sync"
)

const rawStderr = 64 << 10

// capture keeps the start of a process's error output. Reading goes on after
// the cap so the process never blocks on a full pipe.
type capture struct {
	mu  sync.Mutex
	buf []byte
}

func (c *capture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if room := rawStderr - len(c.buf); room > 0 {
		c.buf = append(c.buf, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

// text returns the output with every secret removed, cut to maxStderr bytes. The
// secrets come out before the cut, so a value cannot be left half showing.
func (c *capture) text(secrets []string) string {
	c.mu.Lock()
	raw := string(c.buf)
	c.mu.Unlock()
	return clipBytes(scrub(raw, secrets), maxStderr)
}

// scrub replaces each secret value (longest first) with a marker.
func scrub(s string, secrets []string) string {
	vals := make([]string, 0, len(secrets))
	for _, v := range secrets {
		if v != "" {
			vals = append(vals, v)
		}
	}
	sort.Slice(vals, func(i, j int) bool { return len(vals[i]) > len(vals[j]) })
	for _, v := range vals {
		s = strings.ReplaceAll(s, v, "[removed]")
	}
	return strings.ToValidUTF8(s, "?")
}

func clipBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "") + "\n[cut]"
}
