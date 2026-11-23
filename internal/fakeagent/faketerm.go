package fakeagent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// block is one content block of a prompt.
type block struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
	Resource struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"resource"`
}

// describeBlocks lists what a prompt carried: "text:<text>" and "resource:<uri>:<text>".
func describeBlocks(blocks []block) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if b.Type == "resource" {
			parts = append(parts, "resource:"+b.Resource.URI+":"+b.Resource.Text)
		} else {
			parts = append(parts, b.Type+":"+b.Text)
		}
	}
	return "blocks:" + strings.Join(parts, "|")
}

// termTurn runs "TERM <cmd>" or "TERMKILL <cmd>" through the client's terminal
// and says what it got back.
func (a *agent) termTurn(sid, text string) int {
	kill := strings.HasPrefix(text, "TERMKILL ")
	_, cmd, _ := strings.Cut(text, " ")
	res := a.call("terminal/create", map[string]any{"sessionId": sid, "command": "sh", "args": []string{"-c", cmd}})
	var created struct {
		TerminalID string `json:"terminalId"`
	}
	if len(res.Error) > 0 || json.Unmarshal(res.Result, &created) != nil {
		a.chunk(sid, "term error: "+string(res.Error))
		return 0
	}
	tid := created.TerminalID
	toolID := "term-" + tid
	a.update(sid, map[string]any{"sessionUpdate": "tool_call", "toolCallId": toolID, "title": cmd, "kind": "execute", "status": "in_progress",
		"content": []map[string]string{{"type": "terminal", "terminalId": tid}}})
	args := map[string]any{"sessionId": sid, "terminalId": tid}
	if kill {
		a.call("terminal/kill", args)
	}
	var exit struct {
		ExitCode *int   `json:"exitCode"`
		Signal   string `json:"signal"`
	}
	_ = json.Unmarshal(a.call("terminal/wait_for_exit", args).Result, &exit)
	var out struct {
		Output    string `json:"output"`
		Truncated bool   `json:"truncated"`
	}
	_ = json.Unmarshal(a.call("terminal/output", args).Result, &out)
	a.call("terminal/release", args)
	a.update(sid, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": toolID, "status": "completed"})
	code := "none"
	if exit.ExitCode != nil {
		code = fmt.Sprint(*exit.ExitCode)
	}
	lines := strings.Count(out.Output, "\n")
	if out.Output != "" && !strings.HasSuffix(out.Output, "\n") {
		lines++
	}
	a.chunk(sid, fmt.Sprintf("term: bytes=%d lines=%d exit=%s signal=%s truncated=%v compacted=%v",
		len(out.Output), lines, code, exit.Signal, out.Truncated, strings.Contains(out.Output, "[mooring:")))
	return len(out.Output)
}

// imageCount is how many image blocks a prompt carries. An image block counts only with a
// type and some data, so a test sees what the agent really got.
func imageCount(blocks []block) int {
	n := 0
	for _, b := range blocks {
		if b.Type == "image" && b.MimeType != "" && b.Data != "" {
			n++
		}
	}
	return n
}
