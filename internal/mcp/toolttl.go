package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"sync"
	"time"
)

// maxToolTTL is the longest ttlMs a server may ask for (24 hours). Anything the client
// cannot read as a whole number of milliseconds from 1 to this limit is not kept: zero or
// less means stale (MCP 2026-07-28 "Caching"), and a float, a string or a huge number is
// a reply this client does not trust enough to hold a list for. Only a missing ttlMs
// gets the default.
const maxToolTTL = 24 * time.Hour

// ttlSpy reads the ttlMs of tools/list replies off the bytes of a stream (newline
// separated JSON, or server-sent events) without touching them. The SDK's result type
// has no ttlMs field.
type ttlSpy struct {
	mu       sync.Mutex
	min      int64
	explicit bool
}

func (t *ttlSpy) wrap(r io.Reader) io.Reader { return &spyReader{r: r, spy: t} }

// ttl is the smallest ttlMs of the pages seen, or the default when no page gave one.
func (t *ttlSpy) ttl() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.explicit {
		return defaultToolTTL
	}
	return time.Duration(t.min) * time.Millisecond
}

// ttlMillis reads a ttlMs token. Zero means do not keep.
func ttlMillis(raw json.RawMessage) int64 {
	s := string(bytes.TrimSpace(raw))
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 || n > maxToolTTL.Milliseconds() {
		return 0
	}
	return n
}

// message reads one JSON-RPC message.
func (t *ttlSpy) message(b []byte) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return
	}
	var m struct {
		Result *struct {
			Tools json.RawMessage `json:"tools"`
			TTL   json.RawMessage `json:"ttlMs"`
		} `json:"result"`
	}
	if json.Unmarshal(b, &m) != nil || m.Result == nil || m.Result.Tools == nil || m.Result.TTL == nil {
		return
	}
	v := ttlMillis(m.Result.TTL)
	t.mu.Lock()
	if !t.explicit || v < t.min {
		t.min = v
	}
	t.explicit = true
	t.mu.Unlock()
}

const spyLineMax = 4 << 20

// spyReader hands every message to the spy: a line of JSON, or the data lines of one
// server-sent event joined with newlines.
type spyReader struct {
	r     io.Reader
	spy   *ttlSpy
	line  []byte
	event []byte
	over  bool
}

func (p *spyReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	chunk := b[:n]
	for len(chunk) > 0 {
		i := bytes.IndexByte(chunk, '\n')
		if i < 0 {
			p.add(chunk)
			break
		}
		p.add(chunk[:i])
		p.flush()
		chunk = chunk[i+1:]
	}
	if err != nil {
		p.flush()
		p.send()
	}
	return n, err
}

func (p *spyReader) add(b []byte) {
	if p.over || len(p.line)+len(p.event)+len(b) > spyLineMax {
		p.over, p.line, p.event = true, nil, nil
		return
	}
	p.line = append(p.line, b...)
}

// flush takes one finished line.
func (p *spyReader) flush() {
	line := bytes.TrimSpace(p.line)
	over := p.over
	p.line, p.over = p.line[:0], false
	if over {
		return
	}
	switch {
	case len(line) == 0:
		p.send()
	case bytes.HasPrefix(line, []byte("data:")):
		if len(p.event) > 0 {
			p.event = append(p.event, '\n')
		}
		p.event = append(p.event, bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))...)
	case line[0] == '{':
		p.spy.message(line)
	}
}

// send ends an event.
func (p *spyReader) send() {
	if len(p.event) > 0 {
		p.spy.message(p.event)
	}
	p.event = p.event[:0]
}
