package git

import "testing"

func TestAmend(t *testing.T) {
	r := newRig(t, opts{})
	r.write("a.txt", "a\n")
	r.must(r.s.Stage(bg, "p1", "", []string{""}))
	_, err := r.s.Commit(bg, "p1", "", "add a")
	r.must(err)
	before := r.git(r.root, "rev-list", "--count", "HEAD")

	// Reword only, then add a staged file and keep the message.
	_, err = r.s.commit(bg, "p1", "", "add a, reworded", true)
	r.must(err)
	r.write("b.txt", "b\n")
	r.must(r.s.Stage(bg, "p1", "", []string{""}))
	_, err = r.s.commit(bg, "p1", "", "", true)
	r.must(err)
	if got := r.git(r.root, "log", "-1", "--format=%s"); got != "add a, reworded" {
		t.Errorf("subject %q", got)
	}
	if r.git(r.root, "rev-list", "--count", "HEAD") != before {
		t.Error("amend added a commit")
	}
	if r.git(r.root, "show", "--name-only", "--format=", "HEAD") != "a.txt\nb.txt" {
		t.Error("amend did not take the staged file")
	}

	// Once the upstream has it, amend is refused.
	r.git(r.root, "remote", "add", "origin", r.remote)
	_, err = r.s.Push(bg, "p1", "", true)
	r.must(err)
	_, err = r.s.commit(bg, "p1", "", "too late", true)
	r.wantCode(err, "ALREADY_PUSHED")
	if r.git(r.root, "log", "-1", "--format=%s") != "add a, reworded" {
		t.Error("refused amend still changed the commit")
	}
	// A new local commit can be amended again.
	r.write("c.txt", "c\n")
	r.commitAll("c")
	_, err = r.s.commit(bg, "p1", "", "c reworded", true)
	r.must(err)
}

func TestAmendRefusedWithoutUpstream(t *testing.T) {
	r := newRig(t, opts{})
	r.git(r.root, "remote", "add", "origin", r.remote)
	if r.status("").Pushed {
		t.Fatal("a commit no remote has reads as pushed")
	}
	// Pushed without -u: no upstream, but a remote branch has the commit.
	r.git(r.root, "push", "origin", "main")
	if st := r.status(""); st.Upstream != "" || !st.Pushed {
		t.Fatalf("status %+v", st)
	}
	_, err := r.s.commit(bg, "p1", "", "late", true)
	r.wantCode(err, "ALREADY_PUSHED")

	// Detached at the remote branch: refused too. A local commit on top is fine.
	r.git(r.root, "checkout", "-q", "--detach", "origin/main")
	_, err = r.s.commit(bg, "p1", "", "late", true)
	r.wantCode(err, "ALREADY_PUSHED")
	r.write("d.txt", "d\n")
	r.commitAll("d")
	if r.status("").Pushed {
		t.Fatal("a local commit on a detached HEAD reads as pushed")
	}
	_, err = r.s.commit(bg, "p1", "", "d reworded", true)
	r.must(err)
}

func TestAmendNeedsACommit(t *testing.T) {
	r := newRig(t, opts{noCommit: true})
	_, err := r.s.commit(bg, "p1", "", "x", true)
	r.wantCode(err, "BAD_REQUEST")
}
