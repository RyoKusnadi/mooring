package fakeagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Account behaviour. FAKE_CONFIG is the agent's settings folder, the way the real
// adapter has CLAUDE_CONFIG_DIR. Unset: the agent's own login, always signed in.
// Set: signed in only when the file "signed-in" is in that folder; otherwise a
// prompt gets the same answer as the real adapter, an authentication error
// (JSON-RPC -32000) after an auth/status_update notification.

func (a *agent) signedOut() bool {
	dir := os.Getenv("FAKE_CONFIG")
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "signed-in"))
	return err != nil
}

func (a *agent) failAuth(id json.RawMessage) {
	a.send(map[string]any{"jsonrpc": "2.0", "method": "auth/status_update", "params": map[string]string{"state": "unauthenticated"}})
	a.fail(id, -32000, "Authentication required")
}

// rateWindow sends a usage_update that carries one rate-limit window, as the
// Claude adapter does in _meta["_claude/rateLimit"]. utilization is a fraction.
func (a *agent) rateWindow(sid, status, kind string, utilization float64) {
	a.update(sid, map[string]any{"sessionUpdate": "usage_update", "used": 1200, "size": 200000,
		"_meta": map[string]any{"_claude/rateLimit": map[string]any{
			"status": status, "rateLimitType": kind, "utilization": utilization,
			"resetsAt": time.Now().Add(time.Hour).Unix()}}})
}

// rate handles "RATE [window [percent]]": one window at the given use, then a short answer.
func (a *agent) rate(sid, text string) {
	kind, pct := "five_hour", 42.0
	if f := strings.Fields(text); len(f) > 1 {
		kind = f[1]
		if len(f) > 2 {
			if v, err := strconv.ParseFloat(f[2], 64); err == nil {
				pct = v
			}
		}
	}
	status := "allowed"
	if pct >= 100 {
		status = "rejected"
	} else if pct >= 80 {
		status = "allowed_warning"
	}
	a.rateWindow(sid, status, kind, pct/100)
	a.chunk(sid, fmt.Sprintf("rate:%s:%v", kind, pct))
}

// limit handles LIMIT (a rejected window, then an error) and LIMITTEXT (the error
// alone): the turn fails for the account's limit.
func (a *agent) limit(id json.RawMessage, sid, text string) {
	if text == "LIMIT" {
		a.rateWindow(sid, "rejected", "five_hour", 1)
	}
	a.fail(id, -32603, "Claude usage limit reached. Your limit resets in 1 hour.")
}
