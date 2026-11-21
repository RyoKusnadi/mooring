package fakeagent

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type line struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

func (a *agent) file(sid string) string {
	dir := os.Getenv("FAKE_STATE")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "fake-session-"+sid+".json")
}

func (a *agent) open(sid string) {
	if _, err := os.Stat(a.file(sid)); err != nil {
		_ = os.WriteFile(a.file(sid), []byte("[]"), 0o600)
	}
}

func (a *agent) load(sid string) []line {
	var out []line
	if raw, err := os.ReadFile(a.file(sid)); err == nil {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func (a *agent) remember(sid, role, text string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	raw, _ := json.Marshal(append(a.load(sid), line{role, text}))
	_ = os.WriteFile(a.file(sid), raw, 0o600)
}

// replay sends the saved conversation the way a real agent does on session/load.
func (a *agent) replay(sid string) {
	for _, l := range a.load(sid) {
		kind := "agent_message_chunk"
		if l.Role == "user" {
			kind = "user_message_chunk"
		}
		a.update(sid, map[string]any{"sessionUpdate": kind, "content": map[string]string{"type": "text", "text": l.Text}})
	}
}
