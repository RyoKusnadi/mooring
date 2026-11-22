package chat

import (
	"context"
	"strings"
	"testing"
)

func TestSecondOpinionGetsTheChanges(t *testing.T) {
	for _, embed := range []bool{true, false} {
		env := []string{}
		if embed {
			env = append(env, "FAKE_EMBED=1")
		}
		r := newRig(t, env...)
		var folder string
		stat, diff := " a.go | 2 +-\n", "+"+strings.Repeat("x", maxChangesDiff+10)
		r.m.Uncommitted = func(_ context.Context, p, f string) (string, string, error) {
			folder = f
			return stat, diff, nil
		}
		r.start("old")
		start := func(id string, a map[string]any) (map[string]any, error) {
			a["id"], a["projectId"], a["withChanges"] = id, r.project, true
			_, err := r.run("chat.start", a)
			return a, err
		}
		if _, err := start("x1", map[string]any{}); err == nil || !has(err.Error(), "BAD_REQUEST") {
			t.Fatalf("no chat to review: %v", err)
		}
		if _, err := start("new", map[string]any{"fromChat": "old", "context": "Goal: ship."}); err != nil {
			t.Fatal(err)
		}
		if folder != "." {
			t.Fatalf("asked git about %q", folder)
		}
		c := r.m.get("new")
		r.waitState(c, StateIdle)
		ctx := c.messages()[0]
		if ctx.Kind != KindContext || !strings.HasPrefix(ctx.Changes, "Files:\n a.go | 2 +-\n\nDiff:\n+x") || !has(ctx.Changes, "[cut here") || len(ctx.Changes) > maxChangesStat+maxChangesDiff+200 {
			t.Fatalf("context: %.300q", ctx.Changes)
		}
		got := r.ask(c, "BLOCKS")
		if embed && !has(got, "resource:mooring://summary/old:Goal: ship.|resource:mooring://changes/old:Files:") {
			t.Fatalf("embedded: %.300q", got)
		}
		if !embed && !has(got, "Summary of an earlier chat:\n> Goal: ship.\n\nUncommitted changes in that chat's folder:\n```\nFiles:") {
			t.Fatalf("plain: %.300q", got)
		}

		// Nothing changed: the review still starts, and says so.
		stat = ""
		if _, err := start("none", map[string]any{"fromChat": "old", "context": "Goal: ship."}); err != nil {
			t.Fatal(err)
		}
		if got := r.m.get("none").messages()[0].Changes; got != noChanges {
			t.Fatalf("no changes: %q", got)
		}
		// A chat started from another without asking for changes gets none.
		if _, err := r.run("chat.start", map[string]any{"id": "plain", "projectId": r.project, "fromChat": "old", "context": "s"}); err != nil {
			t.Fatal(err)
		}
		if got := r.m.get("plain").messages()[0].Changes; got != "" {
			t.Fatalf("unasked changes: %q", got)
		}
		r.m.Shutdown()
	}
}
