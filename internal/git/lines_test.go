package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func numbered(prefix string, n int, eol string) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%s%d%s", prefix, i, eol)
	}
	return b.String()
}

// pick returns the [hunk, line] pairs of the diff lines that read kind+text.
func pick(t *testing.T, d *Diff, want ...string) [][2]int {
	t.Helper()
	var res [][2]int
	for _, w := range want {
		found := false
		for hi, h := range d.Hunks {
			for li, l := range h.Lines {
				if l[0]+l[1] == w {
					res, found = append(res, [2]int{hi, li}), true
				}
			}
		}
		if !found {
			t.Fatalf("diff has no line %q", w)
		}
	}
	return res
}

func (r *rig) stageLines(path string, staged bool, d *Diff, want ...string) error {
	r.t.Helper()
	return r.s.StageLines(bg, "p1", "", path, staged, d.Version, pick(r.t, d, want...))
}

func (r *rig) blobRaw(path string) string {
	r.t.Helper()
	out, err := gitOut(r, "cat-file", "-p", ":"+path)
	r.must(err)
	return out
}

func gitOut(r *rig, args ...string) (string, error) {
	cmd := newCmd(r, args...)
	out, err := cmd.Output()
	return string(out), err
}

// mix is a file of n lines: L<i>, but N<i> for the even i listed in news.
func mix(n int, eol string, news ...int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		p := "L"
		for _, k := range news {
			if k == i {
				p = "N"
			}
		}
		fmt.Fprintf(&b, "%s%d%s", p, i, eol)
	}
	return b.String()
}

func TestStageLinesTracked(t *testing.T) {
	r := newRig(t, opts{})
	r.write("f.txt", mix(10, "\n"))
	r.commitAll("f")
	r.write("f.txt", mix(10, "\n", 2, 4, 6, 8, 10))
	d := r.diff("f.txt", false)
	if len(d.Version) != 32 {
		t.Fatalf("version %q", d.Version)
	}
	r.must(r.stageLines("f.txt", false, d, "-L4", "+N4", "-L8", "+N8"))
	if got, want := r.blobRaw("f.txt"), mix(10, "\n", 4, 8); got != want {
		t.Fatalf("index:\n%s\nwant:\n%s", got, want)
	}
	if r.read("f.txt") != mix(10, "\n", 2, 4, 6, 8, 10) {
		t.Fatal("working file changed")
	}
	// The same diff again is stale.
	r.wantCode(r.stageLines("f.txt", false, d, "-L6", "+N6"), "DIFF_CHANGED")
	st := find(r.status(""), "f.txt")
	if st.Staged != "modified" || st.Unstaged != "modified" {
		t.Fatalf("status %+v", st)
	}
	// Unstage one of the two again.
	sd := r.diff("f.txt", true)
	r.must(r.stageLines("f.txt", true, sd, "-L4", "+N4"))
	if got, want := r.blobRaw("f.txt"), mix(10, "\n", 8); got != want {
		t.Fatalf("after unstage:\n%s\nwant:\n%s", got, want)
	}
	// Unstage the rest: the index is HEAD again.
	sd = r.diff("f.txt", true)
	var all []string
	for _, l := range sd.Hunks[0].Lines {
		if l[0] != " " {
			all = append(all, l[0]+l[1])
		}
	}
	r.must(r.stageLines("f.txt", true, sd, all...))
	if r.git(r.root, "diff", "--cached", "--stat") != "" {
		t.Fatal("index differs from HEAD")
	}
	// Block: stage a whole hunk with all its changed lines.
	d = r.diff("f.txt", false)
	all = nil
	for _, l := range d.Hunks[0].Lines {
		if l[0] != " " {
			all = append(all, l[0]+l[1])
		}
	}
	r.must(r.stageLines("f.txt", false, d, all...))
	if r.blobRaw("f.txt") != mix(10, "\n", 2, 4, 6, 8, 10) {
		t.Fatal("full stage differs")
	}
	if n := r.rec.count("git.changed"); n < 4 {
		t.Fatalf("git.changed published %d times", n)
	}
}

func TestStageLinesUntracked(t *testing.T) {
	r := newRig(t, opts{})
	r.write("n.txt", numbered("line", 10, "\n"))
	d := r.diff("n.txt", false)
	r.must(r.stageLines("n.txt", false, d, "+line3", "+line7"))
	if got := r.blobRaw("n.txt"); got != "line3\nline7\n" {
		t.Fatalf("index %q", got)
	}
	st := find(r.status(""), "n.txt")
	if st.Staged != "added" || st.Unstaged != "modified" {
		t.Fatalf("status %+v", st)
	}
	// Now it is tracked: stage another line the normal way.
	d = r.diff("n.txt", false)
	r.must(r.stageLines("n.txt", false, d, "+line5"))
	if got := r.blobRaw("n.txt"); got != "line3\nline5\nline7\n" {
		t.Fatalf("index %q", got)
	}
	// Unstage line5 and line3: line7 stays; then the rest removes the file from the index.
	sd := r.diff("n.txt", true)
	r.must(r.stageLines("n.txt", true, sd, "+line5", "+line3"))
	if got := r.blobRaw("n.txt"); got != "line7\n" {
		t.Fatalf("index %q", got)
	}
	sd = r.diff("n.txt", true)
	r.must(r.stageLines("n.txt", true, sd, "+line7"))
	if _, err := gitOut(r, "cat-file", "-p", ":n.txt"); err == nil {
		t.Fatal("file still in the index")
	}
	if st := find(r.status(""), "n.txt"); st == nil || st.Kind != "untracked" {
		t.Fatalf("status %+v", st)
	}
}

func TestStageLinesCRLF(t *testing.T) {
	r := newRig(t, opts{})
	r.git(r.root, "config", "core.autocrlf", "false")
	r.write("c.txt", mix(10, "\r\n"))
	r.commitAll("c")
	r.write("c.txt", mix(10, "\r\n", 2, 4, 6, 8, 10))
	d := r.diff("c.txt", false)
	r.must(r.s.StageLines(bg, "p1", "", "c.txt", false, d.Version, pick(t, d, "-L4\r", "+N4\r", "-L8\r", "+N8\r")))
	got, want := r.blobRaw("c.txt"), mix(10, "\r\n", 4, 8)
	if got != want {
		t.Fatalf("index bytes %q want %q", got, want)
	}
	sd := r.diff("c.txt", true)
	r.must(r.s.StageLines(bg, "p1", "", "c.txt", true, sd.Version, pick(t, sd, "-L8\r", "+N8\r")))
	if got, want := r.blobRaw("c.txt"), mix(10, "\r\n", 4); got != want {
		t.Fatalf("after unstage %q want %q", got, want)
	}
	// New CRLF file.
	r.write("n.txt", numbered("x", 10, "\r\n"))
	nd := r.diff("n.txt", false)
	r.must(r.s.StageLines(bg, "p1", "", "n.txt", false, nd.Version, pick(t, nd, "+x4\r", "+x5\r")))
	if got := r.blobRaw("n.txt"); got != "x4\r\nx5\r\n" {
		t.Fatalf("new file bytes %q", got)
	}
}

func TestStageLinesDeleted(t *testing.T) {
	r := newRig(t, opts{})
	r.write("d.txt", numbered("l", 10, "\n"))
	r.commitAll("d")
	if err := os.Remove(filepath.Join(r.root, "d.txt")); err != nil {
		t.Fatal(err)
	}
	d := r.diff("d.txt", false)
	r.must(r.stageLines("d.txt", false, d, "-l2", "-l3"))
	if got, want := r.blobRaw("d.txt"), "l1\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"; got != want {
		t.Fatalf("index %q", got)
	}
	d = r.diff("d.txt", false)
	var rest []string
	for _, l := range d.Hunks[0].Lines {
		if l[0] == "-" {
			rest = append(rest, l[0]+l[1])
		}
	}
	r.must(r.stageLines("d.txt", false, d, rest...))
	if st := find(r.status(""), "d.txt"); st == nil || st.Staged != "deleted" || st.Unstaged != "" {
		t.Fatalf("status %+v", st)
	}
	// Unstage part of the staged delete: the file returns with those lines.
	sd := r.diff("d.txt", true)
	r.must(r.stageLines("d.txt", true, sd, "-l5", "-l6"))
	if got := r.blobRaw("d.txt"); got != "l5\nl6\n" {
		t.Fatalf("index %q", got)
	}
	sd = r.diff("d.txt", true)
	r.must(r.stageLines("d.txt", true, sd, "-l1", "-l2", "-l3", "-l4", "-l7", "-l8", "-l9", "-l10"))
	if r.blobRaw("d.txt") != numbered("l", 10, "\n") || r.git(r.root, "diff", "--cached", "--stat") != "" {
		t.Fatal("index differs from HEAD")
	}
}
