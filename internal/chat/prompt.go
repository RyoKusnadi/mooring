package chat

import (
	"context"
	"strings"
)

// turnReq is one prompt to send. job is "" for the user's own words, or the kind
// of a prompt Mooring sends for the user ("summary").
type turnReq struct {
	text string
	job  string
	atts []sent
	refs []AttachRef
}

// Prompt sends the user's text to the agent and returns once the turn has
// started; the reply arrives in the stream.
func (c *Chat) Prompt(text string, atts []Attachment) error {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > maxPromptText {
		return apiErr("BAD_REQUEST", "A message needs 1 to %d characters.", maxPromptText)
	}
	list, refs, err := c.resolveAttachments(atts)
	if err != nil {
		return err
	}
	_, _, err = c.beginTurn(turnReq{text: text, atts: list, refs: refs})
	return err
}

// beginTurn adds the user message and starts the turn. done closes when the
// turn has ended; idx is the user message.
func (c *Chat) beginTurn(q turnReq) (done <-chan struct{}, idx int, err error) {
	c.mu.Lock()
	r := c.cur
	if c.moving {
		c.mu.Unlock()
		return nil, 0, apiErr("BUSY", "The chat is moving to another account. Try again in a moment.")
	}
	switch c.state {
	case StateIdle:
	case StateRunning:
		c.mu.Unlock()
		return nil, 0, apiErr("BUSY", "The agent is still answering. Cancel it or wait.")
	case StateStarting:
		c.mu.Unlock()
		return nil, 0, apiErr("NOT_READY", "The agent is still starting.")
	case StateNeedsLogin:
		c.mu.Unlock()
		e := c.loginError()
		return nil, 0, apiErr(e.Code, "%s", e.Message)
	default:
		c.mu.Unlock()
		return nil, 0, apiErr("NOT_RUNNING", "The agent is not running. Resume the chat first.")
	}
	summary, fromChat := c.pendingContextLocked()
	blocks := promptBlocks(q.text, c.caps.EmbeddedContext, summary, fromChat, q.atts)
	c.openIdx, c.planIdx = -1, -1
	idx = c.addLocked(Message{Kind: KindUser, Text: q.text, Job: q.job, Attachments: q.refs})
	if c.title == "" && q.job == "" {
		c.title, c.autoTitle = firstLine(q.text), true
	}
	c.state, c.errInfo, c.cancelling = StateRunning, nil, false
	c.limited, c.turnRejected, c.turnResetsAt = false, false, 0
	c.turn++
	turn, sid := c.turn, c.sessionID
	ctx, cancel := context.WithCancel(c.baseContext())
	c.turnCancel = cancel
	sentCh := make(chan struct{})
	c.promptSent = sentCh
	finished := make(chan struct{})
	c.mu.Unlock()
	c.publishInfo("chat.changed")
	go c.runTurn(ctx, r, turn, sid, blocks, sentCh, finished)
	return finished, idx, nil
}

// pendingContextLocked is the summary a fresh chat starts from, until the first user message goes out.
func (c *Chat) pendingContextLocked() (text, fromChat string) {
	for _, m := range c.msgs {
		switch m.Kind {
		case KindUser:
			return "", ""
		case KindContext:
			text, fromChat = m.Text, m.FromChat
		}
	}
	return text, fromChat
}
