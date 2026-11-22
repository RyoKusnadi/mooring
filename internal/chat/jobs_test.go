package chat

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

func (r *rig) saveSettings(body string) {
	r.t.Helper()
	if _, err := r.set.Save(json.RawMessage(body)); err != nil {
		r.t.Fatal(err)
	}
}

func (r *rig) calls(file string) string {
	raw, _ := os.ReadFile(file)
	return string(raw)
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var le *ledger.Error
	if !errors.As(err, &le) || le.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func (r *rig) staging(stat, diff string) {
	r.staged = func(context.Context, string, string) (string, string, error) { return stat, diff, nil }
}

func TestTitleJobNamesAChatTheAgentDidNotName(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	r := newRig(t, "FAKE_CLOSE=close", "FAKE_CALLS="+calls)
	r.saveSettings(`{"titleJobs":true,"smallModel":{"fake":"deep"}}`)
	c := r.start("c1")
	r.ask(c, "hello")
	waitFor(t, "the title job", func() bool { return c.Info().Title == "Fake title" })
	got := r.calls(calls)
	if !strings.Contains(got, "session/set_config_option:deep") || !strings.Contains(got, "session/close") {
		t.Fatalf("calls: %q", got)
	}
	if _, err := os.Stat(filepath.Join(r.data, "jobs")); err != nil {
		t.Fatalf("jobs folder: %v", err)
	}
	var tokens int64
	_ = r.db.QueryRow(`SELECT tokens FROM job_usage WHERE kind = 'title'`).Scan(&tokens)
	if tokens != 1050 {
		t.Fatalf("job usage %d", tokens)
	}
	// Only the first turn can start a title job.
	r.ask(c, "again")
	time.Sleep(100 * time.Millisecond)
	var runs int
	_ = r.db.QueryRow(`SELECT SUM(runs) FROM job_usage`).Scan(&runs)
	if runs != 1 {
		t.Fatalf("runs %d", runs)
	}
}

func TestTitleJobSkippedWhenAgentTitledOrOff(t *testing.T) {
	r := newRig(t, "FAKE_TITLE=1")
	r.saveSettings(`{"titleJobs":true,"smallModel":{"fake":"deep"}}`)
	c := r.start("c1")
	r.ask(c, "hello")
	time.Sleep(200 * time.Millisecond)
	if got := c.Info().Title; got != "Fake chat title" {
		t.Fatalf("title %q", got)
	}
	r2 := newRig(t)
	r2.saveSettings(`{"smallModel":{"fake":"deep"}}`) // titleJobs stays off
	c2 := r2.start("c1")
	r2.ask(c2, "hello")
	time.Sleep(200 * time.Millisecond)
	if got := c2.Info().Title; got != "hello" {
		t.Fatalf("title %q", got)
	}
}

func TestSuggestMessage(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	r := newRig(t, "FAKE_CLOSE=delete", "FAKE_CALLS="+calls)
	_, err := r.run("git.suggestMessage", map[string]any{"projectId": "p1", "folder": ""})
	wantCode(t, err, "BAD_REQUEST") // jobs are off until the user turns them on
	r.saveSettings(`{"commitJobs":true,"smallModel":{"fake":"fast"}}`)
	_, err = r.run("git.suggestMessage", map[string]any{"projectId": "p1", "folder": ""})
	wantCode(t, err, "NOTHING_TO_COMMIT")
	r.staging(" a.txt | 1 +\n", "+alpha\n")
	v, err := r.run("git.suggestMessage", map[string]any{"projectId": "p1", "folder": ""})
	if err != nil {
		t.Fatal(err)
	}
	got := v.(map[string]any)
	if got["message"] != "fake: commit subject" || got["tokens"] != int64(1050) {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(r.calls(calls), "session/delete") {
		t.Fatalf("calls: %q", r.calls(calls))
	}
}

func TestJobDailyCap(t *testing.T) {
	r := newRig(t)
	r.saveSettings(`{"commitJobs":true,"smallModel":{"fake":"fast"},"jobTokensPerDay":2000}`)
	r.staging("a | 1\n", "+x\n")
	ask := func() error {
		_, err := r.run("git.suggestMessage", map[string]any{"projectId": "p1"})
		return err
	}
	if err := ask(); err != nil { // 1050 of 2000
		t.Fatal(err)
	}
	if err := ask(); err != nil { // 2100 of 2000: the cap is checked before a job starts
		t.Fatal(err)
	}
	wantCode(t, ask(), "LIMIT_REACHED")
}

func TestJobTimeoutAndOneJobPerProject(t *testing.T) {
	r := newRig(t)
	r.m.JobTimeout = time.Second
	r.saveSettings(`{"commitJobs":true,"smallModel":{"fake":"fast"}}`)
	r.staging("a | 1\n", "SLOW\n")
	first := make(chan error, 1)
	go func() {
		_, err := r.run("git.suggestMessage", map[string]any{"projectId": "p1"})
		first <- err
	}()
	waitFor(t, "the first job", func() bool {
		r.m.jobMu.Lock()
		defer r.m.jobMu.Unlock()
		return r.m.jobBusy["p1"]
	})
	_, err := r.run("git.suggestMessage", map[string]any{"projectId": "p1"})
	wantCode(t, err, "BUSY")
	wantCode(t, <-first, "TIMEOUT")
	r.staging("a | 1\n", "+fine\n")
	if _, err := r.run("git.suggestMessage", map[string]any{"projectId": "p1"}); err != nil {
		t.Fatalf("after a timeout the project is free: %v", err)
	}
}

func TestSmallModelGone(t *testing.T) {
	r := newRig(t)
	r.saveSettings(`{"commitJobs":true,"smallModel":{"fake":"gone"}}`)
	r.staging("a | 1\n", "+x\n")
	_, err := r.run("git.suggestMessage", map[string]any{"projectId": "p1"})
	wantCode(t, err, "MODEL_GONE")
}

func TestAgentModelsSendsNoPrompt(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	r := newRig(t, "FAKE_CLOSE=close", "FAKE_CALLS="+calls)
	v, err := r.run("agent.models", map[string]any{"agent": "fake", "projectId": "p1"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	if string(raw) != `{"models":[{"id":"fast","name":"Fast"},{"id":"deep","name":"Deep"}]}` {
		t.Fatalf("got %s", raw)
	}
	if got := r.calls(calls); got != "session/close\n" {
		t.Fatalf("calls: %q", got)
	}
	var runs int
	_ = r.db.QueryRow(`SELECT COUNT(*) FROM job_usage`).Scan(&runs)
	if runs != 0 {
		t.Fatal("no job should be recorded")
	}
	_, err = r.run("agent.models", map[string]any{"agent": "ghost", "projectId": "p1"})
	wantCode(t, err, "NOT_FOUND")
}

func TestJobRefusesPermissions(t *testing.T) {
	h := &jobHandler{}
	params := `{"toolCall":{"toolCallId":"t"},"options":[{"optionId":"a","name":"Allow","kind":"allow_once"},{"optionId":"r","name":"No","kind":"reject_once"}]}`
	got, err := h.request(context.Background(), "session/request_permission", json.RawMessage(params))
	raw, _ := json.Marshal(got)
	if err != nil || string(raw) != `{"outcome":{"optionId":"r","outcome":"selected"}}` {
		t.Fatalf("got %s %v", raw, err)
	}
	if _, err := h.request(context.Background(), "terminal/create", nil); err == nil {
		t.Fatal("a job must not get terminals")
	}
}

func TestCleanMessageAndTitle(t *testing.T) {
	if got := cleanTitle("\n\"Fix the login bug.\"\nextra"); got != "Fix the login bug." {
		t.Fatalf("title %q", got)
	}
	long := strings.Repeat("x", 100)
	if got := cleanTitle(long); len(got) != 80 {
		t.Fatalf("title %d", len(got))
	}
	body := strings.Repeat("line\n", 14)
	got := cleanMessage("```\n" + long + "\n\n" + body + "```")
	lines := strings.Split(got, "\n")
	if len(lines[0]) != 72 || lines[1] != "" || len(lines) != 12 {
		t.Fatalf("message %q (%d lines)", got, len(lines))
	}
	if cleanMessage("  \n") != "" {
		t.Fatal("an empty answer is empty")
	}
}
