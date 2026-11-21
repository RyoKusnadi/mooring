package chat

import (
	"encoding/json"
	"strings"
)

const lastLineRunes = 120

// lastLineOf is the short line a chat list shows under the title: the last non-empty line of the
// newest message the user or the agent wrote. It goes through cleanLine, so secrets stay hidden.
func lastLineOf(msgs []Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if (m.Kind != KindAgent && m.Kind != KindUser) || m.Job != "" {
			continue
		}
		text := strings.TrimSpace(m.Text)
		if r := []rune(text); len(r) > 2000 {
			text = string(r[len(r)-2000:]) // only the end counts, so a long reply costs little
		}
		lines := strings.Split(text, "\n")
		for j := len(lines) - 1; j >= 0; j-- {
			if l := cleanLine(lines[j], lastLineRunes); l != "" {
				return l
			}
		}
	}
	return ""
}

// savedLastLine reads the last line of a chat whose messages are not loaded yet.
func (m *Manager) savedLastLine(id string) string {
	rows, err := m.db.Query(`SELECT body FROM chat_messages WHERE chat_id = ? ORDER BY idx DESC LIMIT 12`, id)
	if err != nil {
		return ""
	}
	defer func() { _ = rows.Close() }()
	var newest []Message
	for rows.Next() {
		var body string
		var msg Message
		if rows.Scan(&body) == nil && json.Unmarshal([]byte(body), &msg) == nil {
			newest = append([]Message{msg}, newest...)
		}
	}
	return lastLineOf(newest)
}
