package chat

import "strings"

// Rename sets the title the user typed. The agent and the title job leave it alone afterwards.
func (c *Chat) Rename(title string) error {
	title = shorten(strings.TrimSpace(title), 120)
	if title == "" {
		return apiErr("BAD_REQUEST", "A chat title cannot be empty.")
	}
	c.mu.Lock()
	c.title, c.agentTitled = title, true
	c.touchLocked()
	c.mu.Unlock()
	c.flush()
	c.publishInfo("chat.changed")
	return nil
}
