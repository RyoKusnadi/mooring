package chat

import (
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
)

// filterCompact is the one output filter there is (internal/compact).
const filterCompact = "compact"

// checkFilters accepts [] or ["compact"]; anything else is BAD_REQUEST.
func checkFilters(in []string) ([]string, error) {
	out := []string{}
	for _, f := range in {
		if f != filterCompact {
			return nil, apiErr("BAD_REQUEST", "%q is not an output filter. The only one is %q.", f, filterCompact)
		}
		if !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out, nil
}

// SetFilters turns output filters on or off; the next terminal/output sees the change.
func (c *Chat) SetFilters(in []string) (Info, error) {
	f, err := checkFilters(in)
	if err != nil {
		return Info{}, err
	}
	c.mu.Lock()
	c.spec.Filters = f
	c.touchLocked()
	c.mu.Unlock()
	c.publishInfo("chat.changed")
	return c.Info(), nil
}

// compactDefault reads the setting that new chats start with compaction on
// (settings key "compactDefault", a JSON boolean).
func (m *Manager) compactDefault() bool {
	var raw string
	err := m.db.QueryRow(`SELECT value FROM settings WHERE key = 'compactDefault'`).Scan(&raw)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			m.log.Warn("chat: read compactDefault", "err", err)
		}
		return false
	}
	var on bool
	return json.Unmarshal([]byte(raw), &on) == nil && on
}
