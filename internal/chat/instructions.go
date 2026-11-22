package chat

import "strings"

// instructionsText is what Mooring tells every agent it starts, when the setting
// is on. The owner reviews this wording; change it here only.
const instructionsText = "You are running inside Mooring, a browser workspace. When output shortening is on for this chat, " +
	"Mooring shortens long command output before you read it: it keeps the first and last lines and the lines that look like errors, " +
	"and ends with a note such as \"[mooring: 120 of 400 lines cut; ask the user for the full output]\". " +
	"When you see that note and need the part that was cut, run a narrower command (grep, head or tail, " +
	"or write the output to a file and read part of it) instead of running the same command again."

// instructionsFor builds the text a new chat of a project is frozen with: the fixed
// text, then the project's extra text. "" when the setting is off or there are no settings.
func (m *Manager) instructionsFor(projectID string) string {
	if m.Settings == nil {
		return ""
	}
	cfg := m.Settings.Get()
	if !cfg.AgentInstructions {
		return ""
	}
	text := instructionsText
	if extra := strings.TrimSpace(cfg.ProjectInstructions[projectID]); extra != "" {
		text += "\n\n" + extra
	}
	return text
}

// systemPrompt is the session parameter for the chat's frozen text, or nil when the
// chat has none or the agent does not advertise it. Only text blocks are sent. When the
// chat has text the agent cannot take, the chat says so once.
func (c *Chat) systemPrompt(cp caps) map[string]any {
	c.mu.Lock()
	text := c.instructions
	c.instructed = false
	c.mu.Unlock()
	if text == "" {
		return nil
	}
	if !cp.SystemPrompt {
		c.instructionsNotice()
		return nil
	}
	c.mu.Lock()
	c.instructed = true
	c.mu.Unlock()
	return map[string]any{"mode": "append", "content": []map[string]string{{"type": "text", "text": text}}}
}

// instructionsNotice adds the "starts without them" notice unless the chat has it already.
func (c *Chat) instructionsNotice() {
	note := c.m.catalog[c.spec.Agent.ID].Name + " does not take extra instructions, so this chat starts without them."
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.msgs {
		if m.Kind == KindNotice && m.Text == note {
			return
		}
	}
	c.noticeLocked(note)
}
