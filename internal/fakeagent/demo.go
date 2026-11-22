package fakeagent

import (
	"encoding/json"
	"time"
)

// demoPrompt is the one prompt the "demo" turn answers. The documentation pictures use it so the
// chat reads like a real one: a short reply in pieces, a permission request for `npm test`, then
// a closing line. Nothing else depends on it.
const demoPrompt = "Run the tests and fix the failing one"

func (a *agent) demoTurn(sid string) string {
	for _, part := range []string{"I'll run the test suite ", "first, then look at ", "whatever fails.\n\n"} {
		a.chunk(sid, part)
		time.Sleep(350 * time.Millisecond)
	}
	a.update(sid, map[string]any{"sessionUpdate": "tool_call", "toolCallId": "d1", "title": "npm test", "kind": "execute", "status": "pending", "rawInput": map[string]string{"command": "npm test"}})
	res := a.call("session/request_permission", map[string]any{
		"sessionId": sid,
		"toolCall":  map[string]any{"toolCallId": "d1", "title": "npm test", "kind": "execute", "rawInput": map[string]string{"command": "npm test"}},
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
	if out.Outcome.Outcome != "selected" {
		return "cancelled"
	}
	if out.Outcome.OptionID != "allow-once" {
		a.update(sid, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "d1", "status": "failed"})
		a.chunk(sid, "I did not run the tests.")
		return "end_turn"
	}
	a.update(sid, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "d1", "status": "completed"})
	for _, part := range []string{"One test failed: ", "greet.test.ts expected ", "\"Hello, Ada!\" but got the new welcome text. ", "I updated the test and it passes now."} {
		a.chunk(sid, part)
		time.Sleep(300 * time.Millisecond)
	}
	return "end_turn"
}
