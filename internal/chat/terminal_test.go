package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// lastAgent is the text of the newest agent message.
func lastAgent(c *Chat) string {
	msgs := c.messages()
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Kind == KindAgent {
			return msgs[i].Text
		}
	}
	return ""
}

// ask sends a prompt, waits for its turn to end and returns the agent's last text.
func (r *rig) ask(c *Chat, text string) string {
	r.t.Helper()
	before := c.turnsDone()
	r.prompt(c, text)
	r.waitTurns(c, before+1)
	return lastAgent(c)
}

func (c *Chat) turnsDone() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.usages)
}

func TestTerminalRunsCommand(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	got := r.ask(c, "TERM echo hi; echo oops >&2; exit 3")
	if !has(got, "bytes=8 lines=2 exit=3") {
		t.Fatalf("reply: %q", got)
	}
	var tool *Message
	for _, m := range c.messages() {
		if m.Kind == KindTool {
			tool = &m
		}
	}
	if tool == nil || len(tool.Outputs) != 1 || tool.Outputs[0].Lines != 2 || tool.Outputs[0].Compacted {
		t.Fatalf("tool message: %+v", tool)
	}
	raw, err := r.m.loadOutput("c1", tool.Outputs[0].ID)
	if err != nil || !has(raw, "hi") || !has(raw, "oops") {
		t.Fatalf("raw output %q: %v", raw, err)
	}
	if st, _ := r.m.Stats("c1"); st.TerminalCalls != 4 {
		t.Fatalf("terminal calls = %d", st.TerminalCalls)
	}
}

func TestCompactFilterKeepsRawOutput(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	if _, err := r.run("chat.setFilters", map[string]any{"id": "c1", "filters": []string{"nope"}}); err == nil || !has(err.Error(), "BAD_REQUEST") {
		t.Fatalf("bad filter: %v", err)
	}
	v, err := r.run("chat.setFilters", map[string]any{"id": "c1", "filters": []string{"compact", "compact"}})
	if err != nil || len(v.(Info).Spec.Filters) != 1 {
		t.Fatalf("setFilters: %v %v", v, err)
	}
	got := r.ask(c, "TERM seq 1 1000")
	if !has(got, "lines=121 exit=0") || !has(got, "compacted=true") {
		t.Fatalf("compact on: %q", got)
	}
	var ref OutputRef
	for _, m := range c.messages() {
		if len(m.Outputs) > 0 {
			ref = m.Outputs[0]
		}
	}
	if ref.Lines != 1000 || ref.KeptLines != 120 || !ref.Compacted {
		t.Fatalf("ref: %+v", ref)
	}
	if raw, _ := r.m.loadOutput("c1", ref.ID); strings.Count(raw, "\n") != 1000 {
		t.Fatalf("raw output has %d lines", strings.Count(raw, "\n"))
	}
	if _, err := r.run("chat.setFilters", map[string]any{"id": "c1", "filters": []string{}}); err != nil {
		t.Fatal(err)
	}
	r.waitTurns(c, 1)
	if got := r.ask(c, "TERM seq 1 1000"); !has(got, "lines=1000") || !has(got, "compacted=false") {
		t.Fatalf("compact off: %q", got)
	}
}

// waitTurns waits until n turns reported usage and the chat is idle.
func (r *rig) waitTurns(c *Chat, n int) {
	r.t.Helper()
	waitFor(r.t, "turns", func() bool { return c.turnsDone() >= n && c.Info().State == StateIdle })
}

func TestTerminalCreateChecks(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	run := c.cur
	_, err := run.terminal("terminal/create", json.RawMessage(`{"command":"true","cwd":"/"}`))
	var re *rpcError
	if !errors.As(err, &re) || !has(string(re.Data), "OUTSIDE_ROOT") {
		t.Fatalf("outside folder: %v", err)
	}
	sub := filepath.Join(r.dir, "sub")
	_ = os.Mkdir(sub, 0o755)
	if _, err := run.terminal("terminal/create", json.RawMessage(fmt.Sprintf(`{"command":"pwd","cwd":%q,"env":[{"name":"X","value":"1"}]}`, sub))); err != nil {
		t.Fatalf("folder inside: %v", err)
	}
	for i := 1; i < maxTerminals; i++ {
		if _, err := run.terminal("terminal/create", json.RawMessage(`{"command":"sleep","args":["30"]}`)); err != nil {
			t.Fatalf("terminal %d: %v", i, err)
		}
	}
	_, err = run.terminal("terminal/create", json.RawMessage(`{"command":"sleep","args":["30"]}`))
	if !errors.As(err, &re) || !has(string(re.Data), "LIMIT_REACHED") {
		t.Fatalf("ninth terminal: %v", err)
	}
	if _, err := run.terminal("terminal/output", json.RawMessage(`{"terminalId":"nope"}`)); err == nil {
		t.Fatal("unknown terminal accepted")
	}
}

func TestOutputByteLimitCutsFromStart(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	v, err := c.cur.terminal("terminal/create", json.RawMessage(`{"command":"sh","args":["-c","printf abcdefghij"],"outputByteLimit":4}`))
	if err != nil {
		t.Fatal(err)
	}
	id := v.(map[string]string)["terminalId"]
	if _, err := c.cur.terminal("terminal/wait_for_exit", json.RawMessage(`{"terminalId":"`+id+`"}`)); err != nil {
		t.Fatal(err)
	}
	out, err := c.cur.terminal("terminal/output", json.RawMessage(`{"terminalId":"`+id+`"}`))
	m := out.(map[string]any)
	if err != nil || m["output"] != "ghij" || m["truncated"] != true {
		t.Fatalf("output: %v %v", m, err)
	}
}

func pidFrom(t *testing.T, file string) int {
	t.Helper()
	var pid int
	waitFor(t, "pid file", func() bool {
		b, _ := os.ReadFile(file)
		n, err := strconv.Atoi(strings.TrimSpace(string(b)))
		pid = n
		return err == nil && n > 0
	})
	return pid
}

func TestCancelAndStopKillTerminals(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	pidFile := filepath.Join(t.TempDir(), "pid")
	r.prompt(c, "TERM echo $$ > "+pidFile+"; sleep 60")
	pid := pidFrom(t, pidFile)
	if !alive(pid) {
		t.Fatal("command is not running")
	}
	_, _ = r.run("chat.cancel", map[string]any{"id": "c1"})
	waitFor(t, "command killed by cancel", func() bool { return !alive(pid) })
	r.waitTurns(c, 1)

	_ = os.Remove(pidFile)
	r.prompt(c, "TERM echo $$ > "+pidFile+"; sleep 60")
	pid = pidFrom(t, pidFile)
	if _, err := r.run("chat.close", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "command killed by stop", func() bool { return !alive(pid) })
}

func TestTermKillReportsSignal(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	start := time.Now()
	got := r.ask(c, "TERMKILL sleep 30")
	if !has(got, "exit=none signal=killed") || time.Since(start) > 5*time.Second {
		t.Fatalf("reply %q after %v", got, time.Since(start))
	}
}

func TestSlowTurnIgnoringCancelStopsAgent(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	r.prompt(c, "SLOW")
	waitFor(t, "running", func() bool { return c.Info().State == StateRunning })
	_, _ = r.run("chat.cancel", map[string]any{"id": "c1"})
	r.waitState(c, StateStopped)
}

// A command the agent asks for runs with the environment the agent was launched
// with (the catalog entry's variables), and the request's own variables win.
func TestTerminalUsesAgentLaunchEnv(t *testing.T) {
	r := newRig(t, "PROBE_AGENT_VAR=from-launch")
	c := r.start("c1")
	if got := r.ask(c, "TERM printenv PROBE_AGENT_VAR"); !has(got, "bytes=12 ") {
		t.Fatalf("reply: %q", got)
	}
	term, err := startTerm("sh", []string{"-c", "printenv A; printenv B"}, t.TempDir(), []string{"A=launch", "B=launch"}, []string{"B=request"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	<-term.done
	if out := string(term.buf); out != "launch\nrequest\n" {
		t.Fatalf("output %q", out)
	}
}
