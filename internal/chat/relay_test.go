package chat

import (
	"strings"
	"testing"
)

func TestRelaySendsOneFinishedReply(t *testing.T) {
	r := newRig(t)
	a, b := r.start("a"), r.start("b")
	r.ask(a, "hello")
	var reply Message
	for _, m := range a.messages() {
		if m.Kind == KindAgent {
			reply = m
		}
	}
	relay := func(to, from string, idx int, note string) error {
		_, err := r.run("chat.relay", map[string]any{"id": to, "from": from, "msgIdx": idx, "text": note})
		return err
	}
	for _, bad := range []struct {
		to, from string
		idx      int
		code     string
	}{
		{"b", "b", reply.ID, "BAD_REQUEST"},
		{"b", "nope", reply.ID, "NOT_FOUND"},
		{"b", "a", 0, "BAD_REQUEST"}, // the user's own message
		{"b", "a", 999, "NOT_FOUND"},
	} {
		if err := relay(bad.to, bad.from, bad.idx, ""); err == nil || !has(err.Error(), bad.code) {
			t.Fatalf("%+v: %v", bad, err)
		}
	}

	if err := relay("b", "a", reply.ID, "Check this."); err != nil {
		t.Fatal(err)
	}
	r.waitState(b, StateIdle)
	var sent *Message
	for _, m := range b.messages() {
		if m.Kind == KindUser {
			sent = &m
		}
	}
	if sent == nil || sent.Relay == nil || sent.Relay.Chat != "a" || sent.Relay.Msg != reply.ID || sent.Job != "" {
		t.Fatalf("relay message: %+v", sent)
	}
	if !strings.HasPrefix(sent.Text, "Check this.\n\nReply from ") || !has(sent.Text, "\n> Hello world") {
		t.Fatalf("text: %q", sent.Text)
	}

	// A reply that is still being written, or a busy target, is refused.
	r.prompt(a, "long")
	waitFor(t, "streaming", func() bool { return lastAgent(a) != reply.Text })
	open := a.messages()[len(a.messages())-1]
	if err := relay("b", "a", open.ID, ""); err == nil || !has(err.Error(), "BUSY") {
		t.Fatalf("open reply: %v", err)
	}
	if err := relay("a", "b", lastMsg(b, KindAgent).ID, ""); err == nil || !has(err.Error(), "BUSY") {
		t.Fatalf("busy target: %v", err)
	}
	r.m.Shutdown()
}

func lastMsg(c *Chat, kind string) Message {
	msgs := c.messages()
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Kind == kind {
			return msgs[i]
		}
	}
	return Message{}
}
