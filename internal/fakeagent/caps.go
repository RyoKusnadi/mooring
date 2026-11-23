package fakeagent

import (
	"encoding/json"
	"os"
	"strings"
)

// endSession logs a close or delete to FAKE_CALLS; a delete also removes the saved session.
func (a *agent) endSession(method, sid string) {
	a.logCall(method)
	if method == "session/delete" && sid != "" {
		_ = os.Remove(a.file(sid))
	}
}

// logDirs writes the extra folders a request names to FAKE_CALLS.
func (a *agent) logDirs(method string, raw json.RawMessage) {
	var p struct {
		Dirs []string `json:"additionalDirectories"`
	}
	if json.Unmarshal(raw, &p) == nil && len(p.Dirs) > 0 {
		a.logCall(method + " dirs " + strings.Join(p.Dirs, ","))
	}
}

// logSystemPrompt writes the systemPrompt a session request carries to FAKE_CALLS as
// "<method> systemPrompt <json>", the value as it arrived, so a test can compare bytes.
func (a *agent) logSystemPrompt(method string, raw json.RawMessage) {
	var p struct {
		SystemPrompt json.RawMessage `json:"systemPrompt"`
	}
	if json.Unmarshal(raw, &p) == nil && len(p.SystemPrompt) > 0 {
		a.logCall(method + " systemPrompt " + string(p.SystemPrompt))
	}
}

// initializeResult is the fake's `initialize` reply. FAKE_LOAD and FAKE_RESUME
// advertise those, FAKE_CLOSE keeps naming one session method, and FAKE_CAPS is a
// comma list of: list, close, delete, fork, systemPrompt, additionalDirectories
// (session methods), mcpAcp, audio and image (prompt capabilities). Other names are ignored.
func initializeResult() map[string]any {
	sc := map[string]any{}
	if os.Getenv("FAKE_RESUME") == "1" {
		sc["resume"] = map[string]any{}
	}
	if v := os.Getenv("FAKE_CLOSE"); v != "" {
		sc[v] = map[string]any{} // "close" or "delete"
	}
	mcp := map[string]bool{"http": true}
	prompt := map[string]bool{"embeddedContext": os.Getenv("FAKE_EMBED") == "1"}
	for _, k := range strings.Split(os.Getenv("FAKE_CAPS"), ",") {
		switch k = strings.TrimSpace(k); k {
		case "list", "close", "delete", "fork", "systemPrompt", "additionalDirectories":
			sc[k] = map[string]any{}
		case "mcpAcp":
			mcp["acp"] = true
		case "audio":
			prompt["audio"] = true
		case "image":
			prompt["image"] = true
		}
	}
	return map[string]any{
		"protocolVersion": 1, "authMethods": []any{},
		"agentCapabilities": map[string]any{
			"loadSession": os.Getenv("FAKE_LOAD") == "1", "sessionCapabilities": sc,
			"promptCapabilities": prompt, "mcpCapabilities": mcp,
		},
	}
}
