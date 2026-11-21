package git

import (
	"os"
	"path/filepath"
	"testing"
)

type fakeTerms []string

func (f fakeTerms) FolderUse(string) []string { return f }

func TestWorktreeRemoveRefusedWithTerminal(t *testing.T) {
	r := newRig(t, opts{})
	wt, err := r.s.WorktreeAdd(bg, "p1", "")
	r.must(err)
	full := filepath.Join(r.root, filepath.FromSlash(wt.Folder))
	r.s.SetTerminals(fakeTerms{filepath.Join(full, "sub")})
	r.wantCode(r.s.WorktreeRemove(bg, "p1", wt.Folder), "BUSY")
	if _, err := os.Stat(full); err != nil {
		t.Fatal("worktree was removed")
	}
	r.s.SetTerminals(fakeTerms{r.root})
	r.must(r.s.WorktreeRemove(bg, "p1", wt.Folder))
	if _, err := os.Stat(full); err == nil {
		t.Fatal("worktree should be gone")
	}
}

func TestWorktreeNoRemoteNoWarning(t *testing.T) {
	r := newRig(t, opts{})
	r.must(func() error { _, err := r.s.WorktreeAdd(bg, "p1", ""); return err }())
	list, err := r.s.Worktrees(bg, "p1")
	r.must(err)
	if len(list) != 1 || list[0].Unpushed != 0 {
		t.Fatalf("no remote: %+v", list)
	}
}

func TestWorktreeUnpushedCount(t *testing.T) {
	r := newRig(t, opts{})
	r.git(r.root, "remote", "add", "origin", r.remote)
	_, err := r.s.Push(bg, "p1", "", true)
	r.must(err)
	wt, err := r.s.WorktreeAdd(bg, "p1", "")
	r.must(err)
	list, err := r.s.Worktrees(bg, "p1")
	r.must(err)
	if len(list) != 1 || list[0].Unpushed != 0 {
		t.Fatalf("fresh worktree: %+v", list)
	}
	full := filepath.Join(r.root, filepath.FromSlash(wt.Folder))
	for _, f := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(full, f), []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
		r.git(full, "add", "-A")
		r.git(full, "commit", "-q", "-m", f)
	}
	list, err = r.s.Worktrees(bg, "p1")
	r.must(err)
	if list[0].Unpushed != 2 {
		t.Fatalf("unpushed: %+v", list[0])
	}
	// Removal goes ahead and the branch stays.
	r.must(r.s.WorktreeRemove(bg, "p1", wt.Folder))
	if r.git(r.root, "branch", "--list", wt.Branch) == "" {
		t.Error("branch was removed")
	}
}
