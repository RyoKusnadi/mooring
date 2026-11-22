package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyAFile(t *testing.T) {
	r := newRig(t, nil)
	r.write("a.txt", "hi")
	res, err := r.s.Copy("p1", "a.txt", "a copy.txt")
	if err != nil || res.Path != "a copy.txt" {
		t.Fatalf("%v %v", res, err)
	}
	if b, _ := os.ReadFile(filepath.Join(r.root, "a copy.txt")); string(b) != "hi" {
		t.Fatalf("content %q", b)
	}
	if _, err := os.Stat(filepath.Join(r.root, "a.txt")); err != nil {
		t.Fatal("source gone")
	}
}

func TestCopyAFolderSkipsLinks(t *testing.T) {
	r := newRig(t, nil)
	r.write("d/one.txt", "1")
	r.write("d/deep/two.txt", "2")
	if err := os.Symlink(r.outside+"/secret.txt", filepath.Join(r.root, "d/leak")); err != nil {
		t.Fatal(err)
	}
	res, err := r.s.Copy("p1", "d", "d copy")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != "d copy/leak" {
		t.Fatalf("skipped %v", res.Skipped)
	}
	if b, _ := os.ReadFile(filepath.Join(r.root, "d copy/deep/two.txt")); string(b) != "2" {
		t.Fatalf("content %q", b)
	}
	if _, err := os.Lstat(filepath.Join(r.root, "d copy/leak")); err == nil {
		t.Fatal("the link was copied")
	}
}

func TestCopyRefusals(t *testing.T) {
	r := newRig(t, nil)
	r.write("a.txt", "x")
	r.write("d/f", "x")
	r.link(r.outside+"/secret.txt", "leak")
	for name, tc := range map[string]struct{ from, to, code string }{
		"name taken":     {"a.txt", "d/f", "ALREADY_EXISTS"},
		"into itself":    {"d", "d/inner", "BAD_REQUEST"},
		"a link":         {"leak", "leak2", "NOT_A_FILE"},
		"source outside": {"../outside/secret.txt", "s.txt", "OUTSIDE_PROJECT"},
		"target outside": {"a.txt", "../outside/a.txt", "OUTSIDE_PROJECT"},
		"missing source": {"nope", "n2", "NOT_FOUND"},
		"project itself": {"", "x", "PROTECTED"},
		"through a link": {"a.txt", "leak/a.txt", "OUTSIDE_PROJECT"},
	} {
		if _, err := r.s.Copy("p1", tc.from, tc.to); code(err) != tc.code {
			t.Errorf("%s: got %v want %s", name, err, tc.code)
		}
	}
	if _, err := os.Stat(filepath.Join(r.outside, "a.txt")); err == nil {
		t.Fatal("copied outside the project")
	}
}
