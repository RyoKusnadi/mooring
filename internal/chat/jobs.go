package chat

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/ryokusnadi/mooring/internal/settings"
)

const (
	maxTitleChars = 80
	maxSubject    = 72
	maxBodyLines  = 10
	maxDiffBytes  = 24 << 10
	maxStatBytes  = 4 << 10
	titleInput    = 2 << 10
)

// maybeTitle starts a title job after the first turn of a chat the agent did not
// name. It returns at once; the job runs in the background and a failure only
// shows in the log, so the title stays the first line of the user's message.
func (c *Chat) maybeTitle() {
	if c.m.Settings == nil {
		return
	}
	cfg := c.m.Settings.Get()
	c.mu.Lock()
	agent := c.spec.Agent.ID
	model := cfg.SmallModel[agent]
	if !cfg.TitleJobs || model == "" || c.titleTried || !c.autoTitle || c.agentTitled || c.removed {
		c.mu.Unlock()
		return
	}
	var first, reply string
	users := 0
	for _, m := range c.msgs {
		switch {
		case m.Kind == KindUser && m.Job == "":
			users++
			if users == 1 {
				first = m.Text
			}
		case m.Kind == KindAgent && users == 1:
			reply += m.Text
		}
	}
	if users != 1 {
		c.mu.Unlock()
		return
	}
	c.titleTried = true
	q := jobReq{kind: "title", agent: agent, model: model, projectID: c.projectID, accountID: c.spec.Account.ID,
		prompt: "Write a title for this chat, at most 8 words. Reply with the title only, no quotes.\n\nUser:\n" +
			capText(first, titleInput) + "\n\nAgent:\n" + capText(reply, titleInput)}
	c.mu.Unlock()
	c.m.jobsWG.Add(1)
	go func() {
		defer c.m.jobsWG.Done()
		text, _, err := c.m.runJob(q)
		if err != nil {
			c.m.log.Warn("chat: title job failed", "chat", c.id, "err", err)
			return
		}
		title := cleanTitle(text)
		c.mu.Lock()
		if title == "" || c.agentTitled || c.removed {
			c.mu.Unlock()
			return
		}
		c.title = title
		c.touchLocked()
		c.mu.Unlock()
		c.flush()
		c.publishInfo("chat.changed")
	}()
}

// cleanTitle keeps the first non-empty line, without quotes or markdown marks,
// at most 80 characters.
func cleanTitle(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.Trim(strings.TrimSpace(line), "\"'`*#_ ")
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) > maxTitleChars {
			line = strings.TrimSpace(string([]rune(line)[:maxTitleChars]))
		}
		return line
	}
	return ""
}

// cleanMessage turns a model's answer into a commit message: no code fences, a
// subject of at most 72 characters, a body of at most 10 lines.
func cleanMessage(s string) string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			continue
		}
		lines = append(lines, strings.TrimRight(l, " \t\r"))
	}
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i == len(lines) {
		return ""
	}
	subject := strings.TrimSpace(lines[i])
	if utf8.RuneCountInString(subject) > maxSubject {
		subject = strings.TrimSpace(string([]rune(subject)[:maxSubject]))
	}
	body := lines[i+1:]
	for len(body) > 0 && strings.TrimSpace(body[0]) == "" {
		body = body[1:]
	}
	for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
		body = body[:len(body)-1]
	}
	if len(body) > maxBodyLines {
		body = body[:maxBodyLines]
	}
	if len(body) == 0 {
		return subject
	}
	return subject + "\n\n" + strings.Join(body, "\n")
}

// smallAgent picks the agent that writes commit messages: the default agent when
// it has a small model, else the first installed agent of the catalog that has one.
func (m *Manager) smallAgent(cfg settings.Settings) (agent, model string) {
	if id := cfg.SmallModel[m.DefaultAgent]; id != "" {
		return m.DefaultAgent, id
	}
	for _, a := range m.order {
		if id := cfg.SmallModel[a]; id != "" && m.catalog[a].Info().Installed {
			return a, id
		}
	}
	return "", ""
}

// SuggestMessage writes a commit message for what is staged in a folder of a project.
func (m *Manager) SuggestMessage(ctx context.Context, projectID, folder string) (message string, tokens int64, err error) {
	var cfg settings.Settings
	if m.Settings != nil {
		cfg = m.Settings.Get()
	}
	agent, model := m.smallAgent(cfg)
	if !cfg.CommitJobs || agent == "" || m.Staged == nil {
		return "", 0, apiErr("BAD_REQUEST", "Commit messages from a small model are off. Turn them on and choose a small model in the settings.")
	}
	stat, diff, err := m.Staged(ctx, projectID, folder)
	if err != nil {
		return "", 0, err
	}
	if strings.TrimSpace(stat) == "" {
		return "", 0, apiErr("NOTHING_TO_COMMIT", "Nothing is staged. Stage some changes first.")
	}
	text, tokens, err := m.runJob(jobReq{kind: "commit", agent: agent, model: model, projectID: projectID,
		prompt: "Write a git commit message for the staged changes below. Reply with the message only: a subject line of at most 72 characters, " +
			"then, if needed, a blank line and up to 10 lines of body. No quotes, no code fences.\n\nFiles:\n" +
			capText(stat, maxStatBytes) + "\n\nDiff:\n" + capText(diff, maxDiffBytes)})
	if err != nil {
		return "", 0, err
	}
	message = cleanMessage(text)
	if message == "" {
		return "", tokens, apiErr("AGENT_FAILED", "The model sent no message.")
	}
	return message, tokens, nil
}

// AgentModels lists the models an agent offers, without sending it a prompt.
func (m *Manager) AgentModels(agent, projectID string) (any, error) {
	if _, ok := m.catalog[agent]; !ok {
		return nil, apiErr("NOT_FOUND", "There is no agent called %q.", agent)
	}
	if _, err := m.Dir(projectID); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(m.baseCtx(), m.jobWait())
	defer cancel()
	a, err := m.openJobAgent(ctx, agent, projectID, "")
	if err != nil {
		return nil, err
	}
	defer a.close()
	_, choices := a.models()
	if choices == nil {
		choices = []Choice{}
	}
	return map[string]any{"models": choices}, nil
}
