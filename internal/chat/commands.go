package chat

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ryokusnadi/mooring/internal/events"
)

// Handles reports whether name is a chat or agent command.
func Handles(name string) bool {
	return strings.HasPrefix(name, "chat.") || name == "agent.list" || name == "agent.models" || name == "git.suggestMessage"
}

type args struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Agent     string `json:"agent"`
	Title     string `json:"title"`
	Spec      *Spec  `json:"spec"`
	Text      string `json:"text"`
	RequestID string `json:"requestId"`
	OptionID  string `json:"optionId"`
	Value     string `json:"value"`

	Filters     []string     `json:"filters"`
	Attachments []Attachment `json:"attachments"`
	FromChat    string       `json:"fromChat"`
	From        string       `json:"from"`   // chat.relay: the chat with the reply
	MsgIdx      int          `json:"msgIdx"` // chat.relay: the reply's message id
	Context     string       `json:"context"`
	WithChanges bool         `json:"withChanges"`
	Folder      string       `json:"folder"`
	SessionID   string       `json:"sessionId"` // chat.import: the agent's own session
}

// Run runs a chat command. Errors are *ledger.Error.
func (m *Manager) Run(cmd events.Command) (any, error) {
	var a args
	if err := json.Unmarshal(cmd.Args, &a); err != nil {
		return nil, apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	switch cmd.Name {
	case "agent.list":
		return map[string]any{"agents": m.Agents()}, nil
	case "agent.models":
		return m.AgentModels(a.Agent, a.ProjectID)
	case "git.suggestMessage":
		msg, tokens, err := m.SuggestMessage(context.Background(), a.ProjectID, a.Folder)
		if err != nil {
			return nil, err
		}
		return map[string]any{"message": msg, "tokens": tokens}, nil
	case "chat.start", "chat.import", "chat.agentSessions":
		return m.runStart(cmd.Name, a)
	case "chat.remove":
		return m.remove(a.ID)
	}
	c, err := m.must(a.ID)
	if err != nil {
		return nil, err
	}
	switch cmd.Name {
	case "chat.resume":
		return m.resume(c)
	case "chat.close":
		c.closeSession()
		c.stopAgent(nil, StateStopped, nil)
		return map[string]any{}, nil
	case "chat.prompt":
		return map[string]any{}, c.Prompt(a.Text, a.Attachments)
	case "chat.setFilters":
		return c.SetFilters(a.Filters)
	case "chat.summarize":
		text, source, err := c.Summarize()
		if err != nil {
			return nil, err
		}
		return map[string]string{"text": text, "source": source}, nil
	case "chat.rename":
		return map[string]any{}, c.Rename(a.Title)
	case "chat.relay":
		return map[string]any{}, c.Relay(a.From, a.MsgIdx, a.Text)
	case "chat.seen":
		c.Seen()
		return map[string]any{}, nil
	case "chat.cancel":
		c.Cancel()
		return map[string]any{}, nil
	case "chat.permission":
		ok, chosen, err := c.Answer(a.RequestID, a.OptionID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"accepted": ok, "chosen": chosen}, nil
	case "chat.setModel":
		return map[string]any{}, c.SetModel(a.Value)
	case "chat.setMode":
		return map[string]any{}, c.SetMode(a.Value)
	}
	return nil, apiErr("BAD_REQUEST", "Unknown command %q.", cmd.Name)
}

func (m *Manager) start(a args) (any, error) {
	if !idPattern.MatchString(a.ID) {
		return nil, apiErr("BAD_REQUEST", "A chat id needs 1 to 64 letters, digits, - or _.")
	}
	var spec Spec
	if a.Spec != nil {
		spec = *a.Spec
	}
	if a.Agent != "" {
		spec.Agent.ID = a.Agent
	}
	changes, err := m.checkContext(a)
	if err != nil {
		return nil, err
	}
	if spec.Filters == nil && m.compactDefault() {
		spec.Filters = []string{filterCompact}
	}
	if spec.Folder.Worktree && spec.Folder.Path != "" {
		return nil, apiErr("BAD_REQUEST", "Choose either a working folder or a new worktree, not both.")
	}
	if m.get(a.ID) != nil {
		return nil, apiErr("ALREADY_EXISTS", "That chat already exists.")
	}
	projectDir, err := m.Dir(a.ProjectID)
	if err != nil {
		return nil, err
	}
	dir, err := spec.normalize(m.catalog, m.DefaultAgent, projectDir)
	if err != nil {
		return nil, err
	}
	if err := m.fillAccount(&spec, a.ProjectID, false); err != nil {
		return nil, err
	}
	for _, t := range spec.ToolServers {
		if t.ID == "" {
			return nil, apiErr("BAD_REQUEST", "Choose tool servers from the saved list.")
		}
		if m.Tools == nil {
			return nil, apiErr("NOT_FOUND", "That tool server is not saved.")
		}
		if err := m.Tools.Usable(t.ID); err != nil {
			return nil, err
		}
	}
	def := m.catalog[spec.Agent.ID]
	if in := def.Info(); !in.Installed {
		return nil, apiErr("AGENT_NOT_INSTALLED", "%s needs %q, which was not found on this machine.", def.Name, in.Missing)
	}
	drop := func() {}
	if spec.Folder.Worktree {
		// Checked first as well, so a full project does not get a worktree it cannot use.
		m.mu.Lock()
		err := m.liveLimit(a.ProjectID, nil)
		m.mu.Unlock()
		if err != nil {
			return nil, err
		}
		if dir, drop, err = m.makeWorktree(a.ProjectID, projectDir, &spec); err != nil {
			return nil, err
		}
	}
	info, err := m.create(a, projectDir, dir, spec)
	if err != nil {
		drop()
		return nil, err
	}
	if a.Context != "" {
		m.get(a.ID).addContext(a.Context, a.FromChat, changes)
		info = m.get(a.ID).Info()
	}
	go m.get(a.ID).boot()
	return info, nil
}

// create adds the chat and its saved row; the caller starts it.
func (m *Manager) create(a args, projectDir, dir string, spec Spec) (Info, error) {
	c, err := newChat(m, a.ID, a.ProjectID, projectDir, dir, spec)
	if err != nil {
		return Info{}, apiErr("INTERNAL", "The chat could not be created.")
	}
	c.title, c.fromChat = shorten(a.Title, 120), a.FromChat
	c.sessionID, c.spec.Imported = a.SessionID, a.SessionID != "" // only chat.import sets both
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Info{}, apiErr("INTERNAL", "The server is shutting down.")
	}
	if _, ok := m.chats[a.ID]; ok {
		m.mu.Unlock()
		return Info{}, apiErr("ALREADY_EXISTS", "That chat already exists.")
	}
	if err := m.liveLimit(a.ProjectID, nil); err != nil {
		m.mu.Unlock()
		return Info{}, err
	}
	c.state = StateStarting
	if err := m.insertRow(c, c.infoLocked()); err != nil {
		m.mu.Unlock()
		m.log.Error("chat: create failed", "err", err)
		return Info{}, apiErr("INTERNAL", "The chat could not be saved.")
	}
	m.chats[a.ID] = c
	m.mu.Unlock()
	info := c.Info()
	m.pub.Publish("chat.added", info)
	return info, nil
}

// resume restarts a stopped chat's agent and continues its conversation.
func (m *Manager) resume(c *Chat) (any, error) {
	if err := c.load(); err != nil {
		return nil, apiErr("INTERNAL", "The saved chat could not be read.")
	}
	if err := m.claim(c); err != nil {
		return nil, err
	}
	return c.Info(), nil
}

// claim moves a stopped chat to starting and boots its agent. A chat that is
// not stopped is left alone.
func (m *Manager) claim(c *Chat) error { return m.claimAs(c, false) }

// claimAs is claim; mover is true for MoveAccount, which holds the chat's moving flag.
func (m *Manager) claimAs(c *Chat, mover bool) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return apiErr("INTERNAL", "The server is shutting down.")
	}
	c.mu.Lock()
	state, moving := c.state, c.moving
	c.mu.Unlock()
	if moving && !mover {
		m.mu.Unlock()
		return apiErr("BUSY", "The chat is moving to another account. Try again in a moment.")
	}
	if state != StateStopped && state != StateNeedsLogin {
		m.mu.Unlock()
		return nil
	}
	fail := func(err error) error { m.mu.Unlock(); return err }
	if err := m.liveLimit(c.projectID, c); err != nil {
		return fail(err)
	}
	projectDir, err := m.Dir(c.projectID)
	if err != nil {
		return fail(err)
	}
	// Under c.mu: a turn that is still ending reads the spec for its last publish.
	c.mu.Lock()
	dir, err := c.spec.normalize(m.catalog, m.DefaultAgent, projectDir)
	def := m.catalog[c.spec.Agent.ID]
	c.mu.Unlock()
	if err != nil {
		return fail(err)
	}
	if in := def.Info(); !in.Installed {
		return fail(apiErr("AGENT_NOT_INSTALLED", "%s needs %q, which was not found on this machine.", def.Name, in.Missing))
	}
	c.mu.Lock()
	if err := m.fillAccount(&c.spec, c.projectID, true); err != nil {
		c.mu.Unlock()
		return fail(err)
	}
	c.projectDir, c.dir = projectDir, dir
	c.limited = false
	c.state, c.errInfo = StateStarting, nil
	c.touchLocked()
	c.mu.Unlock()
	m.mu.Unlock()
	c.publishInfo("chat.changed")
	go c.boot()
	return nil
}

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
	m.pub.Publish("chat.removed", map[string]string{"id": id, "projectId": c.projectID})
	return map[string]any{}, nil
}
