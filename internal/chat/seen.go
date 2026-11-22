package chat

// Seen notes that the user looked at the chat now. Only a chat that finished a
// reply since it was last seen changes, so a tab that is shown often costs nothing.
func (c *Chat) Seen() {
	c.mu.Lock()
	if c.finished <= c.seen {
		c.mu.Unlock()
		return
	}
	c.seen = max(c.m.now().UnixMilli(), c.finished)
	c.mu.Unlock()
	c.publishInfo("chat.changed")
}
