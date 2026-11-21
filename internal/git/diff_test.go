package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (r *rig) diff(path string, staged bool) *Diff {
	r.t.Helper()
	d, err := r.s.Diff(context.Background(), "p1", "", path, staged)
	if err != nil {
		r.t.Fatalf("diff %s: %v", path, err)
	}
	return d
}

func kinds(h Hunk) string {
	var b strings.Builder
	for _, l := range h.Lines {
		b.WriteString(l[0])
	}
	return b.String()
}

func TestDiffModifiedStagedAndUnstaged(t *testing.T) {
	r := newRig(t, opts{})
	r.write("f.txt", "a\nb\nc\n")
	r.commitAll("f")
	r.write("f.txt", "a\nB\nc\n")
	r.git(r.root, "add", "f.txt")
	r.write("f.txt", "a\nB\nC\n")

	d := r.diff("f.txt", true)
	if len(d.Hunks) != 1 || kinds(d.Hunks[0]) != " -+ " {
		t.Fatalf("staged: %+v", d)
	}
	h := d.Hunks[0]
	if h.OldStart != 1 || h.OldLines != 3 || h.NewStart != 1 || h.NewLines != 3 || h.Lines[1][1] != "b" || h.Lines[2][1] != "B" {
		t.Errorf("staged hunk: %+v", h)
	}
	u := r.diff("f.txt", false)
	if len(u.Hunks) != 1 || kinds(u.Hunks[0]) != "  -+" {
		t.Fatalf("unstaged: %+v", u)
	}
	if got := r.diff("README.md", false); len(got.Hunks) != 0 || got.Binary || got.Hunks == nil {
		t.Errorf("unchanged file: %+v", got)
	}
}

func TestDiffUntrackedAndDeleted(t *testing.T) {
	r := newRig(t, opts{})
	r.write("new file.txt", "x\ny\n")
	d := r.diff("new file.txt", false)
	if len(d.Hunks) != 1 || kinds(d.Hunks[0]) != "++" || d.Hunks[0].NewLines != 2 || d.Hunks[0].OldLines != 0 {
		t.Fatalf("untracked: %+v", d)
	}
	if err := os.Remove(filepath.Join(r.root, "README.md")); err != nil {
		t.Fatal(err)
	}
	d = r.diff("README.md", false)
	if len(d.Hunks) != 1 || kinds(d.Hunks[0]) != "-" {
		t.Fatalf("deleted: %+v", d)
	}
}

func TestDiffStagedRename(t *testing.T) {
	r := newRig(t, opts{})
	r.write("one.txt", "1\n2\n3\n4\n5\n6\n7\n8\n")
	r.commitAll("one")
	r.git(r.root, "mv", "one.txt", "two.txt")
	r.write("two.txt", "1\n2\n3\n4\n5\n6\n7\n8\n9\n")
	r.git(r.root, "add", "two.txt")
	d := r.diff("two.txt", true)
	if d.OrigPath != "one.txt" || d.Path != "two.txt" || len(d.Hunks) != 1 || kinds(d.Hunks[0]) != "   +" {
		t.Fatalf("rename: %+v", d)
	}
}

func TestDiffBinaryNoNewlineAndTooLarge(t *testing.T) {
	r := newRig(t, opts{})
	r.write("bin.dat", "a\x00b\x00c")
	if d := r.diff("bin.dat", false); !d.Binary || len(d.Hunks) != 0 {
		t.Fatalf("binary: %+v", d)
	}
	r.write("tail.txt", "no newline")
	d := r.diff("tail.txt", false)
	if len(d.Hunks) != 1 {
		t.Fatalf("no newline: %+v", d)
	}
	ls := d.Hunks[0].Lines
	if len(ls) != 2 || ls[1][0] != "\\" {
		t.Fatalf("marker missing: %+v", ls)
	}
	r.write("big.txt", strings.Repeat("0123456789abcdef\n", 3<<16))
	if d := r.diff("big.txt", false); !d.TooLarge || len(d.Hunks) != 0 {
		t.Fatalf("too large: tooLarge=%v hunks=%d", d.TooLarge, len(d.Hunks))
	}
}

func TestDiffPathChecks(t *testing.T) {
	r := newRig(t, opts{})
	_, err := r.s.Diff(context.Background(), "p1", "", "../x", false)
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Diff(context.Background(), "p1", "", "/etc/passwd", false)
	r.wantCode(err, "BAD_REQUEST")
	_, err = r.s.Diff(context.Background(), "p1", "", "", false)
	r.wantCode(err, "BAD_REQUEST")
}
