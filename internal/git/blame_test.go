package git

import "testing"

func TestBlame(t *testing.T) {
	r := newRig(t, opts{subfolder: "sub"})
	r.write("sub/a.txt", "one\ntwo\nthree\n")
	r.commitAll("add a")
	first := r.git(r.root, "rev-parse", "HEAD")
	r.write("sub/a.txt", "one\nTWO\nthree\n")
	r.git(r.root, "-c", "user.name=Other", "commit", "-q", "-am", "change two")
	second := r.git(r.root, "rev-parse", "HEAD")
	r.write("sub/a.txt", "one\nTWO\nthree\nfour\n") // not committed

	b, err := r.s.Blame(bg, "p1", "", "a.txt")
	r.must(err)
	want := []BlameRun{{1, 1, first}, {2, 1, second}, {3, 1, first}, {4, 1, ""}}
	if len(b.Runs) != len(want) {
		t.Fatalf("runs %+v", b.Runs)
	}
	for i, w := range want {
		if b.Runs[i] != w {
			t.Fatalf("run %d = %+v, want %+v", i, b.Runs[i], w)
		}
	}
	if c := b.Commits[second]; c.Author != "Other" || c.Summary != "change two" || c.Time == 0 {
		t.Fatalf("second %+v", c)
	}
	if c := b.Commits[first]; c.Author != "Test" || c.Summary != "add a" {
		t.Fatalf("first %+v", c)
	}
	if len(b.Commits) != 2 {
		t.Fatalf("commits %+v", b.Commits)
	}

	r.write("sub/new.txt", "x\n")
	_, err = r.s.Blame(bg, "p1", "", "new.txt")
	r.wantCode(err, "NOT_TRACKED")
	_, err = r.s.Blame(bg, "p1", "", "../README.md")
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Blame(bg, "p1", "", "")
	r.wantCode(err, "BAD_REQUEST")
	r.write("sub/dir/x.txt", "x\n")
	_, err = r.s.Blame(bg, "p1", "", "dir")
	r.wantCode(err, "BAD_REQUEST")
}

func TestBlameNoCommits(t *testing.T) {
	r := newRig(t, opts{noCommit: true})
	r.write("a.txt", "a\n")
	_, err := r.s.Blame(bg, "p1", "", "a.txt")
	r.wantCode(err, "NOT_TRACKED")
}

func TestParseBlameJoinsRuns(t *testing.T) {
	a := "1111111111111111111111111111111111111111"
	out := a + " 1 1 2\nauthor A\nauthor-time 5\nsummary s\nfilename f\n\tx\n" + a + " 2 2\n\ty\n" + a + " 3 3 1\n\tz\n"
	b := parseBlame([]byte(out))
	if len(b.Runs) != 1 || b.Runs[0] != (BlameRun{1, 3, a}) || b.Commits[a].Time != 5 {
		t.Fatalf("%+v", b)
	}
}
