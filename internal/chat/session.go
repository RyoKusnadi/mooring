package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
)

// Mode ids that mean "ask before acting". Claude's is "default" (shown as Manual).
var askingModes = []string{"default", "ask", "manual"}

type sessionResult struct {
	SessionID string `json:"sessionId"`
	Modes     *struct {
		CurrentModeID  string `json:"currentModeId"`
		AvailableModes []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"availableModes"`
	} `json:"modes"`
	Models *struct {
		CurrentModelID  string `json:"currentModelId"`
		AvailableModels []struct {
			ModelID string `json:"modelId"`
			Name    string `json:"name"`
		} `json:"availableModels"`
	} `json:"models"`
	ConfigOptions []json.RawMessage `json:"configOptions"`
}

// handshake runs initialize, then opens the session: resume or load the saved
// one when the agent can, else a new one (with a notice when a saved one was lost).
func (c *Chat) handshake(ctx context.Context, r *run) error {
	var ir initResult
	err := r.rpc.Call(ctx, "initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{"fs": map[string]bool{"readTextFile": true, "writeTextFile": true}, "terminal": true,
			"_meta": map[string]bool{"terminal_output_delta": true}},
		"clientInfo": map[string]string{"name": "mooring", "title": "Mooring", "version": "0"},
	}, &ir)
	if err != nil {
		return err
	}
	cp := ir.caps()
	c.mu.Lock()
	c.caps = cp
	saved, imported := c.sessionID, c.spec.Imported
	c.mu.Unlock()
	if err := c.load(); err != nil {
		return err
	}
	// A session brought in from the agent's own program: its replay is the history. The mark
	// is saved and cleared only after a load that worked, so a restart or a failed load retries.
	importing := imported && saved != "" && cp.Load
	if importing && c.hasTalk() {
		c.mu.Lock()
		c.noticeLocked("The last try to bring in this session stopped partway. The whole session follows again below.")
		c.mu.Unlock()
	}
	servers, notes := c.mcpServers(ctx, cp)
	if len(notes) > 0 {
		c.mu.Lock()
		for _, n := range notes {
			c.noticeLocked(n)
		}
		c.mu.Unlock()
	}
	params := map[string]any{"cwd": c.dir, "mcpServers": servers}
	if dirs := c.extraDirs(cp); dirs != nil {
		params["additionalDirectories"] = dirs
	}
	if sp := c.systemPrompt(cp); sp != nil {
		params["systemPrompt"] = sp
	}
	var res sessionResult
	opened := false
	if saved != "" {
		params["sessionId"] = saved
		switch {
		case importing:
			c.setImporting(true)
			err = r.rpc.Call(ctx, "session/load", params, &res)
			c.setImporting(false)
			if err == nil {
				c.mu.Lock()
				c.spec.Imported = false // saved with the next info change, when the start ends
				c.mu.Unlock()
			}
		case cp.Resume:
			err = r.rpc.Call(ctx, "session/resume", params, &res)
		case cp.Load:
			c.setReplaying(true)
			err = r.rpc.Call(ctx, "session/load", params, &res)
			c.setReplaying(false)
		default:
			err = errNoResume
		}
		if err != nil && isAuth(err) {
			return err
		}
		if importing && err != nil {
			// Keep the link to the session; Resume tries the load again.
			return fmt.Errorf("could not open the session to bring in: %w", err)
		}
		opened = err == nil
		res.SessionID = saved
		delete(params, "sessionId")
		if !opened {
			c.m.log.Info("chat: saved session not reopened", "chat", c.id, "err", err)
			c.mu.Lock()
			c.noticeLocked("This agent could not reopen the earlier session. The messages above are kept for reading; the agent starts without them.")
			c.mu.Unlock()
			res = sessionResult{}
		}
	}
	if !opened {
		if err := r.rpc.Call(ctx, "session/new", params, &res); err != nil {
			return err
		}
	}
	c.mu.Lock()
	c.sessionID = res.SessionID
	c.mode, c.modes = "", nil // what this run's agent advertises, not what was saved
	c.applySessionLocked(res)
	c.mu.Unlock()
	return c.askFirst(ctx, r)
}

var errNoResume = &rpcError{Code: -32601, Message: "agent cannot resume sessions"}

// setImporting turns on recording of a replay; turning it off closes the last
// replayed message, so the next reply starts a new one.
func (c *Chat) setImporting(v bool) {
	c.mu.Lock()
	c.importing, c.openIdx = v, -1
	c.mu.Unlock()
}

// hasTalk reports whether the chat has a user or agent message.
func (c *Chat) hasTalk() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.msgs {
		if m.Kind == KindUser || m.Kind == KindAgent {
			return true
		}
	}
	return false
}

func (c *Chat) setReplaying(v bool) {
	c.mu.Lock()
	c.replaying = v
	c.mu.Unlock()
}

// applySessionLocked copies modes, models and config options from a session result.
func (c *Chat) applySessionLocked(res sessionResult) {
	if res.Modes != nil {
		c.mode = res.Modes.CurrentModeID
		c.modes = make([]Choice, 0, len(res.Modes.AvailableModes))
		for _, m := range res.Modes.AvailableModes {
			c.modes = append(c.modes, Choice{ID: m.ID, Name: m.Name})
		}
	}
	if res.Models != nil && len(res.Models.AvailableModels) > 0 {
		c.model, c.modelCfg = res.Models.CurrentModelID, ""
		c.models = make([]Choice, 0, len(res.Models.AvailableModels))
		for _, m := range res.Models.AvailableModels {
			c.models = append(c.models, Choice{ID: m.ModelID, Name: m.Name})
		}
	}
	c.applyConfigLocked(res.ConfigOptions)
}

// askFirst switches the session to the mode that asks before it acts: the
// adapter may start in a mode that runs some commands without asking.
func (c *Chat) askFirst(ctx context.Context, r *run) error {
	c.mu.Lock()
	target, sid := "", c.sessionID
	for _, m := range c.modes {
		if slices.Contains(askingModes, m.ID) {
			target = m.ID
			break
		}
	}
	already := c.mode == target
	advertised := len(c.modes) > 0
	c.mu.Unlock()
	if !advertised || already {
		return nil
	}
	if target == "" {
		c.askNotice()
		return nil
	}
	if err := r.rpc.Call(ctx, "session/set_mode", map[string]string{"sessionId": sid, "modeId": target}, nil); err != nil {
		if isAuth(err) {
			return err
		}
		c.m.log.Warn("chat: could not switch to the asking mode", "chat", c.id, "err", err)
		c.askNotice()
		return nil
	}
	c.mu.Lock()
	c.mode = target
	c.mu.Unlock()
	return nil
}

// askNotice tells the user the agent may act without asking.
func (c *Chat) askNotice() {
	name := c.m.catalog[c.spec.Agent.ID].Name
	c.mu.Lock()
	c.noticeLocked(name + " did not switch to asking before it acts, so it may run commands without asking.")
	c.mu.Unlock()
}
