package chat

import (
	"testing"
)

func (r *rig) addProject(id string) {
	r.t.Helper()
	if _, err := r.db.Exec(`INSERT INTO projects (id, name, path, position, created_at) VALUES (?, ?, ?, 1, 0)`, id, id, r.t.TempDir()); err != nil {
		r.t.Fatal(err)
	}
}

// stopped adds a chat that is never started, which is all a group needs.
func (r *rig) stopped(id, project string) {
	r.t.Helper()
	c, err := newChat(r.m, id, project, r.dir, r.dir, Spec{Agent: AgentSlot{ID: "fake"}})
	if err != nil {
		r.t.Fatal(err)
	}
	if err := r.m.insertRow(c, c.Info()); err != nil {
		r.t.Fatal(err)
	}
	r.m.chats[id] = c
}

func (r *rig) group(ids ...string) (string, error) {
	r.t.Helper()
	out, err := r.run("chat.group", map[string]any{"ids": ids, "title": "task"})
	if err != nil {
		return "", err
	}
	return out.(map[string]any)["groupId"].(string), nil
}

func (r *rig) groupOf(id string) string { return r.m.get(id).Info().GroupID }

func TestGroupMakeChangeUngroupDissolve(t *testing.T) {
	r := newRig(t)
	for _, id := range []string{"a", "b", "c", "d"} {
		r.stopped(id, "p1")
	}
	g1, err := r.group("a", "b")
	if err != nil || !idPattern.MatchString(g1) {
		t.Fatalf("group: %q %v", g1, err)
	}
	if r.groupOf("a") != g1 || r.groupOf("b") != g1 || r.groupOf("c") != "" || r.m.get("a").Info().GroupTitle != "task" {
		t.Fatal("members wrong after make")
	}
	if r.rec.count("chat.changed") != 2 {
		t.Fatalf("chat.changed %d, want 2", r.rec.count("chat.changed"))
	}
	// b moves to a new group with c and d: a is left alone, so g1 dissolves.
	g2, err := r.group("b", "c", "d")
	if err != nil || g2 == g1 {
		t.Fatalf("regroup: %q %v", g2, err)
	}
	if r.groupOf("a") != "" || r.groupOf("b") != g2 || r.groupOf("d") != g2 {
		t.Fatalf("after regroup: a=%q b=%q d=%q", r.groupOf("a"), r.groupOf("b"), r.groupOf("d"))
	}
	if r.m.groupTitle(g1) != "" {
		t.Fatal("a dissolved group keeps its title")
	}
	// Ungroup one of three: the other two stay together.
	if _, err := r.run("chat.ungroup", map[string]any{"id": "d"}); err != nil {
		t.Fatal(err)
	}
	if r.groupOf("d") != "" || r.groupOf("b") != g2 || r.groupOf("c") != g2 {
		t.Fatal("ungroup of one of three")
	}
	// Ungroup one of two: the group dissolves.
	if _, err := r.run("chat.ungroup", map[string]any{"id": "c"}); err != nil {
		t.Fatal(err)
	}
	if r.groupOf("b") != "" || r.groupOf("c") != "" {
		t.Fatal("a group with one chat was kept")
	}
	var n int
	_ = r.db.QueryRow(`SELECT COUNT(*) FROM chat_groups`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d group rows left", n)
	}
	if _, err := r.run("chat.ungroup", map[string]any{"id": "c"}); err != nil {
		t.Fatalf("ungroup of an ungrouped chat: %v", err)
	}
}

func TestGroupRefusals(t *testing.T) {
	r := newRig(t)
	r.addProject("p2")
	for _, id := range []string{"a", "b", "c", "d"} {
		r.stopped(id, "p1")
	}
	r.stopped("x", "p2")
	for name, ids := range map[string][]string{
		"one": {"a"}, "none": {}, "four": {"a", "b", "c", "d"}, "projects": {"a", "x"},
		"twice": {"a", "a"}, "missing": {"a", "nope"},
	} {
		if _, err := r.group(ids...); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	for _, id := range []string{"a", "b", "x"} {
		if r.groupOf(id) != "" {
			t.Errorf("%s was grouped by a refused command", id)
		}
	}
}

func TestGroupSurvivesARestartAndRemoval(t *testing.T) {
	r := newRig(t)
	for _, id := range []string{"a", "b", "c"} {
		r.stopped(id, "p1")
	}
	g, err := r.group("a", "b", "c")
	if err != nil {
		t.Fatal(err)
	}
	r.boot()
	for _, id := range []string{"a", "b", "c"} {
		if in := r.m.get(id).Info(); in.GroupID != g || in.GroupTitle != "task" {
			t.Fatalf("%s after restart: %q %q", id, in.GroupID, in.GroupTitle)
		}
	}
	if _, err := r.run("chat.remove", map[string]any{"id": "c"}); err != nil {
		t.Fatal(err)
	}
	if r.groupOf("a") != g || r.groupOf("b") != g {
		t.Fatal("two chats left should stay grouped")
	}
	if _, err := r.run("chat.remove", map[string]any{"id": "b"}); err != nil {
		t.Fatal(err)
	}
	if r.groupOf("a") != "" {
		t.Fatal("the last chat stays in a dissolved group")
	}
	r.boot()
	if r.groupOf("a") != "" || r.m.groupTitle(g) != "" {
		t.Fatal("the dissolved group came back after a restart")
	}
}
