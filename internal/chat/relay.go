package chat

import "strings"

// maxRelayReply caps the reply a relay quotes; the rest is cut with a note.
const maxRelayReply = 64 << 10

// RelayRef marks a user message that carries another chat's reply: the chat,
// its agent and the message it came from. Every relay is one click of the user.
type RelayRef struct {
	Chat  string `json:"chat"`
	Agent string `json:"agent"`
	Msg   int    `json:"msg"`
}

// Relay sends one finished reply of the agent in chat fromID to this chat, quoted
// after the user's note. This chat must be idle; nothing is sent on from here.
func (c *Chat) Relay(fromID string, msgIdx int, note string) error {
	if fromID == c.id {
		return apiErr("BAD_REQUEST", "Send the reply to another chat.")
	}
	from := c.m.get(fromID)
	if from == nil {
		return apiErr("NOT_FOUND", "The chat with that reply does not exist.")
	}
	if err := from.load(); err != nil {
		return apiErr("INTERNAL", "The saved chat could not be read.")
	}
	reply, err := from.finishedReply(msgIdx)
	if err != nil {
		return err
	}
	note = strings.TrimSpace(note)
	if len(note) > maxPromptText-maxRelayReply-1024 {
		return apiErr("BAD_REQUEST", "The note is too long.")
	}
	agent := from.spec.Agent.ID
	name := c.m.catalog[agent].Name
	if name == "" {
		name = agent
	}
	text := "Reply from " + name + " in the chat \"" + from.Info().Title + "\":\n" + quote(cut(reply, maxRelayReply))
	if note != "" {
		text = note + "\n\n" + text
	}
	_, _, err = c.beginTurn(turnReq{text: text, relay: &RelayRef{Chat: fromID, Agent: agent, Msg: msgIdx}})
	return err
}

// finishedReply is the text of agent message idx, once the agent has finished it.
func (c *Chat) finishedReply(idx int) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.msgs {
		if m.ID != idx {
			continue
		}
		if m.Kind != KindAgent || strings.TrimSpace(m.Text) == "" {
			return "", apiErr("BAD_REQUEST", "Only a reply of the agent can be sent on.")
		}
		if c.state == StateRunning && c.openIdx == idx {
			return "", apiErr("BUSY", "That reply is still being written. Wait for it to finish.")
		}
		return m.Text, nil
	}
	return "", apiErr("NOT_FOUND", "That reply is not in the chat.")
}
