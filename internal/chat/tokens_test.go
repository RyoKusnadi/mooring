package chat

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutputsCapAndCascade(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	for i := 0; i < maxOutputs+5; i++ {
		ref := OutputRef{ID: fmt.Sprintf("o%d", i), Lines: 1, Bytes: 2}
		if err := c.saveOutput("t", ref, "x\n", nil); err != nil {
			t.Fatal(err)
		}
	}
	count := func() (n int) {
		_ = r.db.QueryRow(`SELECT COUNT(*) FROM chat_outputs WHERE chat_id = 'c1'`).Scan(&n)
		return n
	}
	if count() != maxOutputs {
		t.Fatalf("kept %d outputs", count())
	}
	if _, err := r.m.loadOutput("c1", "o0"); err == nil {
		t.Fatal("oldest output kept")
	}
	if _, err := r.m.loadOutput("c1", "o54"); err != nil {
		t.Fatalf("newest output lost: %v", err)
	}
	if _, err := r.run("chat.remove", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	if count() != 0 {
		t.Fatal("outputs outlived the chat")
	}
}

func TestOutputHandler(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	if err := c.saveOutput("t", OutputRef{ID: "o1", Lines: 1, Bytes: 3}, "hi\n", nil); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/chats/{id}/outputs/{outputId}", r.m.OutputHandler())
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	if w := get("/api/chats/c1/outputs/o1"); w.Code != 200 || w.Body.String() != "hi\n" || !has(w.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("output: %d %q", w.Code, w.Body.String())
	}
	for _, p := range []string{"/api/chats/c1/outputs/zz", "/api/chats/nope/outputs/o1", "/api/chats/c1/outputs/a.b"} {
		if w := get(p); w.Code != 404 {
			t.Fatalf("%s: %d", p, w.Code)
		}
	}
}

func TestPromptResultUsageKept(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	r.ask(c, "hello")
	r.ask(c, "again")
	st, err := r.m.Stats("c1")
	if err != nil || len(st.Turns) != 2 {
		t.Fatalf("stats: %+v %v", st, err)
	}
	if u := st.Turns[1]; u.Turn != 2 || u.InputTokens != 1000 || u.OutputTokens != 50 || u.TotalTokens != 1050 || !has(string(u.Raw), `"totalTokens":1050`) {
		t.Fatalf("usage: %+v", u)
	}
	if st.AgentTitled || st.TerminalCalls != 0 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestAgentTitleIsNoted(t *testing.T) {
	for env, want := range map[string]bool{"FAKE_TITLE=1": true, "FAKE_NOTITLE=1": false} {
		r := newRig(t, "FAKE_TITLE=1", env)
		c := r.start("c1")
		r.ask(c, "hello")
		if st, _ := r.m.Stats("c1"); st.AgentTitled != want {
			t.Fatalf("%s: titled = %v", env, st.AgentTitled)
		}
		if want && c.Info().Title != "Fake chat title" {
			t.Fatalf("title %q", c.Info().Title)
		}
	}
}

func TestAttachmentsAsResourcesOrText(t *testing.T) {
	file := "a.go"
	for _, embed := range []bool{true, false} {
		env := []string{}
		if embed {
			env = append(env, "FAKE_EMBED=1")
		}
		r := newRig(t, env...)
		if err := os.WriteFile(filepath.Join(r.dir, file), []byte("package a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		c := r.start("c1")
		if c.Info().Caps.EmbeddedContext != embed {
			t.Fatalf("caps: %+v", c.Info().Caps)
		}
		r.promptWith(c, "BLOCKS", []map[string]any{{"kind": "selection", "path": file, "from": 3, "to": 4, "text": "func f() {}\n"}})
		r.waitTurns(c, 1)
		got := lastAgent(c)
		if embed && (!has(got, "text:BLOCKS|resource:file://") || !has(got, "a.go#L3-4:func f() {}")) {
			t.Fatalf("embedded: %q", got)
		}
		if !embed && !has(got, "text:BLOCKS\n\na.go, lines 3 to 4:\n```\nfunc f() {}\n```") {
			t.Fatalf("fenced: %q", got)
		}
		var user *Message
		for _, m := range c.messages() {
			if m.Kind == KindUser {
				user = &m
			}
		}
		if user == nil || len(user.Attachments) != 1 || user.Attachments[0].Path != file || user.Attachments[0].From != 3 {
			t.Fatalf("user message: %+v", user)
		}
	}
}

func (r *rig) promptWith(c *Chat, text string, atts []map[string]any) {
	r.t.Helper()
	if _, err := r.run("chat.prompt", map[string]any{"id": c.id, "text": text, "attachments": atts}); err != nil {
		r.t.Fatalf("prompt: %v", err)
	}
}

func TestAttachmentLimitsAndOutputs(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	sel := map[string]any{"kind": "selection", "path": "a.go", "from": 1, "to": 1, "text": "x"}
	bad := map[string][]map[string]any{
		"outside": {{"kind": "selection", "path": "/etc/passwd", "from": 1, "to": 1, "text": "x"}},
		"nine":    {sel, sel, sel, sel, sel, sel, sel, sel, sel},
		"big":     {{"kind": "selection", "path": "a.go", "from": 1, "to": 2, "text": strings.Repeat("x", maxAttachBytes+1)}},
		"range":   {{"kind": "selection", "path": "a.go", "from": 5, "to": 2, "text": "x"}},
		"kind":    {{"kind": "image"}},
		"gone":    {{"kind": "output", "outputId": "nope"}},
	}
	for name, atts := range bad {
		if _, err := r.run("chat.prompt", map[string]any{"id": "c1", "text": "hi", "attachments": atts}); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	if len(c.messages()) != 0 {
		t.Fatal("a refused prompt left a message")
	}
	r.ask(c, "TERM seq 1 500")
	var id string
	for _, m := range c.messages() {
		if len(m.Outputs) > 0 {
			id = m.Outputs[0].ID
		}
	}
	r.promptWith(c, "BLOCKS", []map[string]any{{"kind": "output", "outputId": id}})
	r.waitTurns(c, 2)
	if got := lastAgent(c); !has(got, "command output:\n```\n1\n2\n") || !has(got, "[mooring:") {
		t.Fatalf("output attachment: %.200q", got)
	}
}

func TestSummarizeThroughAgentAndFromMessages(t *testing.T) {
	r := newRig(t)
	c := r.start("c1")
	r.ask(c, "hello")
	v, err := r.run("chat.summarize", map[string]any{"id": "c1"})
	m := v.(map[string]string)
	if err != nil || m["source"] != "agent" || !has(m["text"], "Goal: build it") {
		t.Fatalf("agent summary: %v %v", v, err)
	}
	var job *Message
	for _, msg := range c.messages() {
		if msg.Job == "summary" {
			job = &msg
		}
	}
	if job == nil || job.Kind != KindUser || !strings.HasPrefix(job.Text, "Summarise this chat") {
		t.Fatalf("job message: %+v", job)
	}
	if c.Info().Title != "hello" {
		t.Fatalf("the job changed the title: %q", c.Info().Title)
	}

	r.ask(c, "TERM echo x")
	c.mu.Lock()
	c.msgs[len(c.msgs)-2].Locations = []byte(`[{"path":"/p/main.go"}]`)
	c.mu.Unlock()
	if _, err := r.run("chat.close", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateStopped)
	v, err = r.run("chat.summarize", map[string]any{"id": "c1"})
	m = v.(map[string]string)
	if err != nil || m["source"] != "messages" || !has(m["text"], "First request:\nhello") || !has(m["text"], "/p/main.go") || has(m["text"], "Summarise this chat so") {
		t.Fatalf("message summary: %v %v", v, err)
	}
}

func TestStartFromChatWithContext(t *testing.T) {
	for _, embed := range []bool{true, false} {
		env := []string{}
		if embed {
			env = append(env, "FAKE_EMBED=1")
		}
		r := newRig(t, env...)
		r.start("old")
		start := func(a map[string]any) error {
			a["projectId"] = r.project
			_, err := r.run("chat.start", a)
			return err
		}
		if err := start(map[string]any{"id": "x1", "fromChat": "nope", "context": "s"}); err == nil || !has(err.Error(), "NOT_FOUND") {
			t.Fatalf("missing chat: %v", err)
		}
		if err := start(map[string]any{"id": "x2", "fromChat": "old"}); err == nil || !has(err.Error(), "BAD_REQUEST") {
			t.Fatalf("no context: %v", err)
		}
		if err := start(map[string]any{"id": "x3", "context": strings.Repeat("a", maxContextBytes+1)}); err == nil {
			t.Fatal("long context accepted")
		}
		if err := start(map[string]any{"id": "new", "fromChat": "old", "context": "Goal: ship.\nNext: test."}); err != nil {
			t.Fatal(err)
		}
		c := r.m.get("new")
		r.waitState(c, StateIdle)
		if c.Info().FromChat != "old" || c.messages()[0].Kind != KindContext || c.messages()[0].FromChat != "old" {
			t.Fatalf("info %+v messages %+v", c.Info().FromChat, c.messages())
		}
		got := r.ask(c, "BLOCKS")
		if embed && !has(got, "resource:mooring://summary/old:Goal: ship.\nNext: test.|text:BLOCKS") {
			t.Fatalf("embedded: %q", got)
		}
		if !embed && !has(got, "text:Summary of an earlier chat:\n> Goal: ship.\n> Next: test.\n\nBLOCKS") {
			t.Fatalf("quoted: %q", got)
		}
		if got := r.ask(c, "BLOCKS"); has(got, "summary") {
			t.Fatalf("the summary was sent twice: %q", got)
		}
		r.m.Shutdown() // writes the last messages, as a real restart does; without it the save timer may not have fired
		r.boot()
		if again := r.m.get("new"); again.Info().FromChat != "old" || again.count() != 5 {
			t.Fatalf("after restart: %+v, %d messages", again.Info().FromChat, again.count())
		}
		r.m.Shutdown()
	}
}

func TestCompactDefaultSetting(t *testing.T) {
	r := newRig(t)
	if _, err := r.db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('compactDefault', 'true')`); err != nil {
		t.Fatal(err)
	}
	for id, spec := range map[string]map[string]any{"d1": nil, "d2": {"filters": []string{}}} {
		a := map[string]any{"id": id, "projectId": r.project}
		if spec != nil {
			a["spec"] = spec
		}
		v, err := r.run("chat.start", a)
		if err != nil {
			t.Fatal(err)
		}
		on := len(v.(Info).Spec.Filters) == 1
		if on != (spec == nil) {
			t.Fatalf("%s: filters %v", id, v.(Info).Spec.Filters)
		}
	}
}
