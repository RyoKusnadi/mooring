package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunDispatch(t *testing.T) {
	r := newRig(t)
	out, err := r.cmd("toolServer.save", r.stdioArgs("via run", nil))
	if err != nil {
		t.Fatal(err)
	}
	var ts ToolServer
	if json.Unmarshal(out, &ts) != nil || ts.ID == "" || ts.Name != "via run" {
		t.Fatalf("save answer: %s", out)
	}
	if _, err := r.cmd("toolServer.enable", map[string]any{"id": ts.ID, "enabled": true}); err == nil || !strings.Contains(err.Error(), "CHECK_FAILED") {
		t.Fatalf("enable before a check: %v", err)
	}
	if _, err := r.cmd("toolServer.check", map[string]any{"id": ts.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.cmd("toolServer.enable", map[string]any{"id": ts.ID, "enabled": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.cmd("toolServer.remove", map[string]any{"id": ts.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.cmd("toolServer.nope", map[string]any{}); err == nil {
		t.Fatal("unknown command accepted")
	}
	if !Handles("toolServer.check") || Handles("chat.start") {
		t.Fatal("Handles")
	}
}
