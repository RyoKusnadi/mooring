package terminal

import (
	"sync"
	"time"
)

const (
	maxSignalBody = 4 << 10          // longer escape bodies are skipped, not kept
	signalEvery   = 10 * time.Second // at most one bell per terminal in this time
)

type scanState uint8

const (
	scGround scanState = iota
	scEsc              // saw ESC
	scOSC              // inside OSC, collecting the body
	scOSCEsc           // ESC inside OSC: a backslash ends it
	scStr              // DCS, SOS, PM or APC string: skipped; ends like OSC
	scStrEsc
)

// signalKind says why a terminal wants the user.
type signalKind uint8

const (
	sigNone signalKind = iota
	sigBell
	sigNotify
)

// scanner finds notification escapes in live PTY bytes. It keeps at most
// maxSignalBody bytes of an escape body, only to classify it, and never exposes them.
type scanner struct {
	state scanState
	body  []byte
	over  bool // the body passed the cap
}

// feed returns the strongest signal found in data: an explicit notification beats a bell.
func (s *scanner) feed(data []byte) signalKind {
	got := sigNone
	for _, c := range data {
		var k signalKind
		switch s.state {
		case scGround:
			switch c {
			case 0x1b:
				s.state = scEsc
			case 0x07:
				k = sigBell
			}
		case scEsc:
			switch c {
			case ']':
				s.state, s.body, s.over = scOSC, s.body[:0], false
			case 'P', 'X', '^', '_':
				s.state = scStr
			case 0x1b:
			default:
				s.state = scGround
			}
		case scOSC:
			switch c {
			case 0x07:
				k = s.finish()
			case 0x1b:
				s.state = scOSCEsc
			case 0x18, 0x1a:
				s.reset()
			default:
				s.add(c)
			}
		case scOSCEsc:
			if c == '\\' {
				k = s.finish()
			} else {
				s.reset()
				if c == ']' {
					s.state = scOSC
				}
			}
		case scStr:
			switch c {
			case 0x07, 0x18, 0x1a:
				s.reset()
			case 0x1b:
				s.state = scStrEsc
			}
		case scStrEsc:
			s.reset()
		}
		if k > got {
			got = k
		}
	}
	return got
}

func (s *scanner) reset() { s.state, s.body, s.over = scGround, s.body[:0], false }

func (s *scanner) add(c byte) {
	if len(s.body) >= maxSignalBody {
		s.over = true
		return
	}
	s.body = append(s.body, c)
}

func (s *scanner) finish() signalKind {
	k := sigNone
	if !s.over {
		k = classify(s.body)
	}
	s.reset()
	return k
}

// classify decides whether an OSC body is a notification. OSC 9 text, OSC 777
// notify and OSC 99 are; OSC 9;4 progress and other numeric OSC 9 subcodes are not.
func classify(b []byte) signalKind {
	num, rest := cut(b)
	switch string(num) {
	case "9":
		first, _ := cut(rest)
		if len(rest) == 0 || allDigits(first) {
			return sigNone
		}
		return sigNotify
	case "777":
		if verb, _ := cut(rest); string(verb) == "notify" {
			return sigNotify
		}
	case "99":
		return sigNotify
	}
	return sigNone
}

func cut(b []byte) (head, tail []byte) {
	for i, c := range b {
		if c == ';' {
			return b[:i], b[i+1:]
		}
	}
	return b, nil
}

func allDigits(b []byte) bool {
	for _, c := range b {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(b) > 0
}

// signalState is what a terminal remembers about its signals.
type signalState struct {
	scan scanner
	on   bool      // Attention is up and not yet seen
	bell bool      // the Attention came from a bell alone
	last time.Time // when the last bell was raised

	// pubMu orders the events: each one carries the state at the moment it is sent.
	pubMu    sync.Mutex
	sentOn   bool
	sentBell bool
}

// raise feeds live bytes and reports whether the state changed. A notification
// escape raises Attention at once and upgrades a bell; only bells are rate limited.
func (g *signalState) raise(data []byte, now time.Time) bool {
	switch g.scan.feed(data) {
	case sigNotify:
		if g.on && !g.bell {
			return false
		}
		g.on, g.bell = true, false
		return true
	case sigBell:
		if g.on || (!g.last.IsZero() && now.Sub(g.last) < signalEvery) {
			return false
		}
		g.on, g.bell, g.last = true, true, now
		return true
	}
	return false
}

// announce publishes the terminal's current Attention if the browsers were last told something else.
func (t *Terminal) announce() {
	g := &t.sig
	g.pubMu.Lock()
	defer g.pubMu.Unlock()
	t.mu.Lock()
	on, bell := g.on, g.on && g.bell
	t.mu.Unlock()
	if on == g.sentOn && bell == g.sentBell {
		return
	}
	g.sentOn, g.sentBell = on, bell
	d := map[string]any{"id": t.id, "projectId": t.projectID, "on": on}
	if bell {
		d["bell"] = true
	}
	t.m.pub.Publish("terminal.attention", d)
}

// Seen clears a terminal's Attention. With bellOnly, a real notification stays. An unknown id, or one
// with nothing to clear, is not an error.
func (m *Manager) Seen(id string, bellOnly bool) {
	t := m.get(id)
	if t == nil {
		return
	}
	t.mu.Lock()
	clear := t.sig.on && (!bellOnly || t.sig.bell)
	if clear {
		t.sig.on, t.sig.bell = false, false
	}
	t.mu.Unlock()
	if clear {
		t.announce()
	}
}
