package bench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ryokusnadi/mooring/internal/chat"
	"github.com/ryokusnadi/mooring/internal/events"
)

// tracker is what one task knows about its chats while it runs.
type tracker struct {
	project, dir string
	chats        []string
	jobsBefore   int64
	notes        []string
}

func (r *runner) cmd(name string, args map[string]any) (any, error) {
	raw, _ := json.Marshal(args)
	return r.e.m.Run(events.Command{ID: "bench", Name: name, Args: raw})
}

// tokens is what the task has used so far: per chat the larger of the turns' total
// and the context size, plus the small jobs.
func (r *runner) tokens(tk *tracker) int64 {
	var n int64
	for _, id := range tk.chats {
		s, info, ok := r.chatNumbers(id)
		if ok {
			n += max(s.sum, info)
		}
	}
	return n + r.e.jobs() - tk.jobsBefore
}

type sums struct {
	turns                          int
	in, out, cacheRead, cacheWrite int64
	sum                            int64
}

func (r *runner) chatNumbers(id string) (sums, int64, bool) {
	st, err := r.e.m.Stats(id)
	if err != nil {
		return sums{}, 0, false
	}
	var s sums
	for _, t := range st.Turns {
		s.turns++
		s.in += t.InputTokens
		s.out += t.OutputTokens
		s.cacheRead += t.CachedReadTokens
		s.cacheWrite += t.CachedWriteTokens
		if t.TotalTokens > 0 {
			s.sum += t.TotalTokens
		} else {
			s.sum += t.InputTokens + t.OutputTokens
		}
	}
	var used int64
	if info, err := r.e.m.Info(id); err == nil && info.Usage != nil {
		used = info.Usage.Used
	}
	return s, used, true
}

func (r *runner) overBudget(tk *tracker) (bool, string) {
	task := r.tokens(tk)
	if task > r.o.Budget/10 {
		return true, fmt.Sprintf("the task used %d tokens, over a tenth of the budget (%d)", task, r.o.Budget/10)
	}
	if r.spent+task > r.o.Budget {
		return true, fmt.Sprintf("the run used %d tokens, over the budget (%d)", r.spent+task, r.o.Budget)
	}
	return false, ""
}

// await waits until the chat is idle, answering permission requests and watching
// the budget. code is "" when the chat is idle.
func (r *runner) await(ctx context.Context, tk *tracker, id string) (code, detail string) {
	tick := time.NewTicker(r.o.Poll)
	defer tick.Stop()
	stop := func() {
		_, _ = r.cmd("chat.cancel", map[string]any{"id": id})
		_, _ = r.cmd("chat.close", map[string]any{"id": id})
	}
	for {
		if over, why := r.overBudget(tk); over {
			stop()
			return "budget", why
		}
		info, err := r.e.m.Info(id)
		if err != nil {
			return "error", err.Error()
		}
		switch {
		case info.State == chat.StateIdle:
			return "", ""
		case info.State == chat.StateStopped || info.State == chat.StateNeedsLogin:
			msg := "the agent stopped"
			if info.Error != nil {
				msg = info.Error.Message
			}
			return "error", msg
		case info.Attention:
			r.answer(tk, id)
		}
		select {
		case <-ctx.Done():
			stop()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return "timeout", "the task ran past its time limit"
			}
			return "error", "the run was cancelled"
		case <-tick.C:
		}
	}
}

func (r *runner) answer(tk *tracker, id string) {
	open, err := r.e.m.OpenPermissions(id)
	if err != nil {
		return
	}
	for _, p := range open {
		opt, allowed := choosePermission(tk.dir, p)
		if opt == "" {
			continue
		}
		if !allowed {
			tk.notes = append(tk.notes, "rejected a request outside the folder: "+p.Title)
		}
		if _, err := r.cmd("chat.permission", map[string]any{"id": id, "requestId": p.RequestID, "optionId": opt}); err != nil {
			r.log.Warn("bench: permission answer failed", "err", err)
		}
	}
}

func (r *runner) startChat(ctx context.Context, tk *tracker, mode, summary, from string) (string, string, string) {
	id := fmt.Sprintf("c-%s-%d", filepath.Base(tk.dir), len(tk.chats)+1)
	filters := []string{}
	if mode == "on" {
		filters = []string{"compact"}
	}
	args := map[string]any{"id": id, "projectId": tk.project, "agent": r.e.agent,
		"spec": map[string]any{"account": map[string]string{"id": r.o.Account}, "filters": filters}}
	if summary != "" {
		args["context"], args["fromChat"] = summary, from
	}
	if _, err := r.cmd("chat.start", args); err != nil {
		return id, "error", err.Error()
	}
	tk.chats = append(tk.chats, id)
	code, detail := r.await(ctx, tk, id)
	return id, code, detail
}

// drive runs every part of a task and returns "" or the reason it stopped.
func (r *runner) drive(ctx context.Context, t task, mode string, tk *tracker) (code, detail string) {
	cur := ""
	for i, p := range t.parts {
		switch {
		case i == 0:
			id, c, d := r.startChat(ctx, tk, mode, "", "")
			if c != "" {
				return c, d
			}
			cur = id
		case mode == "on":
			v, err := r.cmd("chat.summarize", map[string]any{"id": cur})
			if err != nil {
				return "error", "summary: " + err.Error()
			}
			text := v.(map[string]string)["text"]
			id, c, d := r.startChat(ctx, tk, mode, text, cur)
			if c != "" {
				return c, d
			}
			cur = id
		}
		text := p.prompt
		if r.o.Dry {
			text = p.dry + " " + mode
		}
		if _, err := r.cmd("chat.prompt", map[string]any{"id": cur, "text": text}); err != nil {
			return "error", err.Error()
		}
		if c, d := r.await(ctx, tk, cur); c != "" {
			return c, d
		}
	}
	return "", ""
}

// runCheck runs bench/check/<task>.sh and says whether it passed.
func (r *runner) runCheck(ctx context.Context, name, dir string) (bool, string) {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	script := filepath.Join(r.o.BenchDir, "check", name+".sh")
	out, err := exec.CommandContext(cctx, "sh", script, dir).CombinedOutput()
	last := ""
	if lines := strings.Split(strings.TrimSpace(string(out)), "\n"); len(lines) > 0 {
		last = lines[len(lines)-1]
	}
	return err == nil, last
}
