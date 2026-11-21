package fakeagent

import (
	"fmt"
	"time"
)

// streamTool is the "STREAM" turn with FAKE_STREAM_TOOL=1: a Bash call the way
// claude-agent-acp 0.108 reports it to a client that sends `_meta.terminal_output_delta`.
// The call names a terminal, five updates append output, and the last one replaces it
// with the whole output and exits.
func (a *agent) streamTool(sid string) {
	const id = "b1"
	a.update(sid, map[string]any{"sessionUpdate": "tool_call", "toolCallId": id, "title": "make build", "kind": "execute", "status": "in_progress",
		"rawInput": map[string]string{"command": "make build"},
		"content":  []map[string]any{{"type": "terminal", "terminalId": id}},
		"_meta":    map[string]any{"terminal_info": map[string]string{"terminal_id": id}}})
	whole := ""
	for i := 1; i <= 5; i++ {
		part := fmt.Sprintf("step %d done\n", i)
		whole += part
		a.update(sid, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": id,
			"_meta": map[string]any{"terminal_output_delta": map[string]string{"terminal_id": id, "data": part}}})
		time.Sleep(250 * time.Millisecond)
	}
	a.update(sid, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": id, "status": "completed",
		"content": []map[string]any{{"type": "terminal", "terminalId": id}},
		"_meta": map[string]any{
			"terminal_output": map[string]string{"terminal_id": id, "data": whole},
			"terminal_exit":   map[string]any{"terminal_id": id, "exit_code": 0, "signal": nil}}})
	a.chunk(sid, "Build finished.")
}
