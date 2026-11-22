package git

import (
	"strings"
	"testing"
)

func TestUncommittedAgainstHead(t *testing.T) {
	r := newRig(t, opts{})
	r.write("sub/keep.txt", "keep\n")
	r.commitAll("second")
	stat, diff, err := r.s.Uncommitted(bg, "p1", "")
	r.must(err)
	if stat != "" || diff != "" {
		t.Fatalf("clean tree, got %q %q", stat, diff)
	}
	r.write("README.md", "hello\nworld\n") // changed, not staged
	r.write("sub/keep.txt", "kept\n")      // changed and staged
	r.write("sub/new.txt", "brand new\n")  // not tracked
	r.write(".gitignore", "*.log\n")       // not tracked
	r.write("sub/noise.log", "ignored\n")  // ignored
	r.git(r.root, "add", "sub/keep.txt")
	stat, diff, err = r.s.Uncommitted(bg, "p1", "")
	r.must(err)
	for _, want := range []string{"README.md", "sub/keep.txt", "New files, not in git yet:\n", "sub/new.txt", ".gitignore"} {
		if !strings.Contains(stat, want) {
			t.Fatalf("stat lacks %q:\n%s", want, stat)
		}
	}
	if strings.Contains(stat, "noise.log") || !strings.Contains(diff, "+world") || !strings.Contains(diff, "+kept") {
		t.Fatalf("stat %q diff %q", stat, diff)
	}

	// A subfolder sees only its own changes.
	stat, diff, err = r.s.Uncommitted(bg, "p1", "sub")
	r.must(err)
	if strings.Contains(stat, "README.md") || strings.Contains(diff, "+world") || !strings.Contains(diff, "+kept") {
		t.Fatalf("subfolder: %q %q", stat, diff)
	}
}

func TestUncommittedNeedsACommit(t *testing.T) {
	r := newRig(t, opts{noCommit: true})
	r.write("a.txt", "a\n")
	_, _, err := r.s.Uncommitted(bg, "p1", "")
	r.wantCode(err, "NO_COMMIT")
	_, _, err = newRig(t, opts{noRepo: true}).s.Uncommitted(bg, "p1", "")
	r.wantCode(err, "NOT_A_REPO")
}
