package chat

import (
	"strings"
	"testing"
)

func (r *rig) startIn(id, project string) *Chat {
	r.t.Helper()
	if _, err := r.run("chat.start", map[string]any{"id": id, "projectId": project}); err != nil {
		r.t.Fatalf("start: %v", err)
	}
	c := r.m.get(id)
	r.waitState(c, StateIdle)
	return c
}

func ids(hits []SearchHit) string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.ChatID)
	}
	return strings.Join(out, ",")
}

func TestSearchAcrossProjects(t *testing.T) {
	r := newRig(t)
	if _, err := r.db.Exec(`INSERT INTO projects (id, name, path, position, created_at) VALUES ('p2', 'q', ?, 1, 0)`, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	c1 := r.start("c1")
	r.ask(c1, "the zebra migration plan")
	c2 := r.startIn("c2", "p2")
	r.ask(c2, "quokka deploy")
	r.ask(c2, "and once more")
	c1.flush()
	c2.flush()
	search := func(q string) []SearchHit {
		t.Helper()
		hits, err := r.m.Search(q, 0)
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		return hits
	}
	if h := search("ZEBRA"); ids(h) != "c1" || !strings.Contains(h[0].Snippet, markOpen+"zebra"+markClose) || h[0].ProjectID != "p1" {
		t.Fatalf("zebra: %+v", h)
	}
	if h := search("quok"); ids(h) != "c2" || h[0].ProjectID != "p2" {
		t.Fatalf("the last word is a prefix: %+v", h)
	}
	for _, q := range []string{`zebra OR quokka`, `NEAR(zebra quokka)`, `title:zebra`} {
		if h := search(q); len(h) != 0 {
			t.Fatalf("%q is plain text, not FTS syntax: %+v", q, h)
		}
	}
	for _, q := range []string{`"zebra`, `-zebra`, `zeb*ra`, `(`, `)`, `^`, `""`, `*`} {
		search(q) // punctuation alone is dropped by the tokenizer; it must not be a syntax error
	}
	if h := search("migration plan"); ids(h) != "c1" || h[0].Snippet == "" {
		t.Fatalf("title and text: %+v", h)
	}
	if h := search(""); ids(h) != "c2,c1" {
		t.Fatalf("newest first: %+v", h)
	}
	if _, err := r.m.Search(strings.Repeat("a", searchMaxQuery+1), 0); err == nil {
		t.Fatal("a long query was taken")
	}
	var rows, msgs int
	_ = r.db.QueryRow(`SELECT COUNT(*) FROM chat_search_rows WHERE chat_id = 'c2'`).Scan(&rows)
	for _, m := range c2.messages() {
		if (m.Kind == KindUser || m.Kind == KindAgent) && m.Job == "" && m.Text != "" {
			msgs++
		}
	}
	if rows != msgs {
		t.Fatalf("index rows %d, messages %d: an updated message left a stale row", rows, msgs)
	}
	if _, err := r.run("chat.remove", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	if h := search("zebra"); len(h) != 0 {
		t.Fatalf("a removed chat was found: %+v", h)
	}
	_ = r.db.QueryRow(`SELECT COUNT(*) FROM chat_search_rows WHERE chat_id = 'c1'`).Scan(&rows)
	if rows != 0 {
		t.Fatalf("%d index rows outlived the chat", rows)
	}
}

func TestFTSQuery(t *testing.T) {
	for in, want := range map[string]string{
		"a b":       `"a" "b"*`,
		` say "hi"`: `"say" """hi"""*`,
		"":          "",
	} {
		if got := ftsQuery(in); got != want {
			t.Fatalf("%q: %q, want %q", in, got, want)
		}
	}
}
