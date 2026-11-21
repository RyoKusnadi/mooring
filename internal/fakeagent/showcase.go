package fakeagent

import (
	"strings"
	"time"
)

// showcase is the "SHOWCASE" turn: a reply in markdown (heading, list, table, code, a link), then an edit with
// its diff and a command with output. The chat page is judged against it.
func (a *agent) showcase(sid string) {
	for _, part := range []string{
		"## Plan\n\nI will fix the **parser** and keep `main.go` small.\n\n",
		"- Read the file\n- Change one line\n- Run the tests\n\n",
		"| File | Change |\n| --- | :---: |\n| `main.go` | edit |\n| `main_test.go` | none |\n\n",
		"```go\nfunc main() {\n\tprintln(\"hello\")\n}\n```\n\n",
		"See [the docs](https://example.com/docs) or `<script>alert(1)</script>` stays text.\n",
	} {
		a.chunk(sid, part)
		time.Sleep(15 * time.Millisecond)
	}
	a.update(sid, map[string]any{"sessionUpdate": "tool_call", "toolCallId": "e1", "title": "Edit main.go", "kind": "edit", "status": "completed",
		"locations": []map[string]any{{"path": "main.go"}},
		"content":   []map[string]any{{"type": "diff", "path": "main.go", "oldText": "println(\"hi\")\n", "newText": "println(\"hello\")\n"}}})
	lines := make([]string, 14)
	for i := range lines {
		lines[i] = "test line " + string(rune('A'+i)) + " ok"
	}
	a.update(sid, map[string]any{"sessionUpdate": "tool_call", "toolCallId": "c1", "title": "go test ./...", "kind": "execute", "status": "completed",
		"rawInput": map[string]string{"command": "go test ./..."},
		"content":  []map[string]any{{"type": "content", "content": map[string]any{"type": "text", "text": strings.Join(lines, "\n")}}}})
	a.chunk(sid, "Done. The tests pass.")
}
