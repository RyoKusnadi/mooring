package chat

import (
	"encoding/json"
	"regexp"
	"strings"
)

const lastLineRunes = 120

// lastLineOf is the short line a chat list shows under the title: the last non-empty line of the
// newest message the user or the agent wrote, outside code blocks and without markdown marks.
// It goes through cleanLine, so secrets stay hidden.
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
		inCode := false
		for j := len(lines) - 1; j >= 0; j-- {
			if codeFence.MatchString(lines[j]) {
				inCode = !inCode
				continue
			}
			if inCode {
				continue
			}
			if l := cleanLine(plainLine(lines[j]), lastLineRunes); l != "" {
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

var (
	codeFence = regexp.MustCompile("^\\s*(```|~~~)")
	ruleLine  = regexp.MustCompile(`^\s*(?:(?:-\s*){3,}|(?:\*\s*){3,}|(?:_\s*){3,}|\|?[\s:|-]*-[\s:|-]*)$`)
	leadMark  = regexp.MustCompile(`^\s*(?:#{1,6}\s+|>\s?|[-*+]\s+)`)
	linkMark  = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	boldMark  = regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	codeMark  = regexp.MustCompile("`([^`]*)`")
)

// plainLine drops the markdown marks a reply line carries (headings, bullets, quotes, bold,
// inline code, links, rules and table borders), so the chat list reads like text.
func plainLine(s string) string {
	if ruleLine.MatchString(s) {
		return ""
	}
	s = leadMark.ReplaceAllString(s, "")
	s = linkMark.ReplaceAllString(s, "$1")
	s = boldMark.ReplaceAllString(s, "$1$2")
	return codeMark.ReplaceAllString(s, "$1")
}
