package git

import (
	"os"
	"path/filepath"
	"testing"
)

// Fixes from the phase 6 code review.

func TestDiscardKeepsNestedRepository(t *testing.T) {
	r := newRig(t, opts{})
	nested := filepath.Join(r.root, "nested")
	r.write("nested/own.txt", "mine\n")
	r.git(nested, "init", "-q")
	r.write("loose.txt", "x\n")

	r.must(r.s.Discard(bg, "p1", "", []string{""}, true))
	if _, err := os.Stat(filepath.Join(nested, ".git")); err != nil {
		t.Fatalf("nested repository was thrown away: %v", err)
	}
	if got := r.read("nested/own.txt"); got != "mine\n" {
		t.Errorf("nested file = %q", got)
	}
	if _, err := os.Stat(filepath.Join(r.root, "loose.txt")); !os.IsNotExist(err) {
		t.Errorf("loose.txt should be gone: %v", err)
	}
}

func TestDiscardFolderPath(t *testing.T) {
	r := newRig(t, opts{})
	r.write("src/a.txt", "a\n")
	r.write("src/b.txt", "b\n")
	r.write("srcx.txt", "x\n")
	r.commitAll("base")
	r.write("src/a.txt", "a2\n")
	r.write("src/b.txt", "b2\n")
	r.write("srcx.txt", "x2\n")
	r.must(r.s.Discard(bg, "p1", "", []string{"src"}, false))
	if r.read("src/a.txt") != "a\n" || r.read("src/b.txt") != "b\n" {
		t.Errorf("folder not discarded: %q %q", r.read("src/a.txt"), r.read("src/b.txt"))
	}
	if r.read("srcx.txt") != "x2\n" {
		t.Error("a sibling with the same prefix was discarded")
	}
}

func TestPushNeverForcesThroughConfig(t *testing.T) {
	r := newRig(t, opts{})
	r.git(r.root, "remote", "add", "origin", r.remote)
	_, err := r.s.Push(bg, "p1", "", true)
	r.must(err)
	pushed := r.git(r.remote, "rev-parse", "main")
	r.git(r.root, "commit", "-q", "--amend", "-m", "rewritten")
	r.git(r.root, "config", "remote.origin.push", "+refs/heads/main:refs/heads/main")
	_, err = r.s.Push(bg, "p1", "", false)
	r.wantCode(err, "PUSH_REJECTED")
	if r.git(r.remote, "rev-parse", "main") != pushed {
		t.Fatal("the remote branch was overwritten")
	}
}

func TestWorktreeRemoveKeepsIgnoredFilesInTrash(t *testing.T) {
	r := newRig(t, opts{})
	r.write(".gitignore", ".env\n")
	r.commitAll("ignore")
	wt, err := r.s.WorktreeAdd(bg, "p1", "")
	r.must(err)
	r.write(wt.Folder+"/.env", "SECRET=1\n")
	r.must(r.s.WorktreeRemove(bg, "p1", wt.Folder))
	if _, err := os.Stat(filepath.Join(r.base, "trash", filepath.Base(wt.Folder), ".env")); err != nil {
		t.Fatalf(".env is not in the trash: %v", err)
	}
	if wts, err := r.s.Worktrees(bg, "p1"); err != nil || len(wts) != 0 {
		t.Fatalf("worktree still listed: %+v %v", wts, err)
	}
}

func TestWorktreeRemoveWithoutTrashRefusesIgnoredFiles(t *testing.T) {
	r := newRig(t, opts{noTrash: true})
	r.write(".gitignore", ".env\n")
	r.commitAll("ignore")
	wt, err := r.s.WorktreeAdd(bg, "p1", "")
	r.must(err)
	r.write(wt.Folder+"/.env", "SECRET=1\n")
	r.wantCode(r.s.WorktreeRemove(bg, "p1", wt.Folder), "DIRTY")
	if r.read(wt.Folder+"/.env") != "SECRET=1\n" {
		t.Fatal(".env was deleted")
	}
}

func TestStatusOutputIsCapped(t *testing.T) {
	r := newRig(t, opts{})
	for i := 0; i < 40; i++ {
		r.write(filepath.Join("many", string(rune('a'+i%26))+string(rune('a'+i/26))+".txt"), "x\n")
	}
	old := maxListBytes
	maxListBytes = 300
	defer func() { maxListBytes = old }()
	st := r.status("")
	if !st.Truncated || len(st.Files) == 0 || len(st.Files) >= 40 {
		t.Fatalf("truncated=%v files=%d", st.Truncated, len(st.Files))
	}
	for _, c := range st.Files {
		if c.Kind != "untracked" || filepath.Dir(c.Path) != "many" {
			t.Fatalf("a cut record was parsed: %+v", c)
		}
	}
}
