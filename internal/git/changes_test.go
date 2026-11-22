package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

var bg = context.Background()

func TestStageUnstageAll(t *testing.T) {
	r := newRig(t, opts{})
	r.write("a.txt", "a\n")
	r.write("sub/b.txt", "b\n")
	r.must(r.s.Stage(bg, "p1", "", []string{""}))
	st := r.status("")
	if len(st.Files) != 2 || st.Files[0].Staged != "added" || st.Files[1].Staged != "added" {
		t.Fatalf("after stage all: %+v", st.Files)
	}
	if r.rec.count("git.changed") != 1 {
		t.Errorf("git.changed events: %d", r.rec.count("git.changed"))
	}
	r.must(r.s.Unstage(bg, "p1", "", []string{""}))
	st = r.status("")
	if c := find(st, "a.txt"); c == nil || c.Kind != "untracked" || c.Staged != "" {
		t.Fatalf("after unstage all: %+v", st.Files)
	}
	r.must(r.s.Stage(bg, "p1", "", []string{"a.txt"}))
	if c := find(r.status(""), "a.txt"); c.Staged != "added" {
		t.Fatalf("one file: %+v", c)
	}
}

func TestStageDeleteAndRenamePairs(t *testing.T) {
	r := newRig(t, opts{})
	r.write("old.txt", "one\ntwo\nthree\nfour\nfive\n")
	r.write("gone.txt", "g\n")
	r.commitAll("base")
	if err := os.Remove(filepath.Join(r.root, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(r.root, "old.txt"), filepath.Join(r.root, "new.txt")); err != nil {
		t.Fatal(err)
	}
	r.must(r.s.Stage(bg, "p1", "", []string{"gone.txt"}))
	if c := find(r.status(""), "gone.txt"); c.Staged != "deleted" || c.Unstaged != "" {
		t.Fatalf("staged delete: %+v", c)
	}
	r.must(r.s.Stage(bg, "p1", "", []string{"old.txt", "new.txt"}))
	st := r.status("")
	if c := find(st, "new.txt"); c == nil || c.Staged != "renamed" || c.OrigPath != "old.txt" {
		t.Fatalf("staged rename: %+v", st.Files)
	}
	r.must(r.s.Unstage(bg, "p1", "", []string{"old.txt", "new.txt"}))
	st = r.status("")
	if c := find(st, "old.txt"); c == nil || c.Unstaged != "deleted" || c.Staged != "" {
		t.Fatalf("unstaged rename: %+v", st.Files)
	}
	if c := find(st, "new.txt"); c == nil || c.Kind != "untracked" {
		t.Fatalf("unstaged rename target: %+v", st.Files)
	}
}

func TestUnstageBeforeFirstCommit(t *testing.T) {
	r := newRig(t, opts{noCommit: true})
	r.write("a.txt", "a\n")
	r.must(r.s.Stage(bg, "p1", "", []string{"a.txt"}))
	r.must(r.s.Unstage(bg, "p1", "", []string{"a.txt"}))
	if c := find(r.status(""), "a.txt"); c == nil || c.Kind != "untracked" {
		t.Fatalf("a.txt: %+v", c)
	}
}

func TestPathChecks(t *testing.T) {
	r := newRig(t, opts{})
	outside := filepath.Join(r.base, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(r.root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"../x", "a/../../x", "link/x.txt"} {
		r.wantCode(r.s.Stage(bg, "p1", "", []string{p}), "OUTSIDE_PROJECT")
		r.wantCode(r.s.Discard(bg, "p1", "", []string{p}, false), "OUTSIDE_PROJECT")
	}
	r.wantCode(r.s.Stage(bg, "p1", "", []string{"/abs"}), "BAD_REQUEST")
	r.wantCode(r.s.Stage(bg, "p1", "", nil), "BAD_REQUEST")
	r.wantCode(r.s.Stage(bg, "p1", "../x", []string{""}), "OUTSIDE_PROJECT")
	// A link itself is a fine path: git stores the link, not its target.
	r.must(r.s.Stage(bg, "p1", "", []string{"link"}))
	if c := find(r.status(""), "link"); c == nil || c.Staged != "added" {
		t.Fatalf("link: %+v", c)
	}
}

func TestDiscardToTrashAndPermanent(t *testing.T) {
	r := newRig(t, opts{})
	r.write("t.txt", "orig\n")
	r.write("staged.txt", "s1\n")
	r.write("deleted.txt", "d\n")
	r.commitAll("base")
	r.write("t.txt", "changed\n")
	r.write("staged.txt", "s2\n")
	r.git(r.root, "add", "staged.txt")
	r.write("staged.txt", "s3\n")
	r.write("fresh.txt", "fresh\n")
	if err := os.Remove(filepath.Join(r.root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}

	r.must(r.s.Discard(bg, "p1", "", []string{""}, false))
	if got := r.read("t.txt"); got != "orig\n" {
		t.Errorf("t.txt = %q", got)
	}
	if got := r.read("staged.txt"); got != "s2\n" {
		t.Errorf("staged change must stay: %q", got)
	}
	if got := r.read("deleted.txt"); got != "d\n" {
		t.Errorf("deleted.txt = %q", got)
	}
	if _, err := os.Stat(filepath.Join(r.root, "fresh.txt")); !os.IsNotExist(err) {
		t.Errorf("fresh.txt should be gone: %v", err)
	}
	trashed, _ := os.ReadDir(filepath.Join(r.base, "trash"))
	if len(trashed) != 3 {
		t.Errorf("trash holds %d items, want 3 (t.txt, staged.txt, fresh.txt)", len(trashed))
	}
}

func TestDiscardTrashUnavailable(t *testing.T) {
	r := newRig(t, opts{noTrash: true})
	r.write("README.md", "changed\n")
	r.write("u.txt", "u\n")
	r.wantCode(r.s.Discard(bg, "p1", "", []string{"README.md"}, false), "TRASH_UNAVAILABLE")
	if r.read("README.md") != "changed\n" {
		t.Fatal("file touched without a trash")
	}
	r.must(r.s.Discard(bg, "p1", "", []string{""}, true))
	if r.read("README.md") != "hello\n" {
		t.Fatal("not restored")
	}
	if _, err := os.Stat(filepath.Join(r.root, "u.txt")); !os.IsNotExist(err) {
		t.Fatal("u.txt kept")
	}
}

func TestSubfolderProject(t *testing.T) {
	r := newRig(t, opts{subfolder: "app"})
	r.write("app/in.txt", "in\n")
	r.write("outer.txt", "out\n")
	r.git(r.root, "add", "outer.txt")
	st := r.status("")
	if len(st.Files) != 1 || st.Files[0].Path != "in.txt" || st.Files[0].Kind != "untracked" {
		t.Fatalf("scoped status: %+v", st.Files)
	}
	if st.StagedOut != 1 {
		t.Errorf("stagedOutside = %d, want 1", st.StagedOut)
	}
	r.must(r.s.Stage(bg, "p1", "", []string{""}))
	st = r.status("")
	if c := find(st, "in.txt"); c == nil || c.Staged != "added" {
		t.Fatalf("after stage: %+v", st.Files)
	}
	if find(r.status(""), "outer.txt") != nil {
		t.Error("outer.txt listed")
	}
	r.must(r.s.Unstage(bg, "p1", "", []string{"in.txt"}))
	if r.git(r.root, "diff", "--cached", "--name-only") != "outer.txt" {
		t.Errorf("staged: %q", r.git(r.root, "diff", "--cached", "--name-only"))
	}
	r.wantCode(r.s.Stage(bg, "p1", "", []string{"../outer.txt"}), "OUTSIDE_PROJECT")
	d, err := r.s.Diff(bg, "p1", "", "in.txt", false)
	r.must(err)
	if len(d.Hunks) != 1 || d.Path != "in.txt" {
		t.Fatalf("diff: %+v", d)
	}
	// A commit takes everything staged, outside the place too.
	res, err := r.s.Commit(bg, "p1", "", "scoped")
	r.must(err)
	if res.Branch != "main" || res.Commit == "" {
		t.Fatalf("commit: %+v", res)
	}
	if r.git(r.root, "show", "--name-only", "--format=", "HEAD") != "outer.txt" {
		t.Errorf("commit held %q", r.git(r.root, "show", "--name-only", "--format=", "HEAD"))
	}
}

// A discard acts on the paths the user was asked about, not on whatever the status shows later.
func TestDiscardOnlyTheListedPaths(t *testing.T) {
	r := newRig(t, opts{})
	r.write("a.txt", "orig\n")
	r.commitAll("base")
	r.write("a.txt", "changed\n")
	r.write("asked.txt", "new\n")
	listed := []string{"a.txt", "asked.txt"}
	r.write("later.txt", "made after the question\n")
	r.write("a2.txt", "also later\n")

	r.must(r.s.Discard(bg, "p1", "", listed, false))
	if got := r.read("a.txt"); got != "orig\n" {
		t.Errorf("a.txt = %q", got)
	}
	if _, err := os.Stat(filepath.Join(r.root, "asked.txt")); !os.IsNotExist(err) {
		t.Errorf("asked.txt should be gone: %v", err)
	}
	for _, keep := range []string{"later.txt", "a2.txt"} {
		if _, err := os.Stat(filepath.Join(r.root, keep)); err != nil {
			t.Errorf("%s was not on the list and must survive: %v", keep, err)
		}
	}
}
