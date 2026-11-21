package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Result is one task in one mode.
type Result struct {
	Task          string   `json:"task"`
	Mode          string   `json:"mode"`
	Result        string   `json:"result"` // pass, fail, budget, timeout, error
	Detail        string   `json:"detail,omitempty"`
	Chats         int      `json:"chats"`
	Turns         int      `json:"turns"`
	Input         int64    `json:"input"`
	Output        int64    `json:"output"`
	CacheRead     int64    `json:"cacheRead"`
	CacheWrite    int64    `json:"cacheWrite"`
	Total         int64    `json:"total"` // every turn plus the small jobs
	Cost          float64  `json:"cost"`
	ContextUsed   int64    `json:"contextUsed"`
	JobTokens     int64    `json:"jobTokens"`
	SavedPct      *float64 `json:"savedPct,omitempty"` // on rows: against the off row of the same task
	UsedTerminals bool     `json:"usedTerminals"`
	Seconds       float64  `json:"seconds"`
}

// Report is what a run writes to results.json.
type Report struct {
	RunID    string   `json:"runId"`
	Agent    string   `json:"agent"`
	Account  string   `json:"account"`
	Dry      bool     `json:"dry"`
	Budget   int64    `json:"budgetTokens"`
	Started  string   `json:"started"`
	Finished string   `json:"finished"`
	Stopped  string   `json:"stopped,omitempty"` // why the run ended early
	Work     string   `json:"workFolder"`
	Results  []Result `json:"results"`
}

// fillSaved sets SavedPct on every on row that has a passing off row of the same task.
func fillSaved(rs []Result) {
	off := map[string]Result{}
	for _, r := range rs {
		if r.Mode == "off" {
			off[r.Task] = r
		}
	}
	for i := range rs {
		o, ok := off[rs[i].Task]
		if rs[i].Mode != "on" || !ok || o.Total <= 0 || o.Result == "budget" || rs[i].Result == "budget" {
			continue
		}
		pct := float64(o.Total-rs[i].Total) * 100 / float64(o.Total)
		rs[i].SavedPct = &pct
	}
}

func write(dir string, r *Report) error {
	fillSaved(r.Results)
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("encode results: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "results.json"), append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("write results.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "report.md"), []byte(markdown(r)), 0o600); err != nil {
		return fmt.Errorf("write report.md: %w", err)
	}
	return nil
}

func markdown(r *Report) string {
	var b strings.Builder
	kind := "real"
	if r.Dry {
		kind = "dry (stand-in agent, numbers are not real)"
	}
	fmt.Fprintf(&b, "# Mooring bench %s\n\n", r.RunID)
	fmt.Fprintf(&b, "- Agent: %s\n- Account: %s\n- Run: %s\n- Budget: %d tokens\n- Work folders: %s\n", r.Agent, r.Account, kind, r.Budget, r.Work)
	if r.Stopped != "" {
		fmt.Fprintf(&b, "- Stopped early: %s\n", r.Stopped)
	}
	b.WriteString("\n| Task | Mode | Result | Turns | Input | Output | Cache read | Cache write | Total | Cost | Context | Job tokens | Saved | Ran commands through Mooring |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, x := range r.Results {
		saved := "-"
		if x.SavedPct != nil {
			saved = fmt.Sprintf("%.0f%%", *x.SavedPct)
		}
		term := "no"
		if x.UsedTerminals {
			term = "yes"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %d | %d | %d | %d | %.4f | %d | %d | %s | %s |\n",
			x.Task, x.Mode, x.Result, x.Turns, x.Input, x.Output, x.CacheRead, x.CacheWrite, x.Total, x.Cost, x.ContextUsed, x.JobTokens, saved, term)
	}
	b.WriteString("\nContext is the last context size the agent reported, not tokens spent. Total adds every turn and the small jobs.\n")
	for _, x := range r.Results {
		if x.Detail != "" {
			fmt.Fprintf(&b, "\n- %s %s: %s", x.Task, x.Mode, x.Detail)
		}
	}
	noTerm := false
	for _, x := range r.Results {
		noTerm = noTerm || (x.Mode == "on" && !x.UsedTerminals)
	}
	if noTerm {
		b.WriteString("\n\nIn some on rows the agent never asked Mooring to run a command, so output compaction could not apply there.")
	}
	b.WriteString("\n")
	return b.String()
}
