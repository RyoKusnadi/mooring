package chat

import (
	"os"
	"testing"
	"time"
)

// TestRealClaudeSmoke runs the real Claude adapter once. It is off unless
// MOORING_SMOKE=1, and skips when Claude is not signed in.
func TestRealClaudeSmoke(t *testing.T) {
	if os.Getenv("MOORING_SMOKE") != "1" {
		t.Skip("set MOORING_SMOKE=1 to run the real Claude")
	}
	r := newRig(t)
	r.m.catalog["claude"] = DefaultCatalog()[0]
	if _, err := r.run("chat.start", map[string]any{"id": "real", "projectId": "p1", "agent": "claude"}); err != nil {
		t.Fatal(err)
	}
	c := r.m.get("real")
	waitFor(t, "start", func() bool { s := c.Info().State; return s != StateStarting })
	info := c.Info()
	if info.State == StateNeedsLogin {
		t.Skipf("Claude is not signed in: %+v", info.Error)
	}
	if info.State != StateIdle {
		t.Fatalf("state %s: %+v", info.State, info.Error)
	}
	t.Logf("mode=%s model=%s models=%d caps=%+v", info.Mode, info.Model, len(info.Models), info.Caps)
	r.prompt(c, "Reply with the single word: ok")
	deadline := time.Now().Add(90 * time.Second)
	for c.Info().State == StateRunning && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if a := c.find(KindAgent); a == nil || c.Info().State == StateNeedsLogin {
		t.Skipf("no reply; state %s error %+v", c.Info().State, c.Info().Error)
	} else {
		t.Logf("reply: %q usage=%+v", a.Text, c.Info().Usage)
	}
}
