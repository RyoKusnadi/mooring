package chat

// remove deletes a chat and its saved messages, and drops it from its group.
func (m *Manager) remove(id string) (any, error) {
	m.mu.Lock()
	c := m.chats[id]
	delete(m.chats, id)
	m.mu.Unlock()
	if c == nil {
		return map[string]any{}, nil
	}
	c.discard()
	if _, err := m.db.Exec(`DELETE FROM chats WHERE id = ?`, id); err != nil {
		m.log.Error("chat: delete failed", "chat", id, "err", err)
		return nil, apiErr("INTERNAL", "The chat could not be deleted.")
	}
	m.forgetSession(c)
	m.removeImages(id)
	m.pub.Publish("chat.removed", map[string]string{"id": id, "projectId": c.projectID})
	c.mu.Lock()
	g := c.groupID
	c.mu.Unlock()
	if g != "" {
		if err := m.regroup(nil, []string{g}, nil); err != nil {
			m.log.Warn("chat: group not updated after a removal", "chat", id)
		}
	}
	return map[string]any{}, nil
}
