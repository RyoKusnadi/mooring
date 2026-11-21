package chat

import "encoding/json"

// TurnUsage is the `usage` of one prompt result, kept as the agent sent it.
// Token counts are zero when the agent left a field out.
type TurnUsage struct {
	Turn              int             `json:"turn"`
	Raw               json.RawMessage `json:"raw"`
	InputTokens       int64           `json:"inputTokens"`
	OutputTokens      int64           `json:"outputTokens"`
	TotalTokens       int64           `json:"totalTokens"`
	CachedReadTokens  int64           `json:"cachedReadTokens"`
	CachedWriteTokens int64           `json:"cachedWriteTokens"`
	ThoughtTokens     int64           `json:"thoughtTokens"`
}

// noteUsageLocked records the usage of a finished turn.
func (c *Chat) noteUsageLocked(turn int, raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var f struct {
		InputTokens       int64 `json:"inputTokens"`
		OutputTokens      int64 `json:"outputTokens"`
		TotalTokens       int64 `json:"totalTokens"`
		CachedReadTokens  int64 `json:"cachedReadTokens"`
		CachedWriteTokens int64 `json:"cachedWriteTokens"`
		ThoughtTokens     int64 `json:"thoughtTokens"`
	}
	_ = json.Unmarshal(raw, &f)
	u := TurnUsage{Turn: turn, Raw: append(json.RawMessage{}, raw...), InputTokens: f.InputTokens, OutputTokens: f.OutputTokens,
		TotalTokens: f.TotalTokens, CachedReadTokens: f.CachedReadTokens, CachedWriteTokens: f.CachedWriteTokens, ThoughtTokens: f.ThoughtTokens}
	c.usages = append(c.usages, u)
}

// Stats is what a Go caller (the bench) reads about a chat's token use.
type Stats struct {
	Turns         []TurnUsage // one per finished turn that reported usage
	TerminalCalls int         // terminal/* requests the agent made
	AgentTitled   bool        // the agent sent a title of its own
}

// Stats returns the token numbers of a chat.
func (m *Manager) Stats(id string) (Stats, error) {
	c, err := m.must(id)
	if err != nil {
		return Stats{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{Turns: append([]TurnUsage{}, c.usages...), TerminalCalls: c.termCalls, AgentTitled: c.agentTitled}, nil
}
