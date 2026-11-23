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

// stoppedPair is chat a with one reply and a chat b that was closed, so it is stopped.
func stoppedPair(t *testing.T, env ...string) (*rig, *Chat, *Chat, Message) {
	r := newRig(t, env...)
	a, b := r.start("a"), r.start("b")
	r.ask(a, "hello")
	if _, err := r.run("chat.close", map[string]any{"id": "b"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(b, StateStopped)
	return r, a, b, lastMsg(a, KindAgent)
}

func relayTo(r *rig, to, from string, idx int) error {
	_, err := r.run("chat.relay", map[string]any{"id": to, "from": from, "msgIdx": idx, "text": "Check this."})
	return err
}

func TestRelayStartsAStoppedChat(t *testing.T) {
	r, _, b, reply := stoppedPair(t)
	if err := relayTo(r, "b", "a", reply.ID); err != nil {
		t.Fatal(err)
	}
	r.waitState(b, StateIdle)
	got := lastMsg(b, KindUser)
	if got.Relay == nil || got.Relay.Chat != "a" || !strings.HasPrefix(got.Text, "Check this.") {
		t.Fatalf("relay message: %+v", got)
	}
	if lastMsg(b, KindAgent).Text == "" {
		t.Fatal("the restarted agent did not answer")
	}
}

func TestRelayToStoppedChatThatCannotStart(t *testing.T) {
	r, _, b, reply := stoppedPair(t)
	def := r.m.catalog["fake"]
	def.Command = "false" // starts, then exits before it answers
	r.m.catalog["fake"] = def
	err := relayTo(r, "b", "a", reply.ID)
	if err == nil || !has(err.Error(), "nothing was sent") {
		t.Fatalf("error: %v", err)
	}
	r.waitState(b, StateStopped)
	if lastMsg(b, KindUser).Relay != nil {
		t.Fatal("a message was sent to a chat that did not start")
	}
	def.Command = "/nonexistent/agent"
	r.m.catalog["fake"] = def
	if err := relayTo(r, "b", "a", reply.ID); err == nil || !has(err.Error(), "could not be started") {
		t.Fatalf("missing agent: %v", err)
	}
}

func TestRelayToStoppedChatThatBecameBusy(t *testing.T) {
	r, _, b, reply := stoppedPair(t)
	if _, err := r.run("chat.resume", map[string]any{"id": "b"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(b, StateIdle)
	r.prompt(b, "long")
	waitFor(t, "running", func() bool { return b.Info().State == StateRunning })
	if err := relayTo(r, "b", "a", reply.ID); err == nil || !has(err.Error(), "BUSY") {
		t.Fatalf("busy target: %v", err)
	}
	for _, m := range b.messages() {
		if m.Relay != nil {
			t.Fatal("a reply was sent to a chat that was answering")
		}
	}
}
