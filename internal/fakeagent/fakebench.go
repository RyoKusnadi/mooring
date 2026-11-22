package fakeagent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// isJob reports whether a session works in a folder named "jobs" (a small job).
func (a *agent) isJob(sid string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	cwd := a.cwds[sid]
	return cwd != "" && filepath.Base(filepath.Clean(cwd)) == "jobs"
}

func jobReply(prompt string) string {
	if strings.Contains(prompt, "SLOW") {
		select {}
	}
	if strings.Contains(prompt, "commit message") {
		return "fake: commit subject"
	}
	return "Fake title"
}

// logCall appends a line to the file named by FAKE_CALLS, if any.
func (a *agent) logCall(line string) {
	path := os.Getenv("FAKE_CALLS")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintln(f, line)
	_ = f.Close()
}

// benchTurn is "BENCH <task>[.part] [on|off]": it runs `make noisy` through the
// client's terminal, applies the canned fix (the script bench/fix/<task>.sh found
// through MOORING_BENCH_DIR) in the session's folder, and returns usage that grows
// with the bytes the terminal gave back, as a model's input would.
func (a *agent) benchTurn(sid, text string) map[string]int {
	fields := strings.Fields(text)
	task, mode := "", "off"
	if len(fields) > 1 {
		task = fields[1]
	}
	if len(fields) > 2 {
		mode = fields[2]
	}
	n := a.termTurn(sid, "TERM make noisy")
	a.mu.Lock()
	cwd := a.cwds[sid]
	a.mu.Unlock()
	fix := "no MOORING_BENCH_DIR"
	if dir := os.Getenv("MOORING_BENCH_DIR"); dir != "" && cwd != "" {
		out, err := exec.Command("sh", filepath.Join(dir, "fix", task+".sh"), cwd).CombinedOutput()
		fix = "ok"
		if err != nil {
			fix = "failed: " + strings.TrimSpace(string(out))
		}
	}
	a.chunk(sid, fmt.Sprintf("bench: %s mode=%s bytes=%d fix=%s", task, mode, n, fix))
	in := 1000 + n/4
	return map[string]int{"inputTokens": in, "outputTokens": 200, "totalTokens": in + 200}
}

func modelOption(cur string) map[string]any {
	return map[string]any{"id": "model", "name": "Model", "category": "model", "type": "select", "currentValue": cur,
		"options": []map[string]string{{"value": "fast", "name": "Fast"}, {"value": "deep", "name": "Deep"}}}
}
