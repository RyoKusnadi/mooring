package chat

import "testing"

func TestShowcaseTurnHasMarkdownAndToolCalls(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	r.prompt(c, "SHOWCASE")
	r.waitState(c, StateIdle)
	var text string
	tools := 0
	for _, m := range c.messages() {
		switch m.Kind {
		case KindAgent:
			text += m.Text
		case KindTool:
			tools++
		}
	}
	if !has(text, "## Plan") || !has(text, "| File |") || !has(text, "```go") {
		t.Fatalf("reply: %q", text)
	}
	if tools != 2 {
		t.Fatalf("tool calls: %d", tools)
	}
}

func TestRenameKeepsTheTitleAgainstTheAgent(t *testing.T) {
	r := newRig(t, "FAKE_TITLE=1")
	c := r.start("c1")
	if _, err := r.run("chat.rename", map[string]any{"id": "c1", "title": "  My work  "}); err != nil {
		t.Fatal(err)
	}
	if got := c.Info().Title; got != "My work" {
		t.Fatalf("title: %q", got)
	}
	if _, err := r.run("chat.rename", map[string]any{"id": "c1", "title": " "}); err == nil {
		t.Fatal("an empty title was accepted")
	}
}
