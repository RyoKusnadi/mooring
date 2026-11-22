package chat

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const flushAfter = 500 * time.Millisecond

func (m *Manager) insertRow(c *Chat, info Info) error {
	caps, _ := json.Marshal(info.Caps)
	_, err := m.db.Exec(`INSERT INTO chats (id, project_id, agent, title, session_id, spec, caps, mode, model, usage, from_chat, instructions, created_at, updated_at)
		VALUES (?, ?, ?, ?, '', ?, ?, '', '', '', ?, ?, ?, ?)`,
		c.id, c.projectID, info.Agent, info.Title, c.spec.marshal(), string(caps), c.fromChat, c.instructions, info.CreatedAt, info.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save chat: %w", err)
	}
	return nil
}

// saveRow updates the saved chat. A chat that was removed has no row, so nothing changes.
func (m *Manager) saveRow(info Info, session string) {
	caps, _ := json.Marshal(info.Caps)
	usage := ""
	if info.Usage != nil {
		raw, _ := json.Marshal(info.Usage)
		usage = string(raw)
	}
	_, err := m.db.Exec(`UPDATE chats SET title = ?, session_id = ?, caps = ?, mode = ?, model = ?, usage = ?, spec = ?, updated_at = ?,
		finished_at = ?, seen_at = ? WHERE id = ?`,
		info.Title, session, string(caps), info.Mode, info.Model, usage, info.Spec.marshal(), info.UpdatedAt,
		info.FinishedAt, info.SeenAt, info.ID)
	if err != nil {
		m.log.Error("chat: save failed", "chat", info.ID, "err", err)
	}
}

// loadChats reads every saved chat (without messages). They start stopped.
func (m *Manager) loadChats() error {
	rows, err := m.db.Query(`SELECT c.id, c.project_id, c.title, c.session_id, c.spec, c.caps, c.mode, c.model, c.usage, c.from_chat,
		c.created_at, c.updated_at, c.finished_at, c.seen_at, c.instructions, c.group_id, (SELECT COUNT(*) FROM chat_messages WHERE chat_id = c.id) FROM chats c`)
	if err != nil {
		return fmt.Errorf("list chats: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var withMessages []*Chat
	for rows.Next() {
		var id, project, title, session, spec, capsJSON, mode, model, usage, from, instructions string
		var group sql.NullString
		var created, updated, finished, seen int64
		var count int
		if err := rows.Scan(&id, &project, &title, &session, &spec, &capsJSON, &mode, &model, &usage, &from, &created, &updated, &finished, &seen, &instructions, &group, &count); err != nil {
			return fmt.Errorf("read chat row: %w", err)
		}
		c, err := newChat(m, id, project, "", "", Spec{})
		if err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(spec), &c.spec); err != nil {
			m.log.Error("chat: saved spec unreadable", "chat", id, "err", err)
			continue
		}
		_ = json.Unmarshal([]byte(capsJSON), &c.caps)
		if usage != "" {
			c.usage = &Usage{}
			if json.Unmarshal([]byte(usage), c.usage) != nil {
				c.usage = nil
			}
		}
		c.title, c.sessionID, c.mode, c.model, c.created, c.updated = title, session, mode, model, created, updated
		c.fromChat, c.finished, c.seen = from, finished, seen
		c.instructions, c.groupID = instructions, group.String
		c.loaded, c.savedCount = count == 0, count
		m.chats[id] = c
		if count > 0 {
			withMessages = append(withMessages, c)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_ = rows.Close() // the last lines are read after the list, one query at a time
	for _, c := range withMessages {
		c.savedLast = m.savedLastLine(c.id)
	}
	return nil
}

// load reads the saved messages once. Permission requests that were open when the
// server stopped are marked cancelled: the agent that asked is gone.
func (c *Chat) load() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		return nil
	}
	rows, err := c.m.db.Query(`SELECT idx, body FROM chat_messages WHERE chat_id = ? ORDER BY idx`, c.id)
	if err != nil {
		return fmt.Errorf("read messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var msgs []Message
	for rows.Next() {
		var idx int
		var body string
		if err := rows.Scan(&idx, &body); err != nil {
			return fmt.Errorf("read message row: %w", err)
		}
		var msg Message
		if err := json.Unmarshal([]byte(body), &msg); err != nil || idx != len(msgs) {
			c.m.log.Error("chat: saved message unreadable", "chat", c.id, "idx", idx)
			break
		}
		if msg.Kind == KindPermission && msg.State == PermPending {
			msg.State = PermCancelled
		}
		msgs = append(msgs, msg)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read messages: %w", err)
	}
	c.msgs, c.loaded = msgs, true
	return nil
}

func (c *Chat) markDirtyLocked(i int) {
	c.dirty[i] = struct{}{}
	if c.flushT == nil {
		c.flushT = time.AfterFunc(flushAfter, c.flush)
	}
}

// flush writes the messages that changed since the last flush.
func (c *Chat) flush() {
	c.flushMu.Lock()
	defer c.flushMu.Unlock()
	c.mu.Lock()
	c.flushT = nil
	if c.removed || len(c.dirty) == 0 {
		c.mu.Unlock()
		return
	}
	batch := make(map[int]Message, len(c.dirty))
	for i := range c.dirty {
		batch[i] = c.msgs[i]
	}
	c.dirty = map[int]struct{}{}
	c.mu.Unlock()
	tx, err := c.m.db.Begin()
	if err != nil {
		c.m.log.Error("chat: save messages", "chat", c.id, "err", err)
		return
	}
	defer func() { _ = tx.Rollback() }()
	for i, msg := range batch {
		body, _ := json.Marshal(msg)
		if _, err := tx.Exec(`INSERT INTO chat_messages (chat_id, idx, body) VALUES (?, ?, ?)
			ON CONFLICT(chat_id, idx) DO UPDATE SET body = excluded.body`, c.id, i, string(body)); err != nil {
			c.m.log.Error("chat: save message", "chat", c.id, "err", err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.m.log.Error("chat: save messages", "chat", c.id, "err", err)
	}
}
