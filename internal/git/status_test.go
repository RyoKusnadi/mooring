package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStatusKinds(t *testing.T) {
	r := newRig(t, opts{})
	r.write("keep.txt", "k\n")
	r.write("gone.txt", "g\n")
	r.write("old name.txt", "line one\nline two\nline three\nline four\n")
	r.write("both.txt", "1\n")
	r.commitAll("base")
	if st := r.status(""); len(st.Files) != 0 || st.Branch != "main" || st.Head == "" || st.Upstream != "" || st.Detached {
		t.Fatalf("clean status: %+v", st)
	}

	r.write("new.txt", "n\n")
	r.git(r.root, "add", "new.txt")
	r.git(r.root, "rm", "-q", "gone.txt")
	r.git(r.root, "mv", "old name.txt", "ünï ✓.txt")
	r.write("both.txt", "2\n")
	r.git(r.root, "add", "both.txt")
	r.write("both.txt", "3\n")
	r.write("wild\nname.txt", "w\n")
	r.write("dir/with space/f.txt", "f\n")
	r.write("untracked.txt", "u\n")

	st := r.status("")
	want := map[string]Change{
		"new.txt":              {Path: "new.txt", Staged: "added"},
		"gone.txt":             {Path: "gone.txt", Staged: "deleted"},
		"ünï ✓.txt":            {Path: "ünï ✓.txt", OrigPath: "old name.txt", Staged: "renamed"},
		"both.txt":             {Path: "both.txt", Staged: "modified", Unstaged: "modified"},
		"wild\nname.txt":       {Path: "wild\nname.txt", Kind: "untracked"},
		"dir/with space/f.txt": {Path: "dir/with space/f.txt", Kind: "untracked"},
		"untracked.txt":        {Path: "untracked.txt", Kind: "untracked"},
	}
	if len(st.Files) != len(want) {
		t.Fatalf("got %d changes: %+v", len(st.Files), st.Files)
	}
	for _, c := range st.Files {
		if w, ok := want[c.Path]; !ok || w != c {
			t.Errorf("change %+v, want %+v", c, w)
		}
	}
	for i := 1; i < len(st.Files); i++ {
		if st.Files[i-1].Path > st.Files[i].Path {
			t.Errorf("not sorted at %d", i)
		}
	}
}

func TestStatusUnstagedRenameIsDeleteAndUntracked(t *testing.T) {
	r := newRig(t, opts{})
	r.write("a.txt", "content content content\nmore\nmore\n")
	r.commitAll("a")
	if err := os.Rename(filepath.Join(r.root, "a.txt"), filepath.Join(r.root, "b.txt")); err != nil {
		t.Fatal(err)
	}
	st := r.status("")
	if a := find(st, "a.txt"); a == nil || a.Unstaged != "deleted" {
		t.Errorf("a.txt: %+v", a)
	}
	if b := find(st, "b.txt"); b == nil || b.Kind != "untracked" {
		t.Errorf("b.txt: %+v", b)
	}
}

func TestStatusConflicted(t *testing.T) {
	r := newRig(t, opts{})
	r.write("c.txt", "base\n")
	r.commitAll("base")
	r.git(r.root, "checkout", "-q", "-b", "other")
	r.write("c.txt", "other\n")
	r.commitAll("other")
	r.git(r.root, "checkout", "-q", "main")
	r.write("c.txt", "main\n")
	r.commitAll("main")
	// A merge that conflicts exits non-zero, so do not use r.git.
	merge := exec.Command("git", "merge", "other")
	merge.Dir, merge.Env = r.root, append(os.Environ(), r.env...)
	_ = merge.Run()
	st := r.status("")
	c := find(st, "c.txt")
	if c == nil || c.Kind != "conflicted" || !st.Conflicted {
		t.Fatalf("status: %+v", st)
	}
	_, err := r.s.Commit(context.Background(), "p1", "", "try")
	r.wantCode(err, "CONFLICTED")
}

func TestStatusBranchInfoAndNoRepo(t *testing.T) {
	r := newRig(t, opts{noCommit: true})
	st := r.status("")
	if !st.Repo || st.Branch != "main" || st.Head != "" {
		t.Fatalf("before first commit: %+v", st)
	}
	r2 := newRig(t, opts{noRepo: true})
	if st := r2.status(""); st.Repo || len(st.Files) != 0 {
		t.Fatalf("no repo: %+v", st)
	}
	r2.wantCode(r2.s.Stage(context.Background(), "p1", "", []string{""}), "NOT_A_REPO")
}
