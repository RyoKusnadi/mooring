package fakeagent

import (
	"encoding/json"
	"strings"
	"time"
)

// server is what the agent was given for one tool server; header values are not kept.
type server struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Command string `json:"command"`
	Headers []struct {
		Name string `json:"name"`
	} `json:"headers"`
}

// toolsAnswer lists the servers as name(kind,auth=yes|no): auth says an Authorization header was given.
func (a *agent) toolsAnswer(sid string) string {
	a.mu.Lock()
	list := a.servers[sid]
	a.mu.Unlock()
	parts := []string{}
	for _, sv := range list {
		kind, auth := sv.Type, "no"
		if kind == "" {
			kind = "stdio"
		}
		for _, h := range sv.Headers {
			if strings.EqualFold(h.Name, "Authorization") {
				auth = "yes"
			}
		}
		parts = append(parts, sv.Name+"("+kind+",auth="+auth+")")
	}
	return "tools:" + strings.Join(parts, ";")
}

func (a *agent) setServers(sid string, list []server) {
	a.mu.Lock()
	a.servers[sid] = list
	a.mu.Unlock()
}

func (a *agent) ask(sid string) string {
	a.update(sid, map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t1", "title": "npm test", "kind": "execute", "status": "pending", "rawInput": map[string]string{"command": "npm test"}})
	res := a.call("session/request_permission", map[string]any{
		"sessionId": sid,
		"toolCall":  map[string]any{"toolCallId": "t1", "title": "npm test", "kind": "execute", "rawInput": map[string]string{"command": "npm test"}},
		"options": []map[string]string{
			{"optionId": "allow-once", "name": "Allow", "kind": "allow_once"},
			{"optionId": "reject", "name": "Reject", "kind": "reject_once"}},
	})
	var out struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	_ = json.Unmarshal(res.Result, &out)
	switch {
	case out.Outcome.Outcome != "selected":
		return "cancelled"
	case out.Outcome.OptionID == "allow-once":
		a.update(sid, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "t1", "status": "completed"})
		a.chunk(sid, "ran it")
	default:
		a.update(sid, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "t1", "status": "failed"})
		a.chunk(sid, "skipped it")
	}
	return "end_turn"
}

func (a *agent) long(sid string, cancel chan struct{}) string {
	for i := 0; i < 500; i++ {
		select {
		case <-cancel:
			return "cancelled"
		case <-time.After(20 * time.Millisecond):
			a.chunk(sid, "tick ")
		}
	}
	return "end_turn"
}
