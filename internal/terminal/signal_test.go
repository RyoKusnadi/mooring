package terminal

import (
	"strings"
	"testing"
	"time"
)

func TestScannerKinds(t *testing.T) {
	long := "\x1b]9;" + strings.Repeat("x", maxSignalBody+10) + "\a"
	cases := []struct {
		name string
		in   []string
		want signalKind
	}{
		{"osc 9 text", []string{"\x1b]9;done\a"}, sigNotify},
		{"osc 9 with ST", []string{"\x1b]9;done\x1b\\"}, sigNotify},
		{"osc 9;4 progress", []string{"\x1b]9;4;1;50\a"}, sigNone},
		{"osc 9 progress clear", []string{"\x1b]9;4;0\a"}, sigNone},
		{"osc 9 numeric subcode", []string{"\x1b]9;1;500\a"}, sigNone},
		{"osc 9 empty", []string{"\x1b]9;\a"}, sigNone},
		{"osc 777 notify", []string{"\x1b]777;notify;title;body\a"}, sigNotify},
		{"osc 777 other", []string{"\x1b]777;precmd\a"}, sigNone},
		{"osc 99", []string{"\x1b]99;i=1:d=0;Hello\x1b\\"}, sigNotify},
		{"bare bell", []string{"ding\a"}, sigBell},
		{"bell ending a title is not a bell", []string{"\x1b]0;my title\a"}, sigNone},
		{"bell ending a string is not a bell", []string{"\x1bPq data\a"}, sigNone},
		{"split across reads", []string{"\x1b]9;do", "ne\x1b", "\\"}, sigNotify},
		{"split before the number", []string{"\x1b", "]", "777;notify;a;b", "\a"}, sigNotify},
		{"over the cap", []string{long}, sigNone},
		{"notify beats bell", []string{"\a\x1b]9;x\a"}, sigNotify},
		{"cancelled sequence", []string{"\x1b]9;x\x18ok"}, sigNone},
		{"after a cap the next one works", []string{long, "\x1b]9;ok\a"}, sigNotify},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var s scanner
			got := sigNone
			for _, in := range c.in {
				if k := s.feed([]byte(in)); k > got {
					got = k
				}
			}
			if got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}

func TestScannerKeepsNoMoreThanCap(t *testing.T) {
	var s scanner
	s.feed([]byte("\x1b]9;" + strings.Repeat("y", 3*maxSignalBody)))
	if len(s.body) > maxSignalBody {
		t.Fatalf("kept %d bytes", len(s.body))
	}
}

func TestSignalRateLimit(t *testing.T) {
	var g signalState
	t0 := time.Unix(1000, 0)
	ring := []byte("\a")
	if !g.raise(ring, t0) || !g.bell {
		t.Fatal("first bell should raise")
	}
	if g.raise(ring, t0.Add(time.Second)) {
		t.Fatal("raised while still on")
	}
	g.on, g.bell = false, false
	if g.raise(ring, t0.Add(5*time.Second)) {
		t.Fatal("bell raised inside 10 s")
	}
	// A real notification is not limited, and upgrades a bell that is still up.
	if !g.raise([]byte("\x1b]9;done\a"), t0.Add(6*time.Second)) || g.bell || !g.on {
		t.Fatal("a notification right after a bell should land as a real signal")
	}
	if g.raise([]byte("\x1b]9;again\a"), t0.Add(7*time.Second)) {
		t.Fatal("a second notification while on should not publish again")
	}
	g.on, g.bell = false, false
	if !g.raise(ring, t0.Add(20*time.Second)) {
		t.Fatal("bell should raise after 10 s")
	}
	if !g.raise([]byte("\x1b]777;notify;a;b\a"), t0.Add(21*time.Second)) || g.bell {
		t.Fatal("a notification should upgrade a bell")
	}
}
