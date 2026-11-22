package git

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryokusnadi/mooring/internal/events"
)

func TestBranches(t *testing.T) {
	r := newRig(t, opts{})
	r.git(r.root, "branch", "old")
	r.write("a.txt", "a\n")
	r.commitAll("newer")
	bs, err := r.s.ListBranches(bg, "p1", "")
	r.must(err)
	if bs.Current != "main" || len(bs.Branches) != 2 || bs.Branches[0].Name != "main" || !bs.Branches[0].Head || bs.Branches[1].Name != "old" || bs.Branches[0].Time == 0 {
		t.Fatalf("branches %+v", bs)
	}
	v, err := r.s.Switch(bg, "p1", "", "old")
	r.must(err)
	if v["branch"] != "old" || r.git(r.root, "branch", "--show-current") != "old" {
		t.Fatalf("switch %v", v)
	}
	_, err = r.s.Switch(bg, "p1", "", "missing")
	r.wantCode(err, "NOT_FOUND")
	_, err = r.s.Switch(bg, "p1", "", "--detach")
	r.wantCode(err, "BAD_REQUEST")
	// A switch that would lose changes is git's refusal.
	r.git(r.root, "checkout", "-q", "main")
	r.git(r.root, "checkout", "-q", "-b", "diverge", "HEAD~1")
	r.git(r.root, "checkout", "-q", "main")
	r.write("a.txt", "a2\n")
	r.commitAll("a2")
	r.git(r.root, "checkout", "-q", "diverge")
	r.write("a.txt", "dirty\n")
	_, err = r.s.Switch(bg, "p1", "", "main")
	r.wantCode(err, "GIT_FAILED")
	r.git(r.root, "clean", "-fq")

	v, err = r.s.BranchCreate(bg, "p1", "", "feature/x")
	r.must(err)
	if v["branch"] != "feature/x" || r.git(r.root, "branch", "--show-current") != "feature/x" {
		t.Fatalf("create %v", v)
	}
	_, err = r.s.BranchCreate(bg, "p1", "", "feature/x")
	r.wantCode(err, "ALREADY_EXISTS")
	for _, bad := range []string{"", "a b", "x..y", "-x", "@{-1}", "bad~", "end."} {
		_, err = r.s.BranchCreate(bg, "p1", "", bad)
		r.wantCode(err, "BAD_REQUEST")
	}
}

func TestSwitchBusy(t *testing.T) {
	r := newRig(t, opts{})
	r.git(r.root, "branch", "other")
	r.chats[r.root] = ChatUse{ID: "c1", Live: true}
	_, err := r.s.Switch(bg, "p1", "", "other")
	r.wantCode(err, "BUSY")
	_, err = r.s.BranchCreate(bg, "p1", "", "n")
	r.wantCode(err, "BUSY")
	r.chats[r.root] = ChatUse{ID: "c1", Live: false}
	if _, err = r.s.Switch(bg, "p1", "", "other"); err != nil {
		t.Fatal(err)
	}
	// A chat in a worktree does not block the project folder, and the reverse.
	wt, err := r.s.WorktreeAdd(bg, "p1", "")
	r.must(err)
	delete(r.chats, r.root)
	r.chats[filepath.Join(r.root, filepath.FromSlash(wt.Folder))] = ChatUse{ID: "c2", Live: true}
	if _, err = r.s.Switch(bg, "p1", "", "main"); err != nil {
		t.Fatalf("project folder blocked by a worktree chat: %v", err)
	}
	r.git(r.root, "branch", "w2")
	if _, err = r.s.BranchCreate(bg, "p1", wt.Folder, "inwt"); code(err) != "BUSY" {
		t.Fatalf("worktree place not busy: %v", err)
	}
}

func TestGitCommandsRoute(t *testing.T) {
	r := newRig(t, opts{})
	r.write("a.txt", "1\n2\n")
	r.commitAll("a")
	r.write("a.txt", "1\nx\n")
	d := r.diff("a.txt", false)
	args := `{"projectId":"p1","folder":"","path":"a.txt","staged":false,"version":"` + d.Version + `","lines":[[0,1],[0,2]]}`
	if _, err := r.s.Run(bg, events.Command{Name: "git.stageLines", Args: []byte(args)}); err != nil {
		t.Fatal(err)
	}
	if got := r.blobRaw("a.txt"); got != "1\nx\n" {
		t.Fatalf("index %q", got)
	}
	v, err := r.s.Run(bg, events.Command{Name: "git.branchCreate", Args: []byte(`{"projectId":"p1","name":"nb"}`)})
	r.must(err)
	if v.(map[string]string)["branch"] != "nb" {
		t.Fatalf("value %v", v)
	}
	_, err = r.s.Run(bg, events.Command{Name: "git.stashSave", Args: []byte(`{"projectId":"p1","untracked":true}`)})
	r.must(err)
	if !strings.Contains(r.git(r.root, "stash", "list"), "stash@{0}") {
		t.Fatal("no stash")
	}
}
