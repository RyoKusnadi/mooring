package terminal

import (
	"fmt"
	"slices"
)

// scanner states
const (
	stGround = iota
	stEsc
	stCSI     // after ESC [
	stCSIPriv // after ESC [ ?
	stString  // OSC, DCS, SOS, PM, APC: runs to BEL or ST
	stStringEsc
)

// replayModes are the DEC private modes worth restoring before a replay.
// Anything else only matters if the program repaints.
var replayModes = map[int]bool{
	1: true, 6: true, 7: true, 12: true, 25: true, 47: true, 66: true,
	1000: true, 1002: true, 1003: true, 1004: true, 1005: true, 1006: true,
	1015: true, 1016: true, 1047: true, 1049: true, 2004: true,
}

// tracker follows DEC private modes (CSI ? Pm h / l) through the output. It
// keeps its place across chunk boundaries, so a sequence split in two is read
// correctly. It was proven against a real terminal first.
type tracker struct {
	state int
	param []byte
	modes map[int]bool
}

func newTracker() *tracker { return &tracker{modes: map[int]bool{}} }

func (t *tracker) clone() *tracker {
	c := &tracker{state: t.state, param: slices.Clone(t.param), modes: make(map[int]bool, len(t.modes))}
	for k, v := range t.modes {
		c.modes[k] = v
	}
	return c
}

func (t *tracker) feed(b []byte) {
	for _, c := range b {
		switch t.state {
		case stGround:
			if c == 0x1b {
				t.state = stEsc
			}
		case stEsc:
			switch c {
			case '[':
				t.state = stCSI
			case ']', 'P', 'X', '^', '_':
				t.state = stString
			case 0x1b:
			default:
				t.state = stGround
			}
		case stCSI:
			switch {
			case c == '?':
				t.state, t.param = stCSIPriv, t.param[:0]
			case c == 0x1b:
				t.state = stEsc
			case c >= 0x40 && c <= 0x7e:
				t.state = stGround
			}
		case stCSIPriv:
			switch {
			case c >= '0' && c <= '9' || c == ';':
				if len(t.param) < 64 {
					t.param = append(t.param, c)
				}
			case c == 'h' || c == 'l':
				t.apply(c == 'h')
				t.state = stGround
			case c == 0x1b:
				t.state = stEsc
			case c >= 0x40 && c <= 0x7e:
				t.state = stGround
			}
		case stString:
			switch c {
			case 0x07:
				t.state = stGround
			case 0x1b:
				t.state = stStringEsc
			}
		case stStringEsc:
			// ESC \ ends the string; ESC followed by anything else ends it too.
			t.state = stGround
			if c == '[' {
				t.state = stCSI
			}
		}
	}
}

func (t *tracker) apply(on bool) {
	n := 0
	flush := func() {
		if n != 0 {
			t.modes[n] = on
		}
		n = 0
	}
	for _, p := range t.param {
		if p == ';' {
			flush()
		} else {
			n = min(n*10+int(p-'0'), 100000)
		}
	}
	flush()
}

// alt reports whether the program is on the alternate screen.
func (t *tracker) alt() bool { return t.modes[1049] || t.modes[47] || t.modes[1047] }

// preamble is the escape sequences that put a fresh terminal into this mode state.
func (t *tracker) preamble() string {
	keys := make([]int, 0, len(t.modes))
	for k := range t.modes {
		if replayModes[k] {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	var out []byte
	for _, k := range keys {
		c := byte('l')
		if t.modes[k] {
			c = 'h'
		}
		out = fmt.Appendf(out, "\x1b[?%d%c", k, c)
	}
	return string(out)
}

// safeStart finds where a replay can begin when the ring starts at head's
// position: past the rest of a cut escape sequence and any cut UTF-8
// character. It returns how many bytes to skip and the mode state after them.
// first should be the whole ring: a string can run for any length.
func safeStart(head *tracker, first []byte) (int, *tracker) {
	t := head.clone()
	i := 0
	for i < len(first) && (t.state != stGround || first[i]&0xC0 == 0x80) {
		t.feed(first[i : i+1])
		i++
	}
	// No boundary in all of first: the ring is inside one long string, so
	// replay none of it rather than from the middle.
	return i, t
}
