package chat

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > 60 {
		s = string([]rune(s)[:60]) + "..."
	}
	return s
}

func (c *Chat) runTurn(ctx context.Context, r *run, turn int, sid string, blocks []any, sent, finished chan struct{}) {
	defer close(finished)
	var res struct {
		StopReason string          `json:"stopReason"`
		Usage      json.RawMessage `json:"usage"`
	}
	var once sync.Once
	markSent := func() { once.Do(func() { close(sent) }) }
	err := r.rpc.CallThen(ctx, "session/prompt", map[string]any{
		"sessionId": sid, "prompt": blocks,
	}, &res, markSent)
	markSent()
	c.mu.Lock()
	if c.cur != r || c.turn != turn {
		c.mu.Unlock()
		return // the chat was stopped or moved on; stopAgent already did the cleanup
	}
	c.openIdx = -1
	c.turnCancel, c.cancelling = nil, false
	c.cancelPermsLocked()
	limited := ctx.Err() == nil && err != nil && (c.turnRejected || c.isLimitError(err))
	resets := c.turnResetsAt
	switch {
	case limited:
		c.limited = true
		name := c.m.catalog[c.spec.Agent.ID].Name
		c.addLocked(Message{Kind: KindError, Code: "LIMIT_REACHED", ResetsAt: resets,
			Text: "This turn stopped: the usage limit of this " + name + " account was reached."})
	case err == nil:
		c.noteUsageLocked(turn, res.Usage)
		if res.StopReason == "cancelled" {
			c.noticeLocked("You stopped this reply.")
		}
		if res.StopReason == "refusal" || res.StopReason == "max_tokens" || res.StopReason == "max_turn_requests" {
			c.noticeLocked(stopNotice(res.StopReason))
		}
	case isAuth(err):
		c.state, c.errInfo = StateNeedsLogin, c.loginError()
	case ctx.Err() != nil:
		// cancelled by us and the agent answered with an error: treat as cancelled
		c.noticeLocked("You stopped this reply.")
	default:
		c.m.log.Warn("chat: prompt failed", "chat", c.id, "err", err)
		e := &ErrInfo{Code: "AGENT_FAILED", Message: "The agent could not finish: " + shorten(err.Error(), 200)}
		c.addLocked(Message{Kind: KindError, Code: e.Code, Text: e.Message})
	}
	// Idle comes last: a reader that sees idle also sees the limit or the failure of this turn.
	if c.state == StateRunning {
		c.state = StateIdle
	}
	c.mu.Unlock()
	switch {
	case limited:
		c.learnLimit(resets)
	case err == nil:
		c.learnRan()
		c.maybeTitle()
	case isAuth(err):
		c.learnNeedsLogin()
	}
	c.flush()
	c.publishInfo("chat.changed")
	if err != nil && !isAuth(err) && ctx.Err() == nil && !r.p.running() {
		c.stopAgent(r, StateStopped, &ErrInfo{Code: "AGENT_FAILED", Message: "The agent stopped. Resume the chat to start it again."})
	}
}

func stopNotice(reason string) string {
	switch reason {
	case "refusal":
		return "The agent declined to continue."
	case "max_tokens":
		return "The reply stopped because the agent hit its length limit."
	}
	return "The agent stopped after its limit of steps."
}

// Cancel stops the running turn: open permission requests are answered
// cancelled, then the agent is told to cancel. If it does not finish in time the
// agent is stopped and the chat can be resumed.
func (c *Chat) Cancel() {
	c.mu.Lock()
	if c.state != StateRunning || c.cur == nil {
		c.mu.Unlock()
		return
	}
	r, sid, turn, sent := c.cur, c.sessionID, c.turn, c.promptSent
	c.cancelling = true
	c.cancelPermsLocked()
	c.mu.Unlock()
	r.terms.killAll()
	c.publishInfo("chat.changed")
	select {
	case <-sent:
	case <-time.After(5 * time.Second):
	}
	_ = r.rpc.Notify("session/cancel", map[string]string{"sessionId": sid})
	time.AfterFunc(c.m.CancelWait, func() {
		c.mu.Lock()
		stuck := c.cur == r && c.turn == turn && c.state == StateRunning
		c.mu.Unlock()
		if stuck {
			c.m.log.Warn("chat: agent ignored cancel", "chat", c.id)
			c.stopAgent(r, StateStopped, &ErrInfo{Code: "AGENT_FAILED", Message: "The agent did not stop when asked, so it was stopped. Resume the chat to continue."})
		}
	})
}

// SetModel switches the model through the agent's model option (or set_model).
func (c *Chat) SetModel(value string) error {
	c.mu.Lock()
	r, sid, cfg := c.cur, c.sessionID, c.modelCfg
	known := false
	for _, m := range c.models {
		known = known || m.ID == value
	}
	state := c.state
	c.mu.Unlock()
	if r == nil || (state != StateIdle && state != StateRunning) {
		return apiErr("NOT_RUNNING", "The agent is not running. Resume the chat first.")
	}
	if !known {
		return apiErr("BAD_REQUEST", "That model is not offered by this agent.")
	}
	ctx, cancel := context.WithTimeout(c.baseContext(), 30*time.Second)
	defer cancel()
	var err error
	if cfg != "" {
		var res sessionResult
		err = r.rpc.Call(ctx, "session/set_config_option", map[string]string{"sessionId": sid, "configId": cfg, "value": value}, &res)
		c.mu.Lock()
		if err == nil && len(res.ConfigOptions) > 0 {
			c.applyConfigLocked(res.ConfigOptions)
		}
		c.mu.Unlock()
	} else {
		err = r.rpc.Call(ctx, "session/set_model", map[string]string{"sessionId": sid, "modelId": value}, nil)
	}
	if err != nil {
		return apiErr("AGENT_FAILED", "The agent did not accept that model: %s", shorten(err.Error(), 200))
	}
	c.mu.Lock()
	c.model = value
	c.mu.Unlock()
	c.publishInfo("chat.changed")
	return nil
}

// SetMode switches the agent's mode (for example from asking to accepting edits).
func (c *Chat) SetMode(mode string) error {
	c.mu.Lock()
	r, sid, state := c.cur, c.sessionID, c.state
	known := false
	for _, m := range c.modes {
		known = known || m.ID == mode
	}
	c.mu.Unlock()
	if r == nil || (state != StateIdle && state != StateRunning) {
		return apiErr("NOT_RUNNING", "The agent is not running. Resume the chat first.")
	}
	if !known {
		return apiErr("BAD_REQUEST", "That mode is not offered by this agent.")
	}
	ctx, cancel := context.WithTimeout(c.baseContext(), 30*time.Second)
	defer cancel()
	if err := r.rpc.Call(ctx, "session/set_mode", map[string]string{"sessionId": sid, "modeId": mode}, nil); err != nil {
		return apiErr("AGENT_FAILED", "The agent did not accept that mode: %s", shorten(err.Error(), 200))
	}
	c.mu.Lock()
	c.mode = mode
	c.mu.Unlock()
	c.publishInfo("chat.changed")
	return nil
}
