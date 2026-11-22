package chat

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func saveSettings(t *testing.T, r *rig, patch string) {
	t.Helper()
	if _, err := r.set.Save(json.RawMessage(patch)); err != nil {
		t.Fatalf("save settings %s: %v", patch, err)
	}
}

func promptLines(t *testing.T, calls string) []string {
	t.Helper()
	raw, _ := os.ReadFile(calls)
	var out []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.Contains(l, " systemPrompt ") {
			out = append(out, l)
		}
	}
	return out
}

func restart(t *testing.T, r *rig, c *Chat) *Chat {
	t.Helper()
	if _, err := r.run("chat.close", map[string]any{"id": c.id}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateStopped)
	if _, err := r.run("chat.resume", map[string]any{"id": c.id}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
	return c
}

func TestInstructionsSentOnNewAndResumeWithSameBytes(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	r := newRig(t, "FAKE_CAPS=systemPrompt", "FAKE_RESUME=1", "FAKE_CALLS="+calls)
	saveSettings(t, r, `{"agentInstructions":true,"projectInstructions":{"p1":"  Use tabs.  "}}`)
	c := r.start("a")
	if !c.Info().Instructed {
		t.Fatal("instructed is false after a start that sent the text")
	}
	restart(t, r, c)
	r.boot() // a server restart: the text comes back from the database
	c2 := r.m.get("a")
	if _, err := r.run("chat.resume", map[string]any{"id": "a"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c2, StateIdle)
	got := promptLines(t, calls)
	if len(got) != 3 || !strings.HasPrefix(got[0], "session/new systemPrompt ") || !strings.HasPrefix(got[1], "session/resume systemPrompt ") {
		t.Fatalf("calls: %q", got)
	}
	body := strings.TrimPrefix(got[0], "session/new systemPrompt ")
	for i, l := range got[1:] {
		if !strings.HasSuffix(l, " systemPrompt "+body) {
			t.Fatalf("call %d differs:\n%s\nwant body %s", i+1, l, body)
		}
	}
	var sp struct {
		Mode    string `json:"mode"`
		Content []struct{ Type, Text string }
	}
	if err := json.Unmarshal([]byte(body), &sp); err != nil || sp.Mode != "append" || len(sp.Content) != 1 || sp.Content[0].Type != "text" {
		t.Fatalf("shape: %s (%v)", body, err)
	}
	if want := instructionsText + "\n\nUse tabs."; sp.Content[0].Text != want {
		t.Fatalf("text: %q", sp.Content[0].Text)
	}
}

func TestInstructionsNeedTheCapabilityAndNoticeOnce(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	r := newRig(t, "FAKE_RESUME=1", "FAKE_CALLS="+calls)
	saveSettings(t, r, `{"agentInstructions":true}`)
	c := r.start("a")
	restart(t, r, c)
	if got := promptLines(t, calls); len(got) != 0 {
		t.Fatalf("sent without the capability: %q", got)
	}
	if c.Info().Instructed {
		t.Fatal("instructed without a send")
	}
	n := 0
	for _, m := range c.messages() {
		if m.Kind == KindNotice && m.Text == "Fake does not take extra instructions, so this chat starts without them." {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("notice count %d, want 1", n)
	}
}

func TestInstructionsOffMeansNoText(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	r := newRig(t, "FAKE_CAPS=systemPrompt", "FAKE_CALLS="+calls)
	saveSettings(t, r, `{"agentInstructions":false}`)
	c := r.start("a")
	if got := promptLines(t, calls); len(got) != 0 || c.Info().Instructed || c.find(KindNotice) != nil {
		t.Fatalf("off still sent or noticed: %q", got)
	}
}

func TestChangingTheSettingLeavesExistingChatsAlone(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	r := newRig(t, "FAKE_CAPS=systemPrompt", "FAKE_RESUME=1", "FAKE_CALLS="+calls)
	saveSettings(t, r, `{"agentInstructions":true}`)
	on := r.start("on")
	saveSettings(t, r, `{"agentInstructions":false}`)
	off := r.start("off")
	saveSettings(t, r, `{"agentInstructions":true,"projectInstructions":{"p1":"New text."}}`)
	restart(t, r, on)
	restart(t, r, off)
	got := promptLines(t, calls)
	if len(got) != 2 || !strings.HasPrefix(got[0], "session/new ") || !strings.HasPrefix(got[1], "session/resume ") {
		t.Fatalf("calls: %q", got)
	}
	if strings.Contains(got[1], "New text.") || !strings.HasSuffix(got[1], got[0][len("session/new systemPrompt "):]) {
		t.Fatalf("a running chat's text changed:\n%s\n%s", got[0], got[1])
	}
	if off.instructions != "" {
		t.Fatal("a chat made with the setting off got text on resume")
	}
}

func TestProjectInstructionsCap(t *testing.T) {
	r := newRig(t)
	long := strings.Repeat("é", 2001)
	if _, err := r.set.Save(json.RawMessage(`{"projectInstructions":{"p1":"` + long + `"}}`)); err == nil || !has(err.Error(), "2000") {
		t.Fatalf("2001 characters accepted: %v", err)
	}
	ok := strings.Repeat("é", 2000)
	got, err := r.set.Save(json.RawMessage(`{"projectInstructions":{"p1":"` + ok + `"}}`))
	if err != nil || got.ProjectInstructions["p1"] != ok {
		t.Fatalf("2000 characters refused: %v", err)
	}
	got, err = r.set.Save(json.RawMessage(`{"projectInstructions":{"p1":""}}`))
	if err != nil || len(got.ProjectInstructions) != 0 {
		t.Fatalf("empty text did not clear: %+v %v", got, err)
	}
}

func TestInstructionTextIsNeverLogged(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r := newRig(t, "FAKE_CAPS=systemPrompt", "FAKE_RESUME=1")
	r.m.log, r.set.Log = log, log
	saveSettings(t, r, `{"agentInstructions":true,"projectInstructions":{"p1":"SECRET-PROJECT-WORDS"}}`)
	c := r.start("a")
	restart(t, r, c)
	r.prompt(c, "hello")
	waitFor(t, "idle", func() bool { return c.Info().State == StateIdle && c.find(KindAgent) != nil })
	out := buf.String()
	if strings.Contains(out, "SECRET-PROJECT-WORDS") || strings.Contains(out, "browser workspace") {
		t.Fatalf("instruction text in the log:\n%s", out)
	}
}
