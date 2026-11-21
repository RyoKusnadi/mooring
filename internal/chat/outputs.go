package chat

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/ryokusnadi/mooring/internal/compact"
)

// maxOutputs is how many raw outputs a chat keeps; the oldest go first.
const maxOutputs = 50

// OutputRef is what a tool message says about the raw output of its command.
type OutputRef struct {
	ID        string `json:"id"`
	Lines     int    `json:"lines"`
	Bytes     int    `json:"bytes"`
	KeptLines int    `json:"keptLines"` // lines the agent was given
	Compacted bool   `json:"compacted"`
}

func (c *Chat) hasFilter(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range c.spec.Filters {
		if f == name {
			return true
		}
	}
	return false
}

// termOutput answers terminal/output: the output cut to the agent's limit and,
// while "compact" is on, filtered. The raw text is saved for the user.
func (c *Chat) termOutput(t *term) (any, error) {
	raw, cut := t.snapshot()
	out, limited := lastBytes(raw, t.limit)
	res := map[string]any{"output": out, "truncated": cut || limited, "exitStatus": t.status()}
	if raw == "" {
		return res, nil
	}
	ref := OutputRef{ID: t.outID, Lines: compact.CountLines(raw), Bytes: len(raw), KeptLines: compact.CountLines(out)}
	if c.hasFilter("compact") {
		filtered, fr := compact.Apply(out)
		if fr.Applied {
			res["output"], ref.KeptLines, ref.Compacted = filtered, fr.Kept, true
		}
	}
	if ref.ID == "" {
		ref.ID = randomID("o")
		t.outID = ref.ID
	}
	var code *int
	if st := t.status(); st != nil {
		code = st.ExitCode
	}
	if err := c.saveOutput(t.id, ref, raw, code); err != nil {
		c.m.log.Error("chat: save output", "chat", c.id, "err", err)
		return res, nil
	}
	c.linkOutput(t.id, ref)
	return res, nil
}

// saveOutput writes (or, on a later read of the same terminal, replaces) the raw
// output and drops the chat's oldest ones past the limit.
func (c *Chat) saveOutput(termID string, ref OutputRef, raw string, exit *int) error {
	tool := c.toolOf(termID)
	if _, err := c.m.db.Exec(`INSERT INTO chat_outputs (chat_id, id, tool_id, raw, lines, bytes, kept_lines, exit_code, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(chat_id, id) DO UPDATE SET raw = excluded.raw, lines = excluded.lines, bytes = excluded.bytes,
			kept_lines = excluded.kept_lines, exit_code = excluded.exit_code`,
		c.id, ref.ID, tool, []byte(raw), ref.Lines, ref.Bytes, ref.KeptLines, exit, c.m.now().UnixMilli()); err != nil {
		return err
	}
	_, err := c.m.db.Exec(`DELETE FROM chat_outputs WHERE chat_id = ? AND id NOT IN
		(SELECT id FROM chat_outputs WHERE chat_id = ? ORDER BY rowid DESC LIMIT ?)`, c.id, c.id, maxOutputs)
	return err
}

// toolOf finds the tool call that shows a terminal, "" when none does.
func (c *Chat) toolOf(termID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if i := c.toolForTermLocked(termID); i >= 0 {
		return c.msgs[i].ToolID
	}
	return ""
}

// toolForTermLocked is the tool message whose content names the terminal; failing
// that, the latest command tool that has no output yet.
func (c *Chat) toolForTermLocked(termID string) int {
	for i := len(c.msgs) - 1; i >= 0; i-- {
		m := c.msgs[i]
		if m.Kind == KindTool && strings.Contains(string(m.Content), `"`+termID+`"`) {
			return i
		}
	}
	for i := len(c.msgs) - 1; i >= 0; i-- {
		if m := c.msgs[i]; m.Kind == KindTool && m.ToolKind == "execute" && len(m.Outputs) == 0 && len(m.Content) == 0 {
			return i
		}
	}
	return -1
}

// linkOutput puts the output on its tool message, or keeps it until that message shows up.
func (c *Chat) linkOutput(termID string, ref OutputRef) {
	c.mu.Lock()
	i := c.toolForTermLocked(termID)
	if i < 0 {
		c.pendingOut[termID] = ref
		c.mu.Unlock()
		return
	}
	c.setOutputLocked(i, ref)
	c.mu.Unlock()
}

func (c *Chat) setOutputLocked(i int, ref OutputRef) {
	m := &c.msgs[i]
	for j := range m.Outputs {
		if m.Outputs[j].ID == ref.ID {
			m.Outputs[j] = ref
			c.updateLocked(i)
			return
		}
	}
	m.Outputs = append(m.Outputs, ref)
	c.updateLocked(i)
}

// claimPendingLocked attaches outputs that were read before their tool message
// named the terminal.
func (c *Chat) claimPendingLocked(i int) {
	for termID, ref := range c.pendingOut {
		if strings.Contains(string(c.msgs[i].Content), `"`+termID+`"`) {
			delete(c.pendingOut, termID)
			c.setOutputLocked(i, ref)
		}
	}
}

var errNoOutput = errors.New("no such output")

// loadOutput reads the raw text of a saved output.
func (m *Manager) loadOutput(chatID, id string) (string, error) {
	var raw []byte
	err := m.db.QueryRow(`SELECT raw FROM chat_outputs WHERE chat_id = ? AND id = ?`, chatID, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errNoOutput
	}
	return string(raw), err
}

// OutputHandler serves GET /api/chats/{id}/outputs/{outputId}: the raw text. The
// caller checks the session first.
func (m *Manager) OutputHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, out := r.PathValue("id"), r.PathValue("outputId")
		if m.get(id) == nil || !idPattern.MatchString(out) {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		text, err := m.loadOutput(id, out)
		if errors.Is(err, errNoOutput) {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		if err != nil {
			m.log.Error("chat: read output", "chat", id, "err", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(text))
	})
}

// lastBytes cuts s from the start to at most n bytes, on a character border.
func lastBytes(s string, n int) (string, bool) {
	if n <= 0 || len(s) <= n {
		return s, false
	}
	s = s[len(s)-n:]
	for len(s) > 0 && !utf8.RuneStart(s[0]) {
		s = s[1:]
	}
	return s, true
}
