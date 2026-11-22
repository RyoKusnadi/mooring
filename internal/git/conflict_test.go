package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// conflict makes main and side change a.txt differently and merges side into main.
func conflictRig(t *testing.T) *rig {
	r := newRig(t, opts{})
	r.write("a.txt", "base\n")
	r.write("gone.txt", "kept here\n")
	r.commitAll("base")
	r.git(r.root, "checkout", "-q", "-b", "side")
	r.write("a.txt", "theirs\n")
	r.write("gone.txt", "changed there\n")
	r.commitAll("side")
	r.git(r.root, "checkout", "-q", "main")
	r.write("a.txt", "ours\n")
	r.must(os.Remove(filepath.Join(r.root, "gone.txt")))
	r.commitAll("main")
	r.gitFail("merge", "side")
	return r
}

func (r *rig) gitFail(args ...string) {
	r.t.Helper()
	if _, err := r.s.run(bg, r.root, true, nil, args...); err == nil {
		r.t.Fatalf("git %v did not fail", args)
	}
}

func TestResolveAMergeAndCommit(t *testing.T) {
	r := conflictRig(t)
	st := r.status("")
	if !st.Conflicted || st.Operation != "merge" {
		t.Fatalf("status %+v", st)
	}
	// Committing and continuing wait until every file is resolved.
	_, err := r.s.Finish(bg, "p1", "", "continue")
	r.wantCode(err, "CONFLICTED")
	r.wantCode(r.s.Resolve(bg, "p1", "", []string{"a.txt"}, "mark"), "HAS_MARKERS")
	// The unstaged diff of a conflicted file shows its markers.
	d, err := r.s.Diff(bg, "p1", "", "a.txt", false)
	r.must(err)
	if !strings.Contains(fmt.Sprint(d.Hunks), "<<<<<<<") {
		t.Fatalf("conflict diff %+v", d)
	}
	r.wantCode(r.s.Resolve(bg, "p1", "", []string{"README.md"}, "ours"), "NOT_CONFLICTED")
	r.wantCode(r.s.Resolve(bg, "p1", "", []string{""}, "ours"), "BAD_REQUEST")
	r.wantCode(r.s.Resolve(bg, "p1", "", []string{"a.txt"}, "both"), "BAD_REQUEST")

	r.must(r.s.Resolve(bg, "p1", "", []string{"a.txt"}, "theirs"))
	if r.read("a.txt") != "theirs\n" {
		t.Fatalf("a.txt %q", r.read("a.txt"))
	}
	// Our side deleted gone.txt; keeping ours deletes it.
	r.must(r.s.Resolve(bg, "p1", "", []string{"gone.txt"}, "ours"))
	if _, err := os.Stat(filepath.Join(r.root, "gone.txt")); !os.IsNotExist(err) {
		t.Fatal("gone.txt is still there")
	}
	if st := r.status(""); st.Conflicted || st.Operation != "merge" {
		t.Fatalf("after resolve %+v", st)
	}
	res, err := r.s.Finish(bg, "p1", "", "continue")
	r.must(err)
	if res["operation"] != "" {
		t.Fatalf("still %v", res)
	}
	if p := r.git(r.root, "rev-list", "--parents", "-n1", "HEAD"); len(strings.Fields(p)) != 3 {
		t.Fatalf("not a merge commit: %s", p)
	}
	_, err = r.s.Finish(bg, "p1", "", "abort")
	r.wantCode(err, "NO_OPERATION")
}

func TestMarkResolvedAfterEditing(t *testing.T) {
	r := conflictRig(t)
	r.write("a.txt", "both\n")
	r.must(r.s.Resolve(bg, "p1", "", []string{"a.txt"}, "mark"))
	r.wantCode(r.s.Resolve(bg, "p1", "", []string{"a.txt"}, "ours"), "NOT_CONFLICTED")
	r.must(r.s.Resolve(bg, "p1", "", []string{"gone.txt"}, "theirs"))
	if r.read("gone.txt") != "changed there\n" || r.status("").Conflicted {
		t.Fatal("theirs not kept")
	}
}

func TestCommitAMergeThatKeepsHead(t *testing.T) {
	r := conflictRig(t)
	r.must(r.s.Resolve(bg, "p1", "", []string{"a.txt", "gone.txt"}, "ours"))
	_, err := r.s.Commit(bg, "p1", "", "merge side")
	r.must(err)
	if r.status("").Operation != "" {
		t.Fatal("merge still running")
	}
	if p := r.git(r.root, "rev-list", "--parents", "-n1", "HEAD"); len(strings.Fields(p)) != 3 {
		t.Fatalf("not a merge commit: %s", p)
	}
}

func TestStageLeavesConflictsAlone(t *testing.T) {
	r := conflictRig(t)
	r.write("new.txt", "x\n")
	r.must(r.s.Stage(bg, "p1", "", []string{""}))
	if st := r.status(""); !st.Conflicted {
		t.Fatal("stage all settled the conflicts")
	}
	if r.git(r.root, "diff", "--cached", "--name-only", "--", "new.txt") != "new.txt" {
		t.Fatal("stage all skipped the other files")
	}
	r.wantCode(r.s.Stage(bg, "p1", "", []string{"a.txt"}), "HAS_MARKERS")
	r.write("a.txt", "done\n")
	r.must(r.s.Stage(bg, "p1", "", []string{"a.txt"}))
	if !hasMarkersIn("x\n<<<<<<<\ny\n") || hasMarkersIn("<<<<<<<<< no\n") {
		t.Fatal("marker match")
	}
}

func hasMarkersIn(text string) bool {
	p := filepath.Join(os.TempDir(), "mooring-marker-test")
	_ = os.WriteFile(p, []byte(text), 0o600)
	defer func() { _ = os.Remove(p) }()
	return hasMarkers(p)
}

func TestAbortAMerge(t *testing.T) {
	r := conflictRig(t)
	_, err := r.s.Finish(bg, "p1", "", "abort")
	r.must(err)
	if st := r.status(""); st.Conflicted || st.Operation != "" || r.read("a.txt") != "ours\n" {
		t.Fatalf("after abort %+v", st)
	}
	_, err = r.s.Finish(bg, "p1", "", "later")
	r.wantCode(err, "BAD_REQUEST")
}

func TestRebaseStopsAndContinues(t *testing.T) {
	r := newRig(t, opts{})
	r.write("a.txt", "base\n")
	r.commitAll("base")
	r.git(r.root, "checkout", "-q", "-b", "topic")
	r.write("a.txt", "topic\n")
	r.commitAll("topic one")
	r.write("b.txt", "more\n")
	r.commitAll("topic two")
	r.git(r.root, "checkout", "-q", "main")
	r.write("a.txt", "main\n")
	r.commitAll("main")
	r.git(r.root, "checkout", "-q", "topic")
	r.gitFail("rebase", "main")
	if st := r.status(""); st.Operation != "rebase" || !st.Conflicted {
		t.Fatalf("status %+v", st)
	}
	r.must(r.s.Resolve(bg, "p1", "", []string{"a.txt"}, "theirs")) // in a rebase, theirs is the commit being replayed
	res, err := r.s.Finish(bg, "p1", "", "continue")
	r.must(err)
	if res["operation"] != "" || r.read("a.txt") != "topic\n" || r.read("b.txt") != "more\n" {
		t.Fatalf("after continue %v", res)
	}
}
