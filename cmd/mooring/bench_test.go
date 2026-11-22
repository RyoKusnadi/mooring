package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryokusnadi/mooring/internal/bench"
	"github.com/ryokusnadi/mooring/internal/fakeagent"
)

// The test binary doubles as the stand-in agent when MOORING_FAKE_AGENT=1.
func TestMain(m *testing.M) {
	if os.Getenv("MOORING_FAKE_AGENT") == "1" {
		fakeagent.Run(os.Stdin, os.Stdout)
		return
	}
	os.Exit(m.Run())
}

func runDry(t *testing.T, budget string) (bench.Report, string) {
	t.Helper()
	data := t.TempDir()
	var out, errOut bytes.Buffer
	args := []string{"bench", "-dry", "-data", data, "-bench-dir", "../../bench", "-tmp", t.TempDir(), "-budget-tokens", budget, "-stand-in", os.Args[0]}
	if err := run(args, &out, &errOut); err != nil {
		t.Fatalf("bench: %v\n%s\n%s", err, out.String(), errOut.String())
	}
	runs, _ := filepath.Glob(filepath.Join(data, "bench", "*", "results.json"))
	if len(runs) != 1 {
		t.Fatalf("results files: %v", runs)
	}
	raw, err := os.ReadFile(runs[0])
	if err != nil {
		t.Fatal(err)
	}
	var rep bench.Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(filepath.Dir(runs[0]), "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	return rep, string(md)
}

func TestBenchDry(t *testing.T) {
	rep, md := runDry(t, "3000000")
	if !rep.Dry || rep.Agent != "claude" || rep.Stopped != "" || len(rep.Results) != 10 {
		t.Fatalf("report: %+v", rep)
	}
	total := map[string]map[string]int64{}
	for _, r := range rep.Results {
		if r.Result != "pass" {
			t.Errorf("%s %s: %s (%s)", r.Task, r.Mode, r.Result, r.Detail)
		}
		if r.Turns == 0 || r.Total == 0 || !r.UsedTerminals {
			t.Errorf("%s %s: no usage recorded: %+v", r.Task, r.Mode, r)
		}
		if total[r.Task] == nil {
			total[r.Task] = map[string]int64{}
		}
		total[r.Task][r.Mode] = r.Total
		if r.Mode == "on" && r.JobTokens == 0 {
			t.Errorf("%s on: the title and commit jobs used no tokens", r.Task)
		}
		if r.Mode == "off" && r.JobTokens != 0 {
			t.Errorf("%s off: jobs ran", r.Task)
		}
		if r.Task == "continue" && r.Chats != map[string]int{"off": 1, "on": 2}[r.Mode] {
			t.Errorf("continue %s: %d chats", r.Mode, r.Chats)
		}
	}
	if on, off := total["find-log"]["on"], total["find-log"]["off"]; on == 0 || on >= off {
		t.Errorf("find-log: on %d should be below off %d", on, off)
	}
	for _, want := range []string{"| find-log | on | pass |", "| continue | off | pass |", "Saved"} {
		if !strings.Contains(md, want) {
			t.Errorf("report.md lacks %q", want)
		}
	}
}

func TestBenchDryStopsOnBudget(t *testing.T) {
	rep, _ := runDry(t, "3000")
	if len(rep.Results) != 1 || rep.Results[0].Result != "budget" || rep.Stopped == "" {
		t.Fatalf("report: %+v", rep)
	}
}

func TestBenchRealRunNeedsYes(t *testing.T) {
	var out bytes.Buffer
	err := runBench([]string{"-data", t.TempDir(), "-bench-dir", "../../bench", "-tasks", "find-log"}, strings.NewReader("no\n"), &out, &out)
	if err == nil || !strings.Contains(out.String(), "Type yes to start") || !strings.Contains(out.String(), "Account:  the default account of claude") {
		t.Fatalf("err %v, out %q", err, out.String())
	}
}
