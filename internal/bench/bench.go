// Package bench measures what the token-saving features save: it runs a few coding
// tasks on a copy of bench/fixture, with the features off and on, and reports the
// tokens each run used.
package bench

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options is what a run needs.
type Options struct {
	Agent, Account string
	Tasks, Modes   []string // names; empty means all tasks, off and on
	Budget         int64    // tokens for the whole run
	Dry            bool     // use the stand-in agent
	DataDir        string   // Mooring's data folder: accounts and results live here
	BenchDir       string   // the folder with fixture, check and fix
	TmpDir         string   // where task folders go; empty means the system's
	Exe            string   // the stand-in agent program, needed with Dry
	In             io.Reader
	Out            io.Writer
	Log            *slog.Logger
	Timeout        time.Duration // per task; 0 means 10 minutes
	Poll           time.Duration
}

type runner struct {
	o     Options
	e     *env
	log   *slog.Logger
	tmp   string
	spent int64 // tokens of the finished tasks
}

// Run runs the tasks and writes results.json and report.md to <data>/bench/<runId>.
// A real run first shows what it will do and waits for "yes" on o.In.
func Run(ctx context.Context, o Options) (*Report, error) {
	if err := o.fill(); err != nil {
		return nil, err
	}
	if !o.Dry && !confirm(o) {
		return nil, fmt.Errorf("not started: the answer was not yes")
	}
	runID, runDir, err := newRunDir(o.DataDir)
	if err != nil {
		return nil, err
	}
	e, err := newEnv(ctx, o, runDir, o.Log)
	if err != nil {
		return nil, err
	}
	defer e.close()
	r := &runner{o: o, e: e, log: o.Log, tmp: filepath.Join(o.TmpDir, "mooring-bench-"+runID)}
	rep := &Report{RunID: runID, Agent: o.Agent, Account: accountLabel(o), Dry: o.Dry, Budget: o.Budget, Started: time.Now().UTC().Format(time.RFC3339), Work: r.tmp}
	defer func() {
		rep.Finished = time.Now().UTC().Format(time.RFC3339)
		if err := write(runDir, rep); err != nil {
			o.Log.Error("bench: report not written", "err", err)
		}
	}()
	for _, name := range o.Tasks {
		t, _ := findTask(name)
		for _, mode := range o.Modes {
			if ctx.Err() != nil {
				rep.Stopped = "cancelled"
				return rep, nil
			}
			res := r.runTask(ctx, t, mode)
			rep.Results = append(rep.Results, res)
			r.spent += res.Total
			fmt.Fprintf(o.Out, "%-9s %-3s %-7s %9d tokens  %s\n", res.Task, res.Mode, res.Result, res.Total, res.Detail)
			if res.Result == "budget" {
				rep.Stopped = res.Task + " " + res.Mode + ": " + res.Detail
				return rep, nil
			}
		}
	}
	fmt.Fprintf(o.Out, "Report: %s\n", filepath.Join(runDir, "report.md"))
	return rep, nil
}

func (o *Options) fill() error {
	if o.Agent == "" {
		o.Agent = "claude"
	}
	if len(o.Tasks) == 0 || (len(o.Tasks) == 1 && o.Tasks[0] == "all") {
		o.Tasks = TaskNames()
	}
	for _, n := range o.Tasks {
		if _, ok := findTask(n); !ok {
			return fmt.Errorf("unknown task %q (tasks: %s)", n, strings.Join(TaskNames(), ", "))
		}
	}
	if len(o.Modes) == 0 {
		o.Modes = []string{"off", "on"}
	}
	for _, m := range o.Modes {
		if m != "off" && m != "on" {
			return fmt.Errorf("unknown mode %q (modes: off, on)", m)
		}
	}
	if o.Budget <= 0 {
		return fmt.Errorf("the token budget must be above 0")
	}
	if o.DataDir == "" {
		return fmt.Errorf("the data folder is not set")
	}
	if _, err := os.Stat(filepath.Join(o.BenchDir, "fixture", "go.mod")); err != nil {
		return fmt.Errorf("bench folder %q has no fixture (use -bench-dir): %w", o.BenchDir, err)
	}
	if o.TmpDir == "" {
		o.TmpDir = os.TempDir()
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	if o.Timeout == 0 {
		o.Timeout = 10 * time.Minute
	}
	if o.Poll == 0 {
		o.Poll = 100 * time.Millisecond
	}
	return nil
}

func accountLabel(o Options) string {
	switch {
	case o.Dry:
		return "none (stand-in)"
	case o.Account != "":
		return o.Account
	}
	return "the default account of " + o.Agent
}

func newRunDir(data string) (id, dir string, err error) {
	base := time.Now().UTC().Format("20060102-150405")
	for n := 0; n < 100; n++ {
		id = base
		if n > 0 {
			id = fmt.Sprintf("%s-%d", base, n+1)
		}
		dir = filepath.Join(data, "bench", id)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return "", "", fmt.Errorf("create the run folder: %w", err)
			}
			return id, dir, nil
		}
	}
	return "", "", fmt.Errorf("no free run folder under %s", filepath.Join(data, "bench"))
}

// confirm shows what a real run will spend and asks the user to type yes.
func confirm(o Options) bool {
	chats := 0
	for _, n := range o.Tasks {
		t, _ := findTask(n)
		for _, m := range o.Modes {
			chats++
			if m == "on" && len(t.parts) > 1 {
				chats += len(t.parts) - 1
			}
		}
	}
	fmt.Fprintf(o.Out, "Mooring bench, real run\n")
	fmt.Fprintf(o.Out, "  Agent:    %s\n  Account:  %s\n", o.Agent, accountLabel(o))
	fmt.Fprintf(o.Out, "  Tasks:    %s\n  Modes:    %s\n  Chats:    about %d, each in a fresh copy of the fixture\n", strings.Join(o.Tasks, ", "), strings.Join(o.Modes, ", "), chats)
	fmt.Fprintf(o.Out, "  Caps:     %d tokens for the run, %d for one task; a task or run over its cap is cancelled\n", o.Budget, o.Budget/10)
	fmt.Fprintf(o.Out, "  Time:     %s for one task\n", o.Timeout)
	fmt.Fprintf(o.Out, "  Safety:   the agent may change files only inside its task folder; other requests are refused\n")
	fmt.Fprintf(o.Out, "This uses your account's allowance. Type yes to start: ")
	if o.In == nil {
		return false
	}
	line, _ := bufio.NewReader(o.In).ReadString('\n')
	return strings.TrimSpace(line) == "yes"
}
