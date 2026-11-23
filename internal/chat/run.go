package chat

import (
	"context"
	"errors"
	"strings"
	"time"
)

// run is one started agent process and its connection. A chat has at most one
// current run; late messages from an older run are ignored.
type run struct {
	c     *Chat
	p     *proc
	rpc   *rpcConn
	terms *termHost // commands the agent asked Mooring to run
	env   []string  // what the agent was launched with on top of Mooring's own environment
}

func (c *Chat) boot() {
	c.mu.Lock()
	spec, dir, gen := c.spec, c.dir, c.bootGen
	c.mu.Unlock()
	def := c.m.catalog[spec.Agent.ID]
	env, err := c.launchEnv(spec.Agent.ID, spec.Account.ID)
	if err != nil {
		c.m.log.Error("chat: account folder", "chat", c.id, "err", err)
		c.stopAgent(nil, StateStopped, &ErrInfo{Code: "AGENT_FAILED", Message: "The account of this chat could not be used. Choose another account and resume the chat."})
		return
	}
	l := spec.launchFor(def, dir, env)
	p, err := startProc(l)
	if err != nil {
		c.m.log.Error("chat: start agent", "chat", c.id, "agent", def.ID, "err", err)
		c.stopAgent(nil, StateStopped, &ErrInfo{Code: "AGENT_FAILED", Message: "The agent could not be started: " + startCause(err) + "."})
		return
	}
	r := &run{c: c, p: p, terms: newTermHost(), env: l.env}
	r.rpc = newRPC(p.out, p.in, r)
	c.mu.Lock()
	if c.state != StateStarting || c.removed || c.bootGen != gen {
		c.mu.Unlock()
		p.stop(c.m.KillGrace)
		return
	}
	c.cur = r
	c.tools, c.planIdx, c.openIdx = map[string]int{}, -1, -1
	c.mu.Unlock()
	go func() {
		select {
		case <-p.exited:
		case <-r.rpc.Done():
		}
		c.mu.Lock()
		current := c.cur == r
		c.mu.Unlock()
		if current {
			c.m.log.Warn("chat: agent exited", "chat", c.id)
			msg := "The agent stopped. Resume the chat to start it again."
			select {
			case <-p.exited:
				msg = "The agent stopped (" + p.why() + "). Resume the chat to start it again."
			case <-time.After(300 * time.Millisecond):
			}
			c.stopAgent(r, StateStopped, &ErrInfo{Code: "AGENT_FAILED", Message: msg})
		}
	}()
	ctx, cancel := context.WithTimeout(c.baseContext(), startTimeout)
	defer cancel()
	if err := c.handshake(ctx, r); err != nil {
		c.failBoot(r, err)
		return
	}
	c.mu.Lock()
	if c.cur == r && c.state == StateStarting {
		c.state = StateIdle
		c.touchLocked()
	}
	c.mu.Unlock()
	c.publishInfo("chat.changed")
}

func (c *Chat) baseContext() context.Context {
	if c.m.Context != nil {
		return c.m.Context
	}
	return context.Background()
}

func (c *Chat) failBoot(r *run, err error) {
	c.mu.Lock()
	current := c.cur == r
	c.mu.Unlock()
	if !current {
		return // stopped on purpose or already handled
	}
	if isAuth(err) {
		c.stopAgent(r, StateNeedsLogin, c.loginError())
		c.learnNeedsLogin()
		return
	}
	c.m.log.Error("chat: agent start failed", "chat", c.id, "err", err)
	msg := "The agent did not start: " + shorten(err.Error(), 200)
	if errors.Is(err, context.DeadlineExceeded) {
		msg = "The agent did not answer in time."
	}
	// An agent that died while starting says why on its way out.
	select {
	case <-r.p.exited:
		msg = "The agent did not start: " + r.p.why() + "."
	case <-time.After(300 * time.Millisecond):
	}
	c.stopAgent(r, StateStopped, &ErrInfo{Code: "AGENT_FAILED", Message: msg})
}

func (c *Chat) loginError() *ErrInfo {
	name := c.m.catalog[c.spec.Agent.ID].Name
	return &ErrInfo{Code: "AUTH_REQUIRED", Message: name + " is not signed in. Sign in with its own program on the machine running Mooring, then resume the chat."}
}

// isAuth reports a missing login: JSON-RPC error -32000.
func isAuth(err error) bool {
	var re *rpcError
	return errors.As(err, &re) && re.Code == -32000
}

func shorten(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "..."
	}
	return s
}

// stopAgent ends the current run (killing its process group), cancels open
// permission requests and moves the chat to state. A nil errInfo clears the error.
// A non-nil only stops that run and does nothing when a newer run has replaced it;
// a nil only stops whichever run is current.
func (c *Chat) stopAgent(only *run, state string, errInfo *ErrInfo) {
	c.mu.Lock()
	if only != nil && c.cur != only {
		c.mu.Unlock()
		return
	}
	r := c.cur
	c.cur = nil
	c.bootGen++
	if c.turnCancel != nil {
		c.turnCancel()
		c.turnCancel = nil
	}
	c.cancelPermsLocked()
	c.dropStreamsLocked()
	c.openIdx = -1
	prev := c.state
	c.state, c.errInfo, c.cancelling = state, errInfo, false
	if errInfo != nil && prev != StateStopped && state != StateNeedsLogin {
		c.addLocked(Message{Kind: KindError, Code: errInfo.Code, Text: errInfo.Message})
	}
	c.touchLocked()
	c.mu.Unlock()
	if r != nil {
		r.terms.closeAll()
		r.p.stop(c.m.KillGrace)
	}
	c.flush()
	c.publishInfo("chat.changed")
}

// discard stops the agent of a chat that is being deleted.
func (c *Chat) discard() {
	c.mu.Lock()
	c.removed = true
	c.mu.Unlock()
	c.stopAgent(nil, StateStopped, nil)
	c.m.removeImages(c.id)
}
