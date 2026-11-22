package bench

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// runTask runs one task in one mode in a fresh copy of the fixture.
func (r *runner) runTask(ctx context.Context, t task, mode string) (res Result) {
	res = Result{Task: t.name, Mode: mode}
	started := time.Now()
	defer func() { res.Seconds = time.Since(started).Seconds() }()
	fail := func(err error) Result { res.Result, res.Detail = "error", err.Error(); return res }

	dir := filepath.Join(r.tmp, t.name+"-"+mode)
	if err := prepare(r.o.BenchDir, dir); err != nil {
		return fail(err)
	}
	tk := &tracker{project: "p-" + t.name + "-" + mode, dir: dir, jobsBefore: r.e.jobs()}
	if _, err := r.e.db.Exec(`INSERT INTO projects (id, name, path, position, created_at) VALUES (?, ?, ?, 0, ?)`,
		tk.project, t.name+" "+mode, dir, time.Now().UnixMilli()); err != nil {
		return fail(fmt.Errorf("add the project: %w", err))
	}
	r.e.mu.Lock()
	r.e.dirs[tk.project] = dir
	r.e.mu.Unlock()
	if err := r.e.useJobs(mode == "on"); err != nil {
		return fail(err)
	}

	tctx, cancel := context.WithTimeout(ctx, r.o.Timeout)
	defer cancel()
	code, detail := r.drive(tctx, t, mode, tk)
	if code == "" && mode == "on" {
		detail = r.finishJobs(tctx, tk)
	}
	if code == "" {
		if over, why := r.overBudget(tk); over {
			code, detail = "budget", why
		}
	}
	switch code {
	case "":
		ok, out := r.runCheck(ctx, t.name, dir)
		res.Result = "pass"
		if !ok {
			res.Result, detail = "fail", out
		}
	default:
		res.Result = code
	}
	r.collect(&res, tk)
	var parts []string
	for _, p := range append([]string{detail}, tk.notes...) {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	res.Detail = strings.Join(parts, "; ")
	return res
}

// finishJobs waits for the title job, then asks for a commit message for what the
// agent changed. It returns a note when a job did not run.
func (r *runner) finishJobs(ctx context.Context, tk *tracker) string {
	if !r.e.smallOn {
		return "no small model is set for " + r.e.agent + ", so the title and commit jobs did not run"
	}
	wctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := r.e.m.WaitJobs(wctx); err != nil {
		return "the title job did not finish"
	}
	if out, err := gitIn(tk.dir, "add", "-A"); err != nil {
		return "git add: " + strings.TrimSpace(out)
	}
	if _, _, err := r.e.m.SuggestMessage(ctx, tk.project, ""); err != nil {
		return "commit message job: " + err.Error()
	}
	return ""
}

// collect adds the token numbers of every chat of the task to res.
func (r *runner) collect(res *Result, tk *tracker) {
	var total int64
	for _, id := range tk.chats {
		s, used, ok := r.chatNumbers(id)
		if !ok {
			continue
		}
		res.Chats++
		res.Turns += s.turns
		res.Input += s.in
		res.Output += s.out
		res.CacheRead += s.cacheRead
		res.CacheWrite += s.cacheWrite
		res.ContextUsed = max(res.ContextUsed, used)
		total += max(s.sum, used)
		if info, err := r.e.m.Info(id); err == nil && info.Usage != nil {
			res.Cost += info.Usage.Cost
		}
		if st, err := r.e.m.Stats(id); err == nil && st.TerminalCalls > 0 {
			res.UsedTerminals = true
		}
	}
	res.JobTokens = r.e.jobs() - tk.jobsBefore
	res.Total = total + res.JobTokens
}
