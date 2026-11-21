package bench

import (
	"strings"
	"testing"
)

func TestFillSavedAndMarkdown(t *testing.T) {
	rs := []Result{
		{Task: "a", Mode: "off", Result: "pass", Total: 1000},
		{Task: "a", Mode: "on", Result: "pass", Total: 400, UsedTerminals: true},
		{Task: "b", Mode: "off", Result: "budget", Total: 900},
		{Task: "b", Mode: "on", Result: "pass", Total: 100},
	}
	fillSaved(rs)
	if rs[1].SavedPct == nil || *rs[1].SavedPct != 60 {
		t.Fatalf("saved %v", rs[1].SavedPct)
	}
	if rs[3].SavedPct != nil {
		t.Fatal("a cancelled off row gives no comparison")
	}
	md := markdown(&Report{RunID: "r1", Agent: "claude", Budget: 5, Results: rs, Stopped: "over budget"})
	for _, want := range []string{"# Mooring bench r1", "| a | on | pass |", "60%", "Stopped early: over budget"} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q", want)
		}
	}
}
