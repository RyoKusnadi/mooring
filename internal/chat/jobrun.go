package chat

import (
	"context"
	"errors"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// jobUsage is the usage of a job's prompt result.
type jobUsage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	TotalTokens  int64 `json:"totalTokens"`
}

func (u jobUsage) total() int64 {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.InputTokens + u.OutputTokens
}

// JobRecord is what one finished job spent; the bench hooks it.
type JobRecord struct {
	Kind, Agent, Model string
	Tokens             int64
}

// jobReq is one job.
type jobReq struct {
	kind, agent, model, projectID, accountID, prompt string
}

func (m *Manager) jobWait() time.Duration {
	if m.JobTimeout > 0 {
		return m.JobTimeout
	}
	return jobTimeout
}

// runJob checks the limits, runs one prompt in a fresh session and returns the
// reply and the tokens spent.
func (m *Manager) runJob(q jobReq) (string, int64, error) {
	if !m.takeProject(q.projectID) {
		return "", 0, apiErr("BUSY", "A small job is already running in this project. Try again in a moment.")
	}
	defer m.freeProject(q.projectID)
	if err := m.checkJobCap(); err != nil {
		return "", 0, err
	}
	ctx, cancel := context.WithTimeout(m.baseCtx(), m.jobWait())
	defer cancel()
	a, err := m.openJobAgent(ctx, q.agent, q.projectID, q.accountID)
	if err != nil {
		return "", 0, err
	}
	defer a.close()
	if err := a.choose(ctx, q.model); err != nil {
		var le *ledger.Error
		if errors.As(err, &le) {
			return "", 0, err
		}
		return "", 0, jobError(ctx, err)
	}
	var res struct {
		Usage jobUsage `json:"usage"`
	}
	err = a.rpc.Call(ctx, "session/prompt", map[string]any{
		"sessionId": a.sid, "prompt": []any{map[string]string{"type": "text", "text": q.prompt}},
	}, &res)
	if err != nil {
		if ctx.Err() != nil {
			_ = a.rpc.Notify("session/cancel", map[string]string{"sessionId": a.sid})
		}
		return "", 0, jobError(ctx, err)
	}
	tokens := res.Usage.total()
	m.recordJob(JobRecord{Kind: q.kind, Agent: q.agent, Model: q.model, Tokens: tokens})
	text := a.h.reply()
	if text == "" {
		return "", tokens, apiErr("AGENT_FAILED", "The model sent no answer.")
	}
	return text, tokens, nil
}

func (m *Manager) takeProject(id string) bool {
	m.jobMu.Lock()
	defer m.jobMu.Unlock()
	if m.jobBusy == nil {
		m.jobBusy = map[string]bool{}
	}
	if m.jobBusy[id] {
		return false
	}
	m.jobBusy[id] = true
	return true
}

func (m *Manager) freeProject(id string) {
	m.jobMu.Lock()
	delete(m.jobBusy, id)
	m.jobMu.Unlock()
}

func (m *Manager) today() string { return m.now().Format("2006-01-02") }

// checkJobCap refuses a job once today's jobs used the daily token limit.
func (m *Manager) checkJobCap() error {
	limit := int64(200000)
	if m.Settings != nil {
		limit = m.Settings.Get().JobTokensPerDay
	}
	var used int64
	if err := m.db.QueryRow(`SELECT COALESCE(SUM(tokens), 0) FROM job_usage WHERE day = ?`, m.today()).Scan(&used); err != nil {
		m.log.Error("chat: read job usage", "err", err)
		return apiErr("INTERNAL", "The job usage could not be read.")
	}
	if used >= limit {
		return apiErr("LIMIT_REACHED", "Small jobs used %d of %d tokens today. The limit can be raised in the settings; it starts again tomorrow.", used, limit)
	}
	return nil
}

func (m *Manager) recordJob(r JobRecord) {
	_, err := m.db.Exec(`INSERT INTO job_usage (day, agent, model, kind, runs, tokens) VALUES (?, ?, ?, ?, 1, ?)
		ON CONFLICT(day, agent, model, kind) DO UPDATE SET runs = runs + 1, tokens = tokens + excluded.tokens`,
		m.today(), r.Agent, r.Model, r.Kind, r.Tokens)
	if err != nil {
		m.log.Error("chat: record job usage", "err", err)
	}
	if m.JobHook != nil {
		m.JobHook(r)
	}
}

func (m *Manager) baseCtx() context.Context {
	if m.Context != nil {
		return m.Context
	}
	return context.Background()
}
