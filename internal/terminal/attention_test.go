package terminal

import (
	"sync"
	"testing"
	"time"
)

func (r *recorder) count(kind string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, k := range r.kind {
		if k == kind {
			n++
		}
	}
	return n
}

// lastAttention keeps the newest attention event of each terminal.
type lastAttention struct {
	mu   sync.Mutex
	last map[string]any
}

func (l *lastAttention) Publish(kind string, data any) {
	if kind != "terminal.attention" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.last = data.(map[string]any)
}

func (l *lastAttention) get(key string) any {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.last[key]
}

func TestSeenBellOnlyKeepsARealSignal(t *testing.T) {
	m, _ := newManager(t, script(`sleep 30`))
	term := open(t, m, "a")
	feed := func(s string) {
		term.mu.Lock()
		term.sig.raise([]byte(s), m.now())
		term.mu.Unlock()
		term.announce()
	}
	feed("\a")
	m.Seen("a", true)
	if term.info().Attention {
		t.Fatal("a bell-only signal should clear")
	}
	feed("\x1b]9;done\a")
	if i := term.info(); !i.Attention || i.Bell {
		t.Fatalf("a real signal right after a clear should land: %+v", i)
	}
	m.Seen("a", true)
	if !term.info().Attention {
		t.Fatal("bell-only seen cleared a real signal")
	}
	m.Seen("a", false)
	if term.info().Attention {
		t.Fatal("seen should clear a real signal")
	}
}

func TestAttentionEventsEndOnTheFinalState(t *testing.T) {
	m, _ := newManager(t, script(`sleep 30`))
	rec := &lastAttention{}
	m.pub = rec
	term := open(t, m, "a")
	for i := 0; i < 300; i++ {
		term.mu.Lock()
		term.sig.on, term.sig.bell = true, false
		term.mu.Unlock()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); term.announce() }()
		term.mu.Lock()
		term.sig.on = false
		term.mu.Unlock()
		go func() { defer wg.Done(); term.announce() }()
		wg.Wait()
		if rec.get("on") == true {
			t.Fatalf("round %d: the last event says on, the state is off", i)
		}
		term.announce()
	}
}

func TestAttentionPublishedOnceAndClearedBySeen(t *testing.T) {
	m, rec := newManager(t, script(`printf '\033]9;done\a'; sleep 30`))
	term := open(t, m, "a")
	waitFor(t, "attention", func() bool { return rec.count("terminal.attention") == 1 })
	if !term.info().Attention {
		t.Fatal("info should carry Attention")
	}
	// A second client replaying the ring does not raise it again.
	c, f := term.attach("", 0)
	if data, _, _ := drain(c); !f.Reset || !contains(data, "done") {
		t.Fatalf("replay: reset=%v data=%q", f.Reset, data)
	}
	time.Sleep(50 * time.Millisecond)
	if n := rec.count("terminal.attention"); n != 1 {
		t.Fatalf("%d attention events after replay", n)
	}
	m.Seen("a", false)
	m.Seen("a", false)
	m.Seen("nope", false)
	if n := rec.count("terminal.attention"); n != 2 {
		t.Fatalf("%d attention events after seen, want 2 (on, off)", n)
	}
	if term.info().Attention {
		t.Fatal("Attention should be cleared")
	}
}
