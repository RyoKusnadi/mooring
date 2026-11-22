package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeLifecycle(t *testing.T) {
	r := newRig(t, opts{})
	wt, err := r.s.WorktreeAdd(bg, "p1", "")
	r.must(err)
	if !strings.HasPrefix(wt.Folder, ".mooring/worktrees/") || len(filepath.Base(wt.Folder)) != 8 ||
		wt.Branch != "mooring/"+filepath.Base(wt.Folder) || wt.Head == "" {
		t.Fatalf("worktree: %+v", wt)
	}
	if _, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(wt.Folder), "README.md")); err != nil {
		t.Fatalf("checkout missing: %v", err)
	}
	if ex := r.read(".git/info/exclude"); !strings.Contains(ex, "\n/.mooring/\n") {
		t.Errorf("info/exclude: %q", ex)
	}
	if st := r.status(""); len(st.Files) != 0 {
		t.Errorf("worktree leaks into project status: %+v", st.Files)
	}
	// A second one does not repeat the exclude line.
	wt2, err := r.s.WorktreeAdd(bg, "p1", "fix-login")
	r.must(err)
	if wt2.Branch != "fix-login" {
		t.Fatalf("branch %q", wt2.Branch)
	}
	if n := strings.Count(r.read(".git/info/exclude"), "/.mooring/"); n != 1 {
		t.Errorf("exclude line written %d times", n)
	}
	_, err = r.s.WorktreeAdd(bg, "p1", "fix-login")
	r.wantCode(err, "ALREADY_EXISTS")
	_, err = r.s.WorktreeAdd(bg, "p1", "bad name~")
	r.wantCode(err, "BAD_REQUEST")

	list, err := r.s.Worktrees(bg, "p1")
	r.must(err)
	if len(list) != 2 {
		t.Fatalf("list: %+v", list)
	}
	// The worktree is a place of its own.
	st, err := r.s.Status(bg, "p1", wt.Folder)
	r.must(err)
	if !st.Repo || st.Branch != wt.Branch || len(st.Files) != 0 {
		t.Fatalf("worktree status: %+v", st)
	}

	// Dirty, then busy, then removed.
	full := filepath.Join(r.root, filepath.FromSlash(wt.Folder))
	if err := os.WriteFile(filepath.Join(full, "scratch.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.wantCode(r.s.WorktreeRemove(bg, "p1", wt.Folder), "DIRTY")
	if err := os.Remove(filepath.Join(full, "scratch.txt")); err != nil {
		t.Fatal(err)
	}
	r.chats[full] = ChatUse{ID: "chat1", Live: true}
	r.wantCode(r.s.WorktreeRemove(bg, "p1", wt.Folder), "BUSY")
	list, _ = r.s.Worktrees(bg, "p1")
	var used string
	for _, w := range list {
		if w.Folder == wt.Folder {
			used = w.ChatID
		}
	}
	if used != "chat1" {
		t.Errorf("chatId = %q", used)
	}
	r.chats[full] = ChatUse{ID: "chat1", Live: false}
	r.must(r.s.WorktreeRemove(bg, "p1", wt.Folder))
	if _, err := os.Stat(full); !os.IsNotExist(err) {
		t.Errorf("folder kept: %v", err)
	}
	if r.git(r.root, "branch", "--list", wt.Branch) == "" {
		t.Error("branch was deleted")
	}
	// A folder that vanished is pruned.
	full2 := filepath.Join(r.root, filepath.FromSlash(wt2.Folder))
	r.must(os.RemoveAll(full2))
	r.must(r.s.WorktreeRemove(bg, "p1", wt2.Folder))
	if list, _ := r.s.Worktrees(bg, "p1"); len(list) != 0 {
		t.Errorf("list after remove: %+v", list)
	}
	r.wantCode(r.s.WorktreeRemove(bg, "p1", "README.md"), "BAD_REQUEST")
	r.wantCode(r.s.WorktreeRemove(bg, "p1", "../x"), "OUTSIDE_PROJECT")
}

func TestWorktreeBeforeFirstCommit(t *testing.T) {
	r := newRig(t, opts{noCommit: true})
	_, err := r.s.WorktreeAdd(bg, "p1", "")
	r.wantCode(err, "NOT_FOUND")
}

func TestWorktreeInSubfolderProject(t *testing.T) {
	r := newRig(t, opts{subfolder: "app"})
	wt, err := r.s.WorktreeAdd(bg, "p1", "")
	r.must(err)
	if !strings.HasPrefix(wt.Folder, ".mooring/worktrees/") || !strings.HasSuffix(wt.Folder, "/app") {
		t.Fatalf("folder %q", wt.Folder)
	}
	if ex := r.read(".git/info/exclude"); !strings.Contains(ex, "/app/.mooring/") {
		t.Errorf("exclude: %q", ex)
	}
	if st := r.status(""); len(st.Files) != 0 {
		t.Errorf("leaks: %+v", st.Files)
	}
	st, err := r.s.Status(bg, "p1", wt.Folder)
	r.must(err)
	if !st.Repo || st.Branch != wt.Branch {
		t.Fatalf("status: %+v", st)
	}
	list, err := r.s.Worktrees(bg, "p1")
	r.must(err)
	if len(list) != 1 || list[0].Folder != wt.Folder {
		t.Fatalf("list: %+v", list)
	}
	r.must(r.s.WorktreeRemove(bg, "p1", wt.Folder))
}
