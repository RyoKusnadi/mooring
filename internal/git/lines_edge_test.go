package git

import (
	"testing"
)

func TestStageLinesNoNewline(t *testing.T) {
	r := newRig(t, opts{})
	// New side lacks the final newline.
	r.write("a.txt", "a\nb\nc\n")
	r.commitAll("a")
	r.write("a.txt", "a\nb\nc\nd")
	d := r.diff("a.txt", false)
	r.must(r.stageLines("a.txt", false, d, "+d"))
	if got := r.blobRaw("a.txt"); got != "a\nb\nc\nd" {
		t.Fatalf("index %q", got)
	}
	r.commitAll("a2")
	// Replace the unterminated last line: choosing only the removal.
	r.write("a.txt", "a\nb\nc\nD\n")
	d = r.diff("a.txt", false)
	r.must(r.stageLines("a.txt", false, d, "-d"))
	if got := r.blobRaw("a.txt"); got != "a\nb\nc\n" {
		t.Fatalf("index %q", got)
	}
	// Old side lacks it; the new line alone cannot be staged.
	r.write("b.txt", "a\nb")
	r.commitAll("b")
	r.write("b.txt", "a\nB\nc\n")
	d = r.diff("b.txt", false)
	r.wantCode(r.stageLines("b.txt", false, d, "+c"), "BAD_REQUEST")
	r.must(r.stageLines("b.txt", false, d, "-b", "+B"))
	if got := r.blobRaw("b.txt"); got != "a\nB\n" {
		t.Fatalf("index %q", got)
	}
	d = r.diff("b.txt", false)
	r.must(r.stageLines("b.txt", false, d, "+c"))
	if got := r.blobRaw("b.txt"); got != "a\nB\nc\n" {
		t.Fatalf("index %q", got)
	}
	// Unstage, with the marker on the other side.
	sd := r.diff("b.txt", true)
	r.must(r.stageLines("b.txt", true, sd, "+c"))
	if got := r.blobRaw("b.txt"); got != "a\nB\n" {
		t.Fatalf("after unstage %q", got)
	}
	// A new file without a final newline.
	r.write("e.txt", "x\ny")
	nd := r.diff("e.txt", false)
	r.must(r.stageLines("e.txt", false, nd, "+y"))
	if got := r.blobRaw("e.txt"); got != "y" {
		t.Fatalf("new file %q", got)
	}
}

func TestStageLinesRefused(t *testing.T) {
	r := newRig(t, opts{})
	r.write("bin.dat", "a\x00b\n")
	r.commitAll("bin")
	r.write("bin.dat", "a\x00c\n")
	d := r.diff("bin.dat", false)
	if !d.Binary {
		t.Fatal("not binary")
	}
	r.wantCode(r.s.StageLines(bg, "p1", "", "bin.dat", false, d.Version, [][2]int{{0, 0}}), "BAD_REQUEST")

	// Stale input, out of range and empty choices.
	r.write("t.txt", "1\n2\n")
	r.commitAll("t")
	r.write("t.txt", "1\nx\n")
	d = r.diff("t.txt", false)
	r.wantCode(r.s.StageLines(bg, "p1", "", "t.txt", false, d.Version, [][2]int{{5, 0}}), "BAD_REQUEST")
	r.wantCode(r.s.StageLines(bg, "p1", "", "t.txt", false, d.Version, nil), "BAD_REQUEST")
	r.wantCode(r.s.StageLines(bg, "p1", "", "t.txt", false, d.Version, [][2]int{{0, 0}}), "BAD_REQUEST") // a context line
	r.wantCode(r.s.StageLines(bg, "p1", "", "../x", false, d.Version, [][2]int{{0, 0}}), "OUTSIDE_PROJECT")

	// Changed after the diff was read.
	r.write("t.txt", "1\ny\n")
	r.wantCode(r.stageLines("t.txt", false, d, "-2", "+x"), "DIFF_CHANGED")

	// Staged rename.
	r.write("old.txt", numbered("keep", 20, "\n"))
	r.commitAll("old")
	r.git(r.root, "mv", "old.txt", "new.txt")
	rd := r.diff("new.txt", true)
	if rd.OrigPath != "old.txt" {
		t.Fatalf("diff %+v", rd)
	}
	r.write("new.txt", numbered("keep", 20, "\n")+"more\n")
	r.git(r.root, "add", "new.txt")
	r.wantCode(r.s.StageLines(bg, "p1", "", "new.txt", true, rd.Version, [][2]int{{0, 0}}), "BAD_REQUEST")

	// Conflicted.
	r.write("m.txt", "base\n")
	r.commitAll("m")
	r.git(r.root, "checkout", "-q", "-b", "other")
	r.write("m.txt", "other\n")
	r.commitAll("other")
	r.git(r.root, "checkout", "-q", "main")
	r.write("m.txt", "main\n")
	r.commitAll("main")
	if _, err := gitOut(r, "merge", "other"); err == nil {
		t.Fatal("merge did not conflict")
	}
	cd := r.diff("m.txt", false)
	r.wantCode(r.s.StageLines(bg, "p1", "", "m.txt", false, cd.Version, [][2]int{{0, 0}}), "BAD_REQUEST")
}

// A change of ten lines in a row is one block: the chosen new lines come after
// the old lines that stay.
func TestStageLinesBlock(t *testing.T) {
	r := newRig(t, opts{})
	r.write("f.txt", numbered("old", 10, "\n"))
	r.commitAll("f")
	r.write("f.txt", numbered("new", 10, "\n"))
	d := r.diff("f.txt", false)
	r.must(r.stageLines("f.txt", false, d, "-old3", "+new3", "-old7", "+new7"))
	if got, want := r.blobRaw("f.txt"), "old1\nold2\nold4\nold5\nold6\nold8\nold9\nold10\nnew3\nnew7\n"; got != want {
		t.Fatalf("index %q want %q", got, want)
	}
	sd := r.diff("f.txt", true)
	r.must(r.stageLines("f.txt", true, sd, "-old3", "+new3", "-old7", "+new7"))
	if r.git(r.root, "diff", "--cached", "--stat") != "" {
		t.Fatal("index differs from HEAD")
	}
}
