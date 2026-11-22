package chat

import "encoding/json"

type updateParams struct {
	SessionID string          `json:"sessionId"`
	Update    json.RawMessage `json:"update"`
	State     string          `json:"state"` // auth/status_update
}

type update struct {
	Kind      string          `json:"sessionUpdate"`
	Content   json.RawMessage `json:"content"`
	ToolID    string          `json:"toolCallId"`
	Title     *string         `json:"title"`
	ToolKind  *string         `json:"kind"`
	Status    *string         `json:"status"`
	RawInput  json.RawMessage `json:"rawInput"`
	Locations json.RawMessage `json:"locations"`
	Entries   json.RawMessage `json:"entries"`
	Used      int64           `json:"used"`
	Size      int64           `json:"size"`
	Cost      *struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	} `json:"cost"`
	Meta          map[string]json.RawMessage `json:"_meta"`
	Commands      []Command                  `json:"availableCommands"`
	CurrentModeID string                     `json:"currentModeId"`
	ConfigOptions []json.RawMessage          `json:"configOptions"`
}

func (r *run) notification(method string, params json.RawMessage) {
	c := r.c
	switch method {
	case "session/update":
		var p updateParams
		var u update
		if json.Unmarshal(params, &p) != nil || json.Unmarshal(p.Update, &u) != nil {
			return
		}
		c.onUpdate(r, u)
	}
	// Anything else is ignored on purpose: agents send notifications Mooring does not know.
	// That includes auth/status_update: Claude's adapter (0.107.0) says "unauthenticated" at
	// start even when signed in, so only the -32000 error marks a missing login.
}

// textOf returns the text of a content block, "" for other kinds.
func textOf(raw json.RawMessage) string {
	var b struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &b) != nil || b.Type != "text" {
		return ""
	}
	return b.Text
}

func (c *Chat) onUpdate(r *run, u update) {
	c.mu.Lock()
	if c.cur != r {
		c.mu.Unlock()
		return
	}
	infoChanged := false
	var rate json.RawMessage
	switch u.Kind {
	case "user_message_chunk":
		if c.importing && !c.replaying {
			c.chunkLocked(KindUser, textOf(u.Content))
		}
	case "agent_message_chunk":
		if !c.replaying {
			c.chunkLocked(KindAgent, textOf(u.Content))
		}
	case "agent_thought_chunk":
		if !c.replaying {
			c.chunkLocked(KindThought, textOf(u.Content))
		}
	case "tool_call", "tool_call_update":
		if !c.replaying {
			c.toolLocked(u)
		}
	case "plan":
		if !c.replaying {
			c.planLocked(u.Entries)
		}
	case "usage_update":
		c.usage, infoChanged = usageOf(u), true
		if !c.replaying {
			rate = c.usage.RateLimit
		}
	case "session_info_update":
		if u.Title != nil && *u.Title != "" {
			c.agentTitled = true
			if *u.Title != c.title {
				c.title, infoChanged = shorten(*u.Title, 120), true
			}
		}
	case "current_mode_update":
		c.mode, infoChanged = u.CurrentModeID, true
	case "config_option_update":
		c.applyConfigLocked(u.ConfigOptions)
		infoChanged = true
	case "available_commands_update":
		cmds := u.Commands
		if len(cmds) > 200 {
			cmds = cmds[:200]
		}
		if !sameCommands(cmds, c.commands) {
			c.commands, infoChanged = append([]Command{}, cmds...), true
		}
	}
	c.mu.Unlock()
	if rate != nil {
		c.learnRate(rate)
	}
	if infoChanged {
		c.publishInfo("chat.changed")
	}
}

func sameCommands(a, b []Command) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func usageOf(u update) *Usage {
	out := &Usage{Used: u.Used, Size: u.Size}
	if u.Cost != nil {
		out.Cost, out.Currency = u.Cost.Amount, u.Cost.Currency
	}
	if raw, ok := u.Meta["_claude/rateLimit"]; ok && len(raw) < 2048 && string(raw) != "null" {
		out.RateLimit = raw
	}
	return out
}

// toolLocked creates or updates the message for a tool call.
func (c *Chat) toolLocked(u update) {
	i, ok := c.tools[u.ToolID]
	if !ok {
		c.openIdx = -1
		i = c.addLocked(Message{Kind: KindTool, ToolID: u.ToolID})
		c.tools[u.ToolID] = i
	}
	m := &c.msgs[i]
	if u.Title != nil {
		m.Title = shorten(*u.Title, 500)
	}
	if u.ToolKind != nil {
		m.ToolKind = *u.ToolKind
	}
	if u.Status != nil {
		m.Status = *u.Status
	} else if m.Status == "" {
		m.Status = "pending"
	}
	if len(u.RawInput) > 0 {
		var t bool
		m.RawInput, t = capBlob(u.RawInput)
		m.Truncated = m.Truncated || t
	}
	if len(u.Locations) > 0 {
		m.Locations, _ = capBlob(u.Locations)
	}
	streamed := c.streamLocked(i, u)
	if len(u.Content) > 0 && string(u.Content) != "null" && !onlyTerminal(u.Content) {
		var t bool
		m.Content, t = capBlob(u.Content)
		m.Truncated = m.Truncated || t
	}
	c.claimPendingLocked(i)
	c.publishToolLocked(i, u.ToolID, streamed && u.Status == nil)
	if u.Status != nil && (*u.Status == "completed" || *u.Status == "failed") {
		c.endStreamLocked(u.ToolID)
	}
}

// planLocked keeps one plan message per turn and replaces it on every update.
func (c *Chat) planLocked(entries json.RawMessage) {
	if c.planIdx >= 0 && c.planIdx < len(c.msgs) {
		c.msgs[c.planIdx].Entries, _ = capBlob(entries)
		c.updateLocked(c.planIdx)
		return
	}
	c.openIdx = -1
	e, _ := capBlob(entries)
	c.planIdx = c.addLocked(Message{Kind: KindPlan, Entries: e})
}
