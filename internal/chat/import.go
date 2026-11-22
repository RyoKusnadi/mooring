package chat

import (
	"context"
	"path/filepath"
	"strings"
	"unicode"
)

// AgentSession is a conversation the agent keeps in its own store, made in the
// project folder (for example in the agent's own command line program).
type AgentSession struct {
	SessionID string `json:"sessionId"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"` // as the agent sent it, ISO 8601 or ""
}

const (
	maxListPages    = 5
	maxListSessions = 200
)

type listResult struct {
	Sessions []struct {
		SessionID string  `json:"sessionId"`
		Cwd       string  `json:"cwd"`
		Title     *string `json:"title"`
		UpdatedAt *string `json:"updatedAt"`
	} `json:"sessions"`
	NextCursor *string `json:"nextCursor"`
}

// AgentSessions asks the agent for its sessions made in the project folder,
// leaving out those a Mooring chat already uses.
func (m *Manager) AgentSessions(agent, projectID string) ([]AgentSession, error) {
	projectDir, err := m.Dir(projectID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(m.baseCtx(), m.jobWait())
	defer cancel()
	a, _, err := m.startJobAgent(ctx, agent, projectID, "")
	if err != nil {
		return nil, err
	}
	defer a.close()
	if !a.caps.List || !a.caps.Load {
		return nil, apiErr("NOT_SUPPORTED", "%s cannot list and reopen its own sessions, so there is nothing to bring in.", a.name)
	}
	known := m.sessionsInUse(agent)
	want := sameDir(projectDir)
	out := []AgentSession{}
	cursor := ""
	for page := 0; page < maxListPages && len(out) < maxListSessions; page++ {
		params := map[string]any{"cwd": projectDir}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var res listResult
		if err := a.rpc.Call(ctx, "session/list", params, &res); err != nil {
			return nil, jobError(ctx, err)
		}
		for _, s := range res.Sessions {
			if !validSessionID(s.SessionID) || known[s.SessionID] || !want(s.Cwd) {
				continue
			}
			as := AgentSession{SessionID: s.SessionID}
			if s.Title != nil {
				as.Title = shorten(strings.TrimSpace(strings.Map(noControl, *s.Title)), 120)
			}
			if s.UpdatedAt != nil && len(*s.UpdatedAt) <= 40 && !strings.ContainsFunc(*s.UpdatedAt, unicode.IsControl) {
				as.UpdatedAt = *s.UpdatedAt
			}
			known[s.SessionID] = true
			if out = append(out, as); len(out) == maxListSessions {
				break
			}
		}
		if res.NextCursor == nil || *res.NextCursor == "" {
			break
		}
		cursor = *res.NextCursor
	}
	return out, nil
}

// runStart runs chat.start, chat.import and chat.agentSessions (the list). Only
// chat.import may name an existing session.
func (m *Manager) runStart(name string, a args) (any, error) {
	switch name {
	case "chat.start":
		a.SessionID = ""
		return m.start(a)
	case "chat.import":
		return m.importChat(a)
	}
	list, err := m.AgentSessions(a.Agent, a.ProjectID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"sessions": list}, nil
}

// sessionsInUse is the set of the agent's session ids that chats hold.
func (m *Manager) sessionsInUse(agent string) map[string]bool {
	m.mu.Lock()
	list := make([]*Chat, 0, len(m.chats))
	for _, c := range m.chats {
		list = append(list, c)
	}
	m.mu.Unlock()
	out := map[string]bool{}
	for _, c := range list {
		c.mu.Lock()
		if c.spec.Agent.ID == agent && c.sessionID != "" {
			out[c.sessionID] = true
		}
		c.mu.Unlock()
	}
	return out
}

// sameDir reports whether a folder the agent names is dir, following links.
func sameDir(dir string) func(string) bool {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		real = dir
	}
	return func(p string) bool {
		if !filepath.IsAbs(p) {
			return false
		}
		if p == dir || p == real {
			return true
		}
		r, err := filepath.EvalSymlinks(p)
		return err == nil && r == real
	}
}

// noControl turns control characters (new lines, escapes) into spaces.
func noControl(r rune) rune {
	if unicode.IsControl(r) {
		return ' '
	}
	return r
}

func validSessionID(id string) bool {
	if id == "" || len(id) > 256 {
		return false
	}
	return !strings.ContainsFunc(id, unicode.IsControl)
}

// importChat makes a chat in the project folder on an existing session of the
// agent. Its first start loads the session and keeps the messages the agent replays.
func (m *Manager) importChat(a args) (any, error) {
	if !validSessionID(a.SessionID) {
		return nil, apiErr("BAD_REQUEST", "Pick a session from the list.")
	}
	if a.Agent == "" {
		return nil, apiErr("BAD_REQUEST", "Name the agent the session belongs to.")
	}
	if m.sessionsInUse(a.Agent)[a.SessionID] {
		return nil, apiErr("ALREADY_EXISTS", "A chat in Mooring already uses that session.")
	}
	spec := Spec{Imported: true, AgentMade: true}
	if a.Spec != nil {
		spec.Account = a.Spec.Account
	}
	a.Spec, a.Context, a.FromChat, a.WithChanges = &spec, "", "", false
	return m.start(a)
}
