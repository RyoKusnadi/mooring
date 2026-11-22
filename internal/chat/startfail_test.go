package chat

import (
	"strings"
	"testing"
)

// A chat that cannot start says why, not just that it failed.
func TestStartFailureNamesTheCause(t *testing.T) {
	for _, tc := range []struct{ name, command, want string }{
		{"missing program", "/nonexistent/agent-program", "no such file or directory"},
		{"exits at once", "/usr/bin/false", "exit status 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			d := r.m.catalog["fake"]
			d.Command, d.Args, d.Env = tc.command, nil, nil
			r.m.catalog["fake"] = d
			if _, err := r.run("chat.start", map[string]any{"id": "c1", "projectId": r.project}); err != nil {
				t.Fatal(err)
			}
			c := r.m.get("c1")
			r.waitState(c, StateStopped)
			got := c.Info().Error
			if got == nil || !strings.Contains(got.Message, tc.want) {
				t.Fatalf("error = %+v, want it to mention %q", got, tc.want)
			}
		})
	}
}

// What the agent wrote to stderr reaches the browser cleaned, and the raw line is never stored.
func TestStartFailureHidesSecrets(t *testing.T) {
	r := newRig(t)
	d := r.m.catalog["fake"]
	d.Command, d.Env = "/bin/sh", nil
	d.Args = []string{"-c", `printf '\033[31mbad ANTHROPIC_API_KEY=sk-ant-api03-abcdefghijklmnop\033[0m\n' >&2; exit 3`}
	r.m.catalog["fake"] = d
	if _, err := r.run("chat.start", map[string]any{"id": "c1", "projectId": r.project}); err != nil {
		t.Fatal(err)
	}
	c := r.m.get("c1")
	r.waitState(c, StateStopped)
	got := c.Info().Error
	if got == nil || !strings.Contains(got.Message, "ANTHROPIC_API_KEY=[hidden]") {
		t.Fatalf("error = %+v, want the key hidden", got)
	}
	if strings.ContainsAny(got.Message, "\x1b") || strings.Contains(got.Message, "sk-ant") {
		t.Fatalf("message not clean: %q", got.Message)
	}
}
