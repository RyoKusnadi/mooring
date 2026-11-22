package projects

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/store"
)

type published struct {
	Kind string
	Data json.RawMessage
}

type recorder struct {
	mu     sync.Mutex
	events []published
}

func (r *recorder) Publish(kind string, data any) {
	raw, _ := json.Marshal(data)
	r.mu.Lock()
	r.events = append(r.events, published{kind, raw})
	r.mu.Unlock()
}

func (r *recorder) kinds() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.events {
		out = append(out, e.Kind)
	}
	return out
}

type env struct {
	svc  *Service
	rec  *recorder
	home string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	svc, err := New(st.DB, rec, home, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &env{svc: svc, rec: rec, home: home}
}

func (e *env) run(name string, args any) (json.RawMessage, error) {
	raw, _ := json.Marshal(args)
	v, err := e.svc.Run(context.Background(), events.Command{ID: "t1", Name: name, Args: raw})
	if err != nil {
		return nil, err
	}
	out, _ := json.Marshal(v)
	return out, nil
}

func code(err error) string {
	var le *ledger.Error
	if errors.As(err, &le) {
		return le.Code
	}
	return "other:" + err.Error()
}

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func (e *env) add(t *testing.T, dir string) Project {
	t.Helper()
	raw, err := e.run("project.add", map[string]string{"path": dir})
	if err != nil {
		t.Fatal(err)
	}
	var p Project
	_ = json.Unmarshal(raw, &p)
	return p
}

func TestFsListFoldersOnlyHidesDotAndMarksGit(t *testing.T) {
	e := newEnv(t)
	mkdir(t, e.home, "Beta", ".git")
	mkdir(t, e.home, "alpha")
	mkdir(t, e.home, ".config", "inner")
	if err := os.WriteFile(filepath.Join(e.home, "file.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := e.run("fs.list", map[string]string{"path": e.home})
	if err != nil {
		t.Fatal(err)
	}
	var l listing
	_ = json.Unmarshal(raw, &l)
	if l.Path != e.home || l.Parent != "" || len(l.Entries) != 2 ||
		l.Entries[0] != (entry{"alpha", true, false}) || l.Entries[1] != (entry{"Beta", true, true}) {
		t.Fatalf("%+v", l)
	}
	raw, err = e.run("fs.list", map[string]string{"path": filepath.Join(e.home, ".config")})
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(raw, &l)
	if l.Parent != e.home || len(l.Entries) != 1 || l.Entries[0].Name != "inner" {
		t.Fatalf("%+v", l)
	}
}

func TestFsListErrors(t *testing.T) {
	e := newEnv(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(e.home, "escape")); err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{
		outside: "OUTSIDE_ROOT", "..": "OUTSIDE_ROOT", "escape": "OUTSIDE_ROOT",
		"missing": "NOT_FOUND",
	} {
		if _, err := e.run("fs.list", map[string]string{"path": in}); code(err) != want {
			t.Errorf("%q: %v, want %s", in, err, want)
		}
	}
	if _, err := e.run("nope", nil); code(err) != "BAD_REQUEST" {
		t.Error("unknown command")
	}
}

func TestProjectLifecycle(t *testing.T) {
	e := newEnv(t)
	a := e.add(t, mkdir(t, e.home, "a"))
	b := e.add(t, mkdir(t, e.home, "b"))
	if a.Name != "a" || a.Position != 0 || b.Position != 1 {
		t.Fatalf("%+v %+v", a, b)
	}
	if _, err := e.run("project.add", map[string]string{"path": "a"}); code(err) != "ALREADY_ADDED" {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := e.run("project.add", map[string]string{"path": "/etc"}); code(err) != "OUTSIDE_ROOT" {
		t.Fatalf("outside: %v", err)
	}
	if _, err := e.run("project.rename", map[string]string{"id": a.ID, "name": "Alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.run("project.rename", map[string]string{"id": "zz", "name": "x"}); code(err) != "NOT_FOUND" {
		t.Fatalf("rename missing: %v", err)
	}
	if _, err := e.run("project.reorder", map[string]any{"ids": []string{b.ID}}); code(err) != "BAD_REQUEST" {
		t.Fatalf("partial reorder: %v", err)
	}
	if _, err := e.run("project.reorder", map[string]any{"ids": []string{b.ID, a.ID}}); err != nil {
		t.Fatal(err)
	}
	layout := map[string]any{"projectId": a.ID, "layout": map[string]any{"root": map[string]any{}, "focus": "t1"}}
	if _, err := e.run("layout.save", layout); err != nil {
		t.Fatal(err)
	}
	if _, err := e.run("layout.save", map[string]any{"projectId": "zz", "layout": map[string]any{}}); code(err) != "NOT_FOUND" {
		t.Fatal("layout for missing project")
	}
	snap, err := e.svc.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	m := snap.(map[string]any)
	ps := m["projects"].([]Project)
	if m["home"] != e.home || len(ps) != 2 || ps[0].ID != b.ID || ps[1].Name != "Alpha" || len(m["layouts"].(map[string]json.RawMessage)) != 1 {
		t.Fatalf("%+v", m)
	}
	if _, err := e.run("project.remove", map[string]string{"id": a.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.home, "a")); err != nil {
		t.Fatal("files must stay on disk")
	}
	snap, _ = e.svc.Snapshot()
	if len(snap.(map[string]any)["layouts"].(map[string]json.RawMessage)) != 0 {
		t.Fatal("layout should go with the project")
	}
	want := []string{"project.added", "project.added", "project.changed", "project.reordered", "layout.changed", "project.removed"}
	got := e.rec.kinds()
	if len(got) != len(want) {
		t.Fatalf("events %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events %v", got)
		}
	}
}

func TestProjectCreate(t *testing.T) {
	e := newEnv(t)
	raw, err := e.run("project.create", map[string]string{"parent": e.home, "name": "fresh"})
	if err != nil {
		t.Fatal(err)
	}
	var p Project
	_ = json.Unmarshal(raw, &p)
	if info, err := os.Stat(filepath.Join(e.home, "fresh")); err != nil || !info.IsDir() || p.Path != filepath.Join(e.home, "fresh") {
		t.Fatalf("%v %+v", err, p)
	}
	if _, err := e.run("project.create", map[string]string{"parent": e.home, "name": "fresh"}); code(err) != "ALREADY_EXISTS" {
		t.Fatalf("%v", err)
	}
	for _, n := range []string{"", "..", "a/b", "."} {
		if _, err := e.run("project.create", map[string]string{"parent": e.home, "name": n}); code(err) != "BAD_REQUEST" {
			t.Fatalf("name %q: %v", n, err)
		}
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
