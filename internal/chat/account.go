package chat

import (
	"encoding/json"

	"github.com/ryokusnadi/mooring/internal/accounts"
)

// The chat's side of accounts (internal/accounts): which account a chat runs on,
// what the chat tells the account about itself, and moving a chat to another one.

// fillAccount settles spec.account for a project. Empty means the project's
// account. lenient lets a chat whose saved account is gone use the project's.
func (m *Manager) fillAccount(spec *Spec, projectID string, lenient bool) error {
	if m.Accounts == nil {
		return nil
	}
	id, err := m.Accounts.Pick(projectID, spec.Agent.ID, spec.Account.ID, lenient)
	if err != nil {
		return err
	}
	spec.Account.ID = id
	return nil
}

// launchEnv is the environment of the account the chat runs on.
func (c *Chat) launchEnv(agent, accountID string) ([]string, error) {
	if c.m.Accounts == nil {
		return nil, nil
	}
	return c.m.Accounts.Env(agent, accountID)
}

func (c *Chat) agentAccount() (agent, id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.spec.Agent.ID, c.spec.Account.ID
}

// The learn methods run without the chat's lock held: accounts may call back into chats.

func (c *Chat) learnRan() {
	if c.m.Accounts != nil {
		c.m.Accounts.Ran(c.agentAccount())
	}
}

func (c *Chat) learnNeedsLogin() {
	if c.m.Accounts != nil {
		c.m.Accounts.NeedsLogin(c.agentAccount())
	}
}

func (c *Chat) learnLimit(resetsAt int64) {
	if c.m.Accounts != nil {
		agent, id := c.agentAccount()
		c.m.Accounts.LimitHit(agent, id, resetsAt)
	}
}

// learnRate passes the agent's rate-limit object on. A refusing window during a turn is kept for the turn's end.
func (c *Chat) learnRate(raw json.RawMessage) {
	if c.m.Accounts == nil {
		return
	}
	agent, id := c.agentAccount()
	rejected, resets := c.m.Accounts.RateLimit(agent, id, raw)
	if !rejected {
		return
	}
	c.mu.Lock()
	if c.state == StateRunning {
		c.turnRejected, c.turnResetsAt = true, resets
	}
	c.mu.Unlock()
}

// isLimitError reports whether an error of the agent says a limit was hit.
func (c *Chat) isLimitError(err error) bool {
	if c.m.Accounts == nil || err == nil || isAuth(err) {
		return false
	}
	return c.m.Accounts.IsLimitText(c.spec.Agent.ID, err.Error())
}

// switchToLocked is the account to offer after a limit, nil otherwise.
func (c *Chat) switchToLocked() *string {
	if !c.limited || c.m.Accounts == nil {
		return nil
	}
	if id := c.m.Accounts.SwitchFor(c.spec.Agent.ID, c.spec.Account.ID); id != "" {
		return &id
	}
	return nil
}

// LiveOn counts the chats with a running agent on an account.
func (m *Manager) LiveOn(accountID string) int {
	m.mu.Lock()
	all := make([]*Chat, 0, len(m.chats))
	for _, c := range m.chats {
		all = append(all, c)
	}
	m.mu.Unlock()
	n := 0
	for _, c := range all {
		c.mu.Lock()
		if c.spec.Account.ID == accountID && (c.state == StateStarting || c.state == StateIdle || c.state == StateRunning || c.state == StateNeedsLogin) {
			n++
		}
		c.mu.Unlock()
	}
	return n
}

var _ accounts.Chats = (*Manager)(nil)

// MoveAccount points the project's chats of an agent at an account. Open chats
// (starting, idle, needing a login) are stopped and resumed on it; one that is
// answering makes the whole move fail with BUSY, before anything changes. Each
// chat is checked and marked as moving under its own lock, so a prompt cannot
// slip in between the check and the stop.
func (m *Manager) MoveAccount(projectID, agent, accountID string) ([]string, error) {
	var marked []*Chat
	defer func() {
		for _, c := range marked {
			c.mu.Lock()
			c.moving = false
			c.mu.Unlock()
		}
	}()
	restarted := []string{}
	for pass := 0; pass < 2; pass++ { // a second look catches a chat made while the first was moving
		fresh, err := m.markMoving(projectID, agent, accountID)
		marked = append(marked, fresh...)
		if err != nil {
			return nil, err
		}
		for _, c := range fresh {
			c.mu.Lock()
			open := c.state == StateIdle || c.state == StateNeedsLogin || c.state == StateStarting
			c.mu.Unlock()
			if open {
				c.stopAgent(nil, StateStopped, nil)
			}
			c.mu.Lock()
			c.spec.Account.ID, c.limited = accountID, false
			c.mu.Unlock()
			c.publishInfo("chat.changed")
			if !open {
				continue
			}
			if err := m.claimAs(c, true); err != nil {
				m.log.Warn("chat: could not resume on the new account", "chat", c.id, "err", err)
				continue
			}
			restarted = append(restarted, c.id)
		}
	}
	return restarted, nil
}

// markMoving sets the moving flag on the project's chats of the agent that are not
// on the account yet. If one is answering, the flags it set are cleared and BUSY is returned.
func (m *Manager) markMoving(projectID, agent, accountID string) ([]*Chat, error) {
	m.mu.Lock()
	var mine []*Chat
	for _, c := range m.chats {
		if c.projectID == projectID && c.spec.Agent.ID == agent {
			mine = append(mine, c)
		}
	}
	m.mu.Unlock()
	var got []*Chat
	for _, c := range mine {
		c.mu.Lock()
		if c.spec.Account.ID == accountID {
			c.mu.Unlock()
			continue
		}
		if c.state == StateRunning || c.moving {
			c.mu.Unlock()
			for _, d := range got {
				d.mu.Lock()
				d.moving = false
				d.mu.Unlock()
			}
			return nil, apiErr("BUSY", "A chat in this project is answering. Wait for it or cancel it first.")
		}
		c.moving = true
		c.mu.Unlock()
		got = append(got, c)
	}
	return got, nil
}
