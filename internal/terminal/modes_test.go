package terminal

import "testing"

func TestTrackerSplitSequences(t *testing.T) {
	tr := newTracker()
	for _, part := range []string{"hello \x1b[?10", "49h text \x1b", "[?25l\x1b[?2004", "h", "\x1b]0;a title\x07\x1b[?1049l"} {
		tr.feed([]byte(part))
	}
	want := map[int]bool{1049: false, 25: false, 2004: true}
	for k, v := range want {
		if got, ok := tr.modes[k]; !ok || got != v {
			t.Errorf("mode %d = %v (seen %v), want %v", k, got, ok, v)
		}
	}
	if tr.alt() {
		t.Error("alt() true after 1049l")
	}
}

func TestTrackerIgnoresStrings(t *testing.T) {
	tr := newTracker()
	tr.feed([]byte("\x1b]0;[?1049h\x07 \x1bP1$r[?47h\x1b\\"))
	if len(tr.modes) != 0 {
		t.Errorf("a mode inside an OSC string was read: %v", tr.modes)
	}
}

func TestPreambleOrderAndFilter(t *testing.T) {
	tr := newTracker()
	tr.feed([]byte("\x1b[?1049h\x1b[?1;25;2004h\x1b[?2026h\x1b[?25l"))
	if got, want := tr.preamble(), "\x1b[?1h\x1b[?25l\x1b[?1049h\x1b[?2004h"; got != want {
		t.Errorf("preamble %q, want %q", got, want)
	}
}

func TestSafeStart(t *testing.T) {
	head := newTracker()
	head.feed([]byte("abc\x1b[?10"))
	// The ring starts inside "ESC [ ? 1 0 4 9 h"; the rest completes the mode.
	skip, tr := safeStart(head, []byte("49hX"))
	if skip != 3 || !tr.modes[1049] {
		t.Errorf("skip %d modes %v", skip, tr.modes)
	}
	// Starts inside a two-byte character.
	skip, _ = safeStart(newTracker(), []byte("\xa9ok"))
	if skip != 1 {
		t.Errorf("utf8 skip %d, want 1", skip)
	}
	// Already at a boundary.
	if skip, _ = safeStart(newTracker(), []byte("ok")); skip != 0 {
		t.Errorf("skip %d, want 0", skip)
	}
}

func TestSafeStartSkipsLongString(t *testing.T) {
	head := newTracker()
	head.feed([]byte("\x1b]0;title"))
	long := append(make([]byte, 10000), "\x07ok"...)
	for i := range long[:10000] {
		long[i] = 'x'
	}
	if skip, _ := safeStart(head, long); skip != 10001 {
		t.Errorf("skip %d, want 10001", skip)
	}
	// A string that never ends in what the ring holds: replay nothing from it.
	if skip, _ := safeStart(head, long[:10000]); skip != 10000 {
		t.Errorf("skip %d, want 10000", skip)
	}
}
