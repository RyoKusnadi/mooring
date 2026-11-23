package chat

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func capsOf(t *testing.T, reply string) caps {
	t.Helper()
	var ir initResult
	if err := json.Unmarshal([]byte(reply), &ir); err != nil {
		t.Fatal(err)
	}
	return ir.caps()
}

func TestCapsReadsEveryKey(t *testing.T) {
	got := capsOf(t, `{"authMethods":[{"id":"claude-ai-login"},{"id":"console-login"}],"agentCapabilities":{
		"loadSession":true,
		"promptCapabilities":{"image":true,"audio":true,"embeddedContext":true},
		"mcpCapabilities":{"http":true,"sse":true,"acp":true},
		"sessionCapabilities":{"list":{},"close":{},"delete":{},"fork":{},"resume":{},"systemPrompt":{},"additionalDirectories":{}}}}`)
	want := caps{Initialized: true, Load: true, Resume: true, Image: true, EmbeddedContext: true, MCPHTTP: true, MCPSSE: true, Audio: true, MCPACP: true,
		List: true, Close: true, Delete: true, Fork: true, SystemPrompt: true, AdditionalDirectories: true,
		AuthMethods: []string{"claude-ai-login", "console-login"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestCapsMissingMeansFalseAndUnknownKeysAreIgnored(t *testing.T) {
	got := capsOf(t, `{"agentCapabilities":{"sessionCapabilities":{"status":{},"title":{},"fork":null,"close":false}}}`)
	if want := (caps{Initialized: true, AuthMethods: []string{}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if got := capsOf(t, `{}`); got.Resume || got.AuthMethods == nil {
		t.Fatalf("empty reply: %+v", got)
	}
}

func TestCapsJunkValuesAreNotSupport(t *testing.T) {
	got := capsOf(t, `{"agentCapabilities":{"sessionCapabilities":{"list":true,"close":false,"delete":0,"fork":"yes","resume":[],"systemPrompt":null,"additionalDirectories":{"x":1}}}}`)
	if !got.List || !got.AdditionalDirectories || got.Close || got.Delete || got.Fork || got.Resume || got.SystemPrompt {
		t.Fatalf("got %+v", got)
	}
}

func TestCapsFromTheFakeAgent(t *testing.T) {
	r := newRig(t, "FAKE_CAPS=list,close,delete,fork,systemPrompt,additionalDirectories,mcpAcp,audio")
	c := r.start("c1")
	got := c.Info().Caps
	if !got.List || !got.Close || !got.Delete || !got.Fork || !got.SystemPrompt || !got.AdditionalDirectories || !got.MCPACP || !got.Audio {
		t.Fatalf("caps %+v", got)
	}
	if plain := newRig(t).start("c2").Info().Caps; plain.List || plain.MCPACP || plain.Fork {
		t.Fatalf("a fake without FAKE_CAPS advertises %+v", plain)
	}
}

func TestStreamedToolLineKeepsOneMessageWithTheFinalText(t *testing.T) {
	r := newRig(t, "FAKE_STREAM_TOOL=1")
	c := r.start("c1")
	r.prompt(c, "STREAM")
	waitFor(t, "partial output", func() bool {
		m := c.find(KindTool)
		return m != nil && m.Status == "in_progress" && strings.Contains(string(m.Content), "step 2 done")
	})
	r.waitState(c, StateIdle)
	var tools []Message
	for _, m := range c.messages() {
		if m.Kind == KindTool {
			tools = append(tools, m)
		}
	}
	if len(tools) != 1 || tools[0].Status != "completed" {
		t.Fatalf("tool messages %+v", tools)
	}
	c.mu.Lock()
	left := len(c.streams)
	c.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d streams kept after the call ended", left)
	}
	var blocks []struct {
		Content struct{ Text string } `json:"content"`
	}
	_ = json.Unmarshal(tools[0].Content, &blocks)
	want := "step 1 done\nstep 2 done\nstep 3 done\nstep 4 done\nstep 5 done\n"
	if len(blocks) != 1 || blocks[0].Content.Text != want {
		t.Fatalf("content %s", tools[0].Content)
	}
}

func TestTenPartialToolUpdatesPublishFewTimes(t *testing.T) {
	r := newRig(t)
	c, err := newChat(r.m, "s", "p1", r.dir, r.dir, Spec{Agent: AgentSlot{ID: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	before := hubPos(t, c)
	c.mu.Lock()
	status := "in_progress"
	c.toolLocked(update{ToolID: "b", Status: &status})
	for i := 0; i < 10; i++ {
		c.toolLocked(update{ToolID: "b", Meta: map[string]json.RawMessage{"terminal_output_delta": json.RawMessage(`{"data":"line\n"}`)}})
	}
	done := "completed"
	whole := strings.Repeat("line\n", 10)
	c.toolLocked(update{ToolID: "b", Status: &done, Meta: map[string]json.RawMessage{"terminal_output": json.RawMessage(`{"data":` + string(mustJSON(whole)) + `}`)},
		Content: json.RawMessage(`[{"type":"terminal","terminalId":"b"}]`)})
	c.mu.Unlock()
	time.Sleep(2 * streamEvery)
	if got := hubPos(t, c) - before; got > 6 {
		t.Fatalf("%d publishes for 12 updates (an unthrottled chat makes 13)", got)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.msgs) != 1 || c.msgs[0].Status != "completed" || !strings.Contains(string(c.msgs[0].Content), strings.ReplaceAll(whole, "\n", `\n`)) {
		t.Fatalf("messages %+v", c.msgs)
	}
}

// hubPos is how many events the chat's stream has numbered so far.
func hubPos(t *testing.T, c *Chat) uint64 {
	t.Helper()
	a, err := c.attach("", 0)
	if err != nil {
		t.Fatal(err)
	}
	c.hub.Detach(a.Sub)
	return a.Pos
}

func mustJSON(v any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}

func TestSilentCommandHasNoContentAndStopFreesStreams(t *testing.T) {
	r := newRig(t)
	c, err := newChat(r.m, "q", "p1", r.dir, r.dir, Spec{Agent: AgentSlot{ID: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	status, done := "in_progress", "completed"
	term := json.RawMessage(`[{"type":"terminal","terminalId":"b"}]`)
	c.mu.Lock()
	c.toolLocked(update{ToolID: "b", Status: &status, Content: term})
	c.toolLocked(update{ToolID: "b", Status: &done, Content: term, Meta: map[string]json.RawMessage{"terminal_output": json.RawMessage(`{"data":""}`)}})
	empty := len(c.msgs[0].Content) == 0
	c.mu.Unlock()
	if !empty {
		t.Fatalf("a silent command kept content %s", c.msgs[0].Content)
	}
	c.mu.Lock()
	c.toolLocked(update{ToolID: "w", Meta: map[string]json.RawMessage{"terminal_output_delta": json.RawMessage(`{"data":"a"}`)}})
	c.toolLocked(update{ToolID: "w", Meta: map[string]json.RawMessage{"terminal_output_delta": json.RawMessage(`{"data":"b"}`)}})
	pending := c.streams["w"] != nil && c.streams["w"].timer != nil
	c.mu.Unlock()
	if !pending {
		t.Fatal("expected a waiting publish")
	}
	c.stopAgent(nil, StateStopped, nil)
	c.mu.Lock()
	n := len(c.streams)
	c.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d streams after stop", n)
	}
	before := hubPos(t, c)
	time.Sleep(2 * streamEvery)
	if hubPos(t, c) != before {
		t.Fatal("published after the agent stopped")
	}
}
