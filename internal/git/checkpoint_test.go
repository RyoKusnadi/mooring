package git

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// repoState is everything a checkpoint must leave alone.
func (r *rig) repoState() string {
	r.t.Helper()
	idx, err := os.ReadFile(filepath.Join(r.root, ".git", "index"))
	if err != nil {
		r.t.Fatal(err)
	}
	parts := []string{
		r.git(r.root, "ls-files", "-s"), r.git(r.root, "rev-parse", "HEAD"), r.git(r.root, "stash", "list"),
		r.git(r.root, "branch", "-a", "-v"), r.git(r.root, "status", "--porcelain=v2", "--branch", "--untracked-files=all"),
		string(idx),
	}
	return strings.Join(parts, "\n--\n")
}

func (r *rig) checkpoint(chat string, end bool) {
	r.t.Helper()
	r.must(r.s.Checkpoint(context.Background(), "p1", r.root, chat, end))
}

func (r *rig) cpRefNames(chat string) []string {
	r.t.Helper()
	out := r.git(r.root, "for-each-ref", "--format=%(refname)", cpNS+chat+"/")
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func TestCheckpointLeavesTheRepositoryAlone(t *testing.T) {
	r := newRig(t, opts{})
	r.write(".gitignore", "*.log\n")
	r.commitAll("ignore logs")
	r.git(r.root, "branch", "other")
	r.write("stashed.txt", "x\n")
	r.git(r.root, "add", "stashed.txt")
	r.git(r.root, "stash", "push", "-q", "-m", "mine")
	r.write("README.md", "hello\nchanged\n")
	r.write("staged.txt", "staged\n")
	r.git(r.root, "add", "staged.txt")
	r.write("new.txt", "untracked\n")
	r.write("sub/deep.txt", "deep\n")
	r.write("debug.log", "ignored\n")
	before := r.repoState()

	r.checkpoint("c1", false)

	if after := r.repoState(); after != before {
		t.Fatalf("the repository changed:\n%s\n--- after ---\n%s", before, after)
	}
	files := r.git(r.root, "ls-tree", "-r", "--name-only", cpNS+"c1/1")
	for _, want := range []string{"README.md", "staged.txt", "new.txt", "sub/deep.txt", ".gitignore"} {
		if !strings.Contains("\n"+files+"\n", "\n"+want+"\n") {
			t.Errorf("%s is not in the checkpoint:\n%s", want, files)
		}
	}
	if strings.Contains(files, "debug.log") {
		t.Errorf("an ignored file is in the checkpoint:\n%s", files)
	}
	if got := r.git(r.root, "show", cpNS+"c1/1:README.md"); got != "hello\nchanged" {
		t.Errorf("README in the checkpoint = %q", got)
	}
	if left, _ := filepath.Glob(filepath.Join(r.root, ".git", "mooring-cp-*")); len(left) > 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

func TestCheckpointInARepositoryWithNoCommit(t *testing.T) {
	r := newRig(t, opts{noCommit: true})
	r.write("a.txt", "a\n")
	r.checkpoint("c1", false)
	if got := r.git(r.root, "ls-tree", "-r", "--name-only", cpNS+"c1/1"); got != "a.txt" {
		t.Fatalf("tree = %q", got)
	}
	if st := r.git(r.root, "status", "--porcelain"); st != "?? a.txt" {
		t.Fatalf("status = %q", st)
	}
}

func TestCheckpointPrunesToTwentyTurns(t *testing.T) {
	r := newRig(t, opts{})
	for i := 1; i <= 25; i++ {
		r.write("f.txt", strconv.Itoa(i)+"\n")
		r.checkpoint("c1", false)
		r.checkpoint("c1", true)
	}
	refs := r.cpRefNames("c1")
	if len(refs) != 40 {
		t.Fatalf("%d refs kept, want 40: %v", len(refs), refs)
	}
	// Turn 6 is the oldest kept: its start is ref 11.
	if got := r.git(r.root, "show", cpNS+"c1/11:f.txt"); got != "6" {
		t.Fatalf("oldest kept start has f.txt = %q, want 6", got)
	}
	r.checkpoint("c2", false)
	if len(r.cpRefNames("c1")) != 40 || len(r.cpRefNames("c2")) != 1 {
		t.Fatal("chats must not prune each other")
	}
}

func TestDropCheckpointsDeletesOneChat(t *testing.T) {
	r := newRig(t, opts{})
	r.checkpoint("c1", false)
	r.checkpoint("c1", true)
	r.checkpoint("c2", false)
	r.must(r.s.DropCheckpoints(context.Background(), r.root, "c1"))
	if len(r.cpRefNames("c1")) != 0 || len(r.cpRefNames("c2")) != 1 {
		t.Fatalf("refs after drop: c1 %v, c2 %v", r.cpRefNames("c1"), r.cpRefNames("c2"))
	}
}

func TestCheckpointOutsideARepositoryDoesNothing(t *testing.T) {
	r := newRig(t, opts{noRepo: true})
	r.must(r.s.Checkpoint(context.Background(), "p1", r.root, "c1", false))
	r.must(r.s.DropCheckpoints(context.Background(), r.root, "c1"))
	if _, err := os.Stat(filepath.Join(r.root, ".git")); !os.IsNotExist(err) {
		t.Fatalf("a .git folder appeared: %v", err)
	}
	res, err := r.s.LastTurn(context.Background(), "p1", "c1")
	r.must(err)
	if res.Turn != nil || len(res.Files) != 0 {
		t.Fatalf("last turn = %+v", res)
	}
}

func TestCheckpointOffSwitch(t *testing.T) {
	r := newRig(t, opts{})
	r.s.SetCheckpointGate(func(id string) bool { return false })
	r.checkpoint("c1", false)
	r.checkpoint("c1", true)
	if n := len(r.cpRefNames("c1")); n != 0 {
		t.Fatalf("%d refs written with checkpoints off", n)
	}
	r.s.SetCheckpointGate(func(id string) bool { return id == "p1" })
	r.checkpoint("c1", false)
	if n := len(r.cpRefNames("c1")); n != 1 {
		t.Fatalf("%d refs with checkpoints on", n)
	}
}

func TestCheckpointRejectsABadChatID(t *testing.T) {
	r := newRig(t, opts{})
	if err := r.s.Checkpoint(context.Background(), "p1", r.root, "../x", false); err == nil {
		t.Fatal("a chat id with a slash was accepted")
	}
}
