package chat

import "context"

// Checkpoints saves the working tree of a chat's folder around each turn, so the
// browser can show what the agent changed (internal/git).
type Checkpoints interface {
	// Checkpoint saves the folder as the start (end false) or end of a turn.
	Checkpoint(ctx context.Context, projectID, dir, chatID string, end bool) error
	// DropCheckpoints deletes every checkpoint of a chat.
	DropCheckpoints(ctx context.Context, dir, chatID string) error
}

// checkpoint saves a checkpoint of the chat's folder. A failure is logged and
// never fails the turn.
func (c *Chat) checkpoint(end bool) {
	cp := c.m.Checkpoints
	if cp == nil {
		return
	}
	c.mu.Lock()
	dir, project := c.dir, c.projectID
	c.mu.Unlock()
	if err := cp.Checkpoint(c.baseContext(), project, dir, c.id, end); err != nil {
		c.m.log.Warn("chat: checkpoint failed", "chat", c.id, "end", end, "err", err)
	}
}

// dropCheckpoints deletes the checkpoints of a chat that is being removed.
func (m *Manager) dropCheckpoints(c *Chat) {
	if m.Checkpoints == nil {
		return
	}
	c.mu.Lock()
	dir := c.dir
	c.mu.Unlock()
	ctx := m.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.Checkpoints.DropCheckpoints(ctx, dir, c.id); err != nil {
		m.log.Warn("chat: deleting checkpoints failed", "chat", c.id, "err", err)
	}
}

// ChatFolder tells the folder and title of a chat, for the git service.
func (m *Manager) ChatFolder(projectID, chatID string) (dir, title string, ok bool) {
	c := m.get(chatID)
	if c == nil {
		return "", "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.projectID != projectID {
		return "", "", false
	}
	return c.dir, c.title, true
}
