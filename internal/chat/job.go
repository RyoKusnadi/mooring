package chat

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// A small-model job runs the agent itself on the chat's account, in an empty folder
// (<data>/jobs): no tool servers, no files, no commands, every permission refused.

const (
	jobTimeout = 60 * time.Second
	jobClose   = 5 * time.Second
)

// jobHandler collects the agent's reply and refuses everything the agent asks for.
type jobHandler struct {
	mu   sync.Mutex
	text strings.Builder
}

func (h *jobHandler) notification(method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	var p updateParams
	var u update
	if json.Unmarshal(params, &p) != nil || json.Unmarshal(p.Update, &u) != nil || u.Kind != "agent_message_chunk" {
		return
	}
	h.mu.Lock()
	h.text.WriteString(textOf(u.Content))
	h.mu.Unlock()
}

func (h *jobHandler) request(_ context.Context, method string, params json.RawMessage) (any, error) {
	if method != "session/request_permission" {
		return nil, &rpcError{Code: -32601, Message: "Method not found: " + method}
	}
	var p permParams
	if json.Unmarshal(params, &p) == nil {
		for _, o := range p.Options {
			if o.Kind == "reject_once" || o.Kind == "reject_always" {
				return map[string]any{"outcome": map[string]string{"outcome": "selected", "optionId": o.ID}}, nil
			}
		}
	}
	return cancelledOutcome(), nil
}

func (h *jobHandler) reply() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.TrimSpace(h.text.String())
}

// jobAgent is a started agent with one session in the jobs folder.
type jobAgent struct {
	m       *Manager
	p       *proc
	rpc     *rpcConn
	h       *jobHandler
	sid     string
	session sessionResult
	name    string // the agent's name in messages
	closeBy string // "session/delete", "session/close" or "" when the agent offers neither
	caps    caps
}

// jobsDir is the empty folder jobs run in.
func (m *Manager) jobsDir() (string, error) {
	if m.DataDir == "" {
		return "", apiErr("INTERNAL", "This server has no folder for small jobs.")
	}
	dir := filepath.Join(m.DataDir, "jobs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		m.log.Error("chat: jobs folder", "err", err)
		return "", apiErr("INTERNAL", "The folder for small jobs could not be made.")
	}
	return dir, nil
}

// jobError turns a failure of the agent into the error the browser gets.
func jobError(ctx context.Context, err error) error {
	switch {
	case isAuth(err):
		return apiErr("AUTH_REQUIRED", "The agent is not signed in. Sign in with its own program, then try again.")
	case errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
		return apiErr("TIMEOUT", "The agent did not answer within a minute.")
	}
	return apiErr("AGENT_FAILED", "The agent could not do the job: %s", shorten(err.Error(), 200))
}

// openJobAgent starts the agent for a project (on accountID, or the project's account
// when empty) and opens a session in the jobs folder. The caller closes it.
func (m *Manager) openJobAgent(ctx context.Context, agent, projectID, accountID string) (*jobAgent, error) {
	a, dir, err := m.startJobAgent(ctx, agent, projectID, accountID)
	if err != nil {
		return nil, err
	}
	if err := a.newSession(ctx, dir); err != nil {
		a.close()
		return nil, jobError(ctx, err)
	}
	return a, nil
}

// startJobAgent starts the agent in the jobs folder and runs initialize, without a session.
func (m *Manager) startJobAgent(ctx context.Context, agent, projectID, accountID string) (*jobAgent, string, error) {
	def, ok := m.catalog[agent]
	if !ok {
		return nil, "", apiErr("NOT_FOUND", "There is no agent called %q.", agent)
	}
	if in := def.Info(); !in.Installed {
		return nil, "", apiErr("AGENT_NOT_INSTALLED", "%s needs %q, which was not found on this machine.", def.Name, in.Missing)
	}
	var accEnv []string
	if m.Accounts != nil {
		id, err := m.Accounts.Pick(projectID, agent, accountID, true)
		if err == nil {
			accEnv, err = m.Accounts.Env(agent, id)
		}
		if err != nil {
			return nil, "", err
		}
	}
	dir, err := m.jobsDir()
	if err != nil {
		return nil, "", err
	}
	p, err := startProc(launch{command: def.Command, args: def.Args, dir: dir, env: append(append([]string{}, def.Env...), accEnv...)})
	if err != nil {
		m.log.Error("chat: start job agent", "agent", agent, "err", err)
		return nil, "", apiErr("AGENT_FAILED", "The agent could not be started.")
	}
	a := &jobAgent{m: m, p: p, h: &jobHandler{}, name: def.Name}
	a.rpc = newRPC(p.out, p.in, a.h)
	if err := a.initialize(ctx); err != nil {
		a.close()
		return nil, "", jobError(ctx, err)
	}
	return a, dir, nil
}

func (a *jobAgent) initialize(ctx context.Context) error {
	var ir initResult
	err := a.rpc.Call(ctx, "initialize", map[string]any{
		"protocolVersion": 1, "clientCapabilities": map[string]any{},
		"clientInfo": map[string]string{"name": "mooring", "title": "Mooring", "version": "0"},
	}, &ir)
	if err != nil {
		return err
	}
	a.caps = ir.caps()
	if a.caps.Delete {
		a.closeBy = "session/delete"
	} else if a.caps.Close {
		a.closeBy = "session/close"
	}
	return nil
}

func (a *jobAgent) newSession(ctx context.Context, dir string) error {
	if err := a.rpc.Call(ctx, "session/new", map[string]any{"cwd": dir, "mcpServers": []any{}}, &a.session); err != nil {
		return err
	}
	a.sid = a.session.SessionID
	return nil
}

// models lists what the session offers: the config option with category model,
// else the older `models` list.
func (a *jobAgent) models() (cfgID string, choices []Choice) {
	if id, _, list, ok := modelOption(a.session.ConfigOptions); ok {
		return id, list
	}
	if mo := a.session.Models; mo != nil {
		for _, m := range mo.AvailableModels {
			choices = append(choices, Choice{ID: m.ModelID, Name: m.Name})
		}
	}
	return "", choices
}

// choose sets the model and the asking mode.
func (a *jobAgent) choose(ctx context.Context, model string) error {
	cfgID, choices := a.models()
	if !slices.ContainsFunc(choices, func(c Choice) bool { return c.ID == model }) {
		return apiErr("MODEL_GONE", "%s no longer offers %s. Small jobs for %s are off until you pick another model.", a.name, model, a.name)
	}
	var err error
	if cfgID != "" {
		err = a.rpc.Call(ctx, "session/set_config_option", map[string]string{"sessionId": a.sid, "configId": cfgID, "value": model}, nil)
	} else {
		err = a.rpc.Call(ctx, "session/set_model", map[string]string{"sessionId": a.sid, "modelId": model}, nil)
	}
	if err != nil {
		return err
	}
	if md := a.session.Modes; md != nil {
		for _, m := range md.AvailableModes {
			if slices.Contains(askingModes, m.ID) && m.ID != md.CurrentModeID {
				return a.rpc.Call(ctx, "session/set_mode", map[string]string{"sessionId": a.sid, "modeId": m.ID}, nil)
			}
		}
	}
	return nil
}

// close deletes (or closes) the session when the agent can, then stops the agent's group.
func (a *jobAgent) close() {
	if a.closeBy != "" && a.sid != "" && a.rpc != nil && a.p.running() {
		ctx, cancel := context.WithTimeout(context.Background(), jobClose)
		if err := a.rpc.Call(ctx, a.closeBy, map[string]string{"sessionId": a.sid}, nil); err != nil {
			a.m.log.Warn("chat: job session not removed", "method", a.closeBy, "err", err)
		}
		cancel()
	}
	a.p.stop(a.m.KillGrace)
}
