package chat

import "testing"

func TestFinishedSinceSeen(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	if in := c.Info(); in.Unseen || in.FinishedAt != 0 {
		t.Fatalf("a new chat: %+v", in)
	}
	seen := func() {
		t.Helper()
		if _, err := r.run("chat.seen", map[string]any{"id": "c1"}); err != nil {
			t.Fatal(err)
		}
	}
	r.ask(c, "hello")
	if in := c.Info(); !in.Unseen || in.FinishedAt == 0 {
		t.Fatalf("after a reply: %+v", in)
	}
	seen()
	if in := c.Info(); in.Unseen || in.SeenAt < in.FinishedAt {
		t.Fatalf("after seen: %+v", in)
	}
	before := r.rec.count("chat.changed")
	seen()
	if r.rec.count("chat.changed") != before {
		t.Fatal("seen again published a change")
	}

	// A job the user asked for (the summary) does not count as a new reply.
	if _, err := r.run("chat.summarize", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	if c.Info().Unseen {
		t.Fatal("the summary job made the chat unseen")
	}

	r.ask(c, "again")
	want := c.Info()
	if !want.Unseen {
		t.Fatal("a second reply was not unseen")
	}
	r.m.Shutdown()
	r.boot()
	if got := r.m.get("c1").Info(); got.FinishedAt != want.FinishedAt || got.SeenAt != want.SeenAt || !got.Unseen {
		t.Fatalf("after restart %+v, want %+v", got, want)
	}
	if _, err := r.run("chat.seen", map[string]any{"id": "nope"}); err == nil {
		t.Fatal("seen on a missing chat")
	}
	r.m.Shutdown()
}
