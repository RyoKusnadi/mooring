package projects

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCloneFromLocalBareRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	e := newEnv(t)
	work := t.TempDir()
	bare := filepath.Join(work, "origin.git")
	gitIn(t, work, "init", "--bare", "-b", "main", bare)
	seed := filepath.Join(work, "seed")
	gitIn(t, work, "clone", bare, seed)
	if err := os.WriteFile(filepath.Join(seed, "README"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, seed, "add", ".")
	gitIn(t, seed, "commit", "-m", "first")
	gitIn(t, seed, "push", "origin", "HEAD:main")

	raw, err := e.run("project.clone", map[string]string{"url": bare, "parent": e.home})
	if err != nil {
		t.Fatal(err)
	}
	var p Project
	_ = json.Unmarshal(raw, &p)
	if p.Name != "origin" || p.Path != filepath.Join(e.home, "origin") {
		t.Fatalf("%+v", p)
	}
	if _, err := os.Stat(filepath.Join(p.Path, "README")); err != nil {
		t.Fatal("clone has no files:", err)
	}
	var progress int
	for _, ev := range e.rec.events {
		if ev.Kind == "clone.progress" {
			progress++
			var d map[string]string
			_ = json.Unmarshal(ev.Data, &d)
			if d["cmd"] != "t1" || d["line"] == "" {
				t.Fatalf("progress data %s", ev.Data)
			}
		}
	}
	if progress == 0 || e.rec.kinds()[len(e.rec.kinds())-1] != "project.added" {
		t.Fatalf("events %v", e.rec.kinds())
	}
	if _, err := e.run("project.clone", map[string]string{"url": bare, "parent": e.home}); code(err) != "ALREADY_EXISTS" {
		t.Fatalf("second clone: %v", err)
	}
}

func TestFailedCloneLeavesNoFolder(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	e := newEnv(t)
	_, err := e.run("project.clone", map[string]string{"url": filepath.Join(e.home, "no-such-repo"), "parent": e.home, "name": "dest"})
	if code(err) != "CLONE_FAILED" {
		t.Fatalf("%v", err)
	}
	if _, statErr := os.Stat(filepath.Join(e.home, "dest")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("folder left behind: %v", statErr)
	}
	snap, _ := e.svc.Snapshot()
	if len(snap.(map[string]any)["projects"].([]Project)) != 0 {
		t.Fatal("failed clone added a project")
	}
}

func TestCloneRejectsBadInput(t *testing.T) {
	e := newEnv(t)
	for _, u := range []string{"", "--upload-pack=touch x", "-x"} {
		if _, err := e.run("project.clone", map[string]string{"url": u, "parent": e.home}); code(err) != "BAD_REQUEST" {
			t.Errorf("%q: %v", u, err)
		}
	}
	if _, err := e.run("project.clone", map[string]string{"url": "x", "parent": "/etc"}); code(err) != "OUTSIDE_ROOT" {
		t.Error("parent outside root")
	}
	if got := repoName("https://host/u/repo.git"); got != "repo" {
		t.Error(got)
	}
	if got := repoName("git@host:u/repo/"); got != "repo" {
		t.Error(got)
	}
	if got := redact("fatal: https://user:pw@host/x"); got != "fatal: https://host/x" {
		t.Error(got)
	}
}
