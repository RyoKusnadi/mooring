package chat

import "context"

// closeSession tells a running agent that the chat is closing, when it can, so it
// lets go of the session before it is stopped. A failure is only logged.
func (c *Chat) closeSession() {
	c.mu.Lock()
	r, sid, can := c.cur, c.sessionID, c.caps.Close
	c.mu.Unlock()
	if r == nil || sid == "" || !can || !r.p.running() {
		return
	}
	ctx, cancel := context.WithTimeout(c.m.baseCtx(), jobClose)
	defer cancel()
	if err := r.rpc.Call(ctx, "session/close", map[string]string{"sessionId": sid}, nil); err != nil {
		c.m.log.Warn("chat: session not closed", "chat", c.id, "err", err)
	}
}

// forgetSession asks the agent to delete its own copy of a removed chat's session,
// when it can and Mooring made the session. It runs in the background on the chat's
// account; a failure is only logged.
func (m *Manager) forgetSession(c *Chat) {
	m.dropCheckpoints(c)
	c.mu.Lock()
	sid, agent, account := c.sessionID, c.spec.Agent.ID, c.spec.Account.ID
	can := c.caps.Delete && !c.spec.AgentMade
	c.mu.Unlock()
	m.mu.Lock()
	closed := m.closed
	if sid != "" && can && !closed {
		m.jobsWG.Add(1)
	}
	m.mu.Unlock()
	if sid == "" || !can || closed {
		return
	}
	go func() {
		defer m.jobsWG.Done()
		ctx, cancel := context.WithTimeout(m.baseCtx(), m.jobWait())
		defer cancel()
		a, _, err := m.startJobAgent(ctx, agent, c.projectID, account)
		if err != nil {
			m.log.Warn("chat: agent session not deleted", "chat", c.id, "err", err)
			return
		}
		defer a.close()
		if !a.caps.Delete {
			return
		}
		if err := a.rpc.Call(ctx, "session/delete", map[string]string{"sessionId": sid}, nil); err != nil {
			m.log.Warn("chat: agent session not deleted", "chat", c.id, "err", err)
		}
	}()
}
