package git

import (
	"os"
	"path/filepath"
	"testing"
)

const png1 = "\x89PNG\r\n\x1a\n-one"
const png2 = "\x89PNG\r\n\x1a\n-two"
const png3 = "\x89PNG\r\n\x1a\n-three"

func (r *rig) image(rel, commit string, staged, old bool) string {
	r.t.Helper()
	out, ct, err := r.s.Image(bg, "p1", "", rel, commit, staged, old)
	r.must(err)
	if ct != "image/png" {
		r.t.Fatalf("type %q", ct)
	}
	return string(out)
}

func TestImageSides(t *testing.T) {
	r := newRig(t, opts{})
	r.write("a.png", png1)
	r.commitAll("add")
	added := r.git(r.root, "rev-parse", "HEAD")
	r.write("a.png", png2)
	r.git(r.root, "add", "a.png")
	r.write("a.png", png3)

	// Staged: HEAD against the index. Not staged: the index against the file.
	if r.image("a.png", "", true, true) != png1 || r.image("a.png", "", true, false) != png2 {
		t.Fatal("staged sides")
	}
	if r.image("a.png", "", false, true) != png2 || r.image("a.png", "", false, false) != png3 {
		t.Fatal("working sides")
	}
	// A commit: its parent against itself. The first version has no earlier side.
	r.git(r.root, "commit", "-q", "-m", "change")
	changed := r.git(r.root, "rev-parse", "HEAD")
	if r.image("a.png", changed, false, true) != png1 || r.image("a.png", changed, false, false) != png2 {
		t.Fatal("commit sides")
	}
	_, _, err := r.s.Image(bg, "p1", "", "a.png", added, false, true)
	r.wantCode(err, "NOT_FOUND")

	// A renamed, staged image shows its old name's version.
	r.git(r.root, "mv", "a.png", "b.png")
	if r.image("b.png", "", true, true) != png2 {
		t.Fatal("renamed old side")
	}
}

func TestImageRefusals(t *testing.T) {
	r := newRig(t, opts{})
	r.write("fake.png", "<script>alert(1)</script>")
	r.write("note.txt", "text")
	r.commitAll("add")
	_, _, err := r.s.Image(bg, "p1", "", "fake.png", "", false, false)
	r.wantCode(err, "NOT_AN_IMAGE")
	_, _, err = r.s.Image(bg, "p1", "", "note.txt", "", true, true)
	r.wantCode(err, "NOT_AN_IMAGE")
	_, _, err = r.s.Image(bg, "p1", "", "../x.png", "", false, false)
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, _, err = r.s.Image(bg, "p1", "", "gone.png", "", false, true)
	r.wantCode(err, "NOT_FOUND")
	// A link in the working tree is not followed.
	outside := filepath.Join(r.base, "secret.png")
	_ = os.WriteFile(outside, []byte(png1), 0o600)
	if err := os.Symlink(outside, filepath.Join(r.root, "link.png")); err != nil {
		t.Fatal(err)
	}
	_, _, err = r.s.Image(bg, "p1", "", "link.png", "", false, false)
	r.wantCode(err, "NOT_FOUND")
}
