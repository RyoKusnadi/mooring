package fakeagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// saveCwd keeps the folder a session was made in, for session/list.
func (a *agent) saveCwd(sid, cwd string) {
	_ = os.WriteFile(strings.TrimSuffix(a.file(sid), ".json")+".cwd", []byte(cwd), 0o600)
}

// list answers session/list: the saved sessions made in cwd (all when cwd is ""),
// titled by their first user line.
func (a *agent) list(cwd string) map[string]any {
	out := []map[string]any{}
	paths, _ := filepath.Glob(a.file("*"))
	for _, p := range paths {
		sid := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "fake-session-"), ".json")
		raw, err := os.ReadFile(strings.TrimSuffix(p, ".json") + ".cwd")
		if err != nil || (cwd != "" && string(raw) != cwd) {
			continue
		}
		s := map[string]any{"sessionId": sid, "cwd": string(raw)}
		for _, l := range a.load(sid) {
			if l.Role == "user" {
				s["title"] = l.Text
				break
			}
		}
		if fi, err := os.Stat(p); err == nil {
			s["updatedAt"] = fi.ModTime().UTC().Format(time.RFC3339)
		}
		out = append(out, s)
	}
	return map[string]any{"sessions": out}
}
