package paths

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func setup(t *testing.T) (root, outside string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(base, "home")
	outside = filepath.Join(base, "outside")
	for _, d := range []string{filepath.Join(root, "a", "b"), outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root, outside
}

func TestResolveInsideAndRelative(t *testing.T) {
	root, _ := setup(t)
	for in, want := range map[string]string{
		"":                                  root,
		root:                                root,
		"a/b":                               filepath.Join(root, "a", "b"),
		filepath.Join(root, "a", "..", "a"): filepath.Join(root, "a"),
		"a/new/deeper":                      filepath.Join(root, "a", "new", "deeper"),
	} {
		got, err := Resolve(root, in)
		if err != nil || got != want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestResolveRejectsEscapes(t *testing.T) {
	root, outside := setup(t)
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{
		"..", "../outside", outside, "/", "a/../../outside",
		"link", "link/file", "link/new/deeper",
		filepath.Join(root, "link"),
	} {
		if _, err := Resolve(root, in); !errors.Is(err, ErrOutsideRoot) {
			t.Errorf("Resolve(%q) error = %v; want ErrOutsideRoot", in, err)
		}
	}
}

func TestResolveSiblingWithSamePrefixIsOutside(t *testing.T) {
	root, _ := setup(t)
	sibling := root + "-evil"
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(root, sibling); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("error = %v; want ErrOutsideRoot", err)
	}
}

func TestResolveSymlinkInsideRootIsFollowed(t *testing.T) {
	root, _ := setup(t)
	if err := os.Symlink(filepath.Join(root, "a"), filepath.Join(root, "short")); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(root, "short/b")
	if err != nil || got != filepath.Join(root, "a", "b") {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestResolveRejectsBrokenLink(t *testing.T) {
	root, outside := setup(t)
	link := filepath.Join(root, "dangling")
	if err := os.Symlink(filepath.Join(outside, "not-yet"), link); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{link, filepath.Join(link, "child")} {
		if _, err := Resolve(root, p); !errors.Is(err, ErrOutsideRoot) {
			t.Errorf("%s: got %v, want ErrOutsideRoot", p, err)
		}
	}
}
