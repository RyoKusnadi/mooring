package paths

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// localFS lets the remote resolver run on a real folder.
type localFS struct{}

func (localFS) Lstat(p string) (os.FileInfo, error) { return os.Lstat(p) }
func (localFS) ReadLink(p string) (string, error)   { return os.Readlink(p) }

func TestResolveInMatchesLocal(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "a", "b"), 0o755))
	must(os.Symlink("a/b", filepath.Join(root, "inner")))
	must(os.Symlink(outside, filepath.Join(root, "out")))
	must(os.Symlink("nothing", filepath.Join(root, "dangling")))
	must(os.Symlink("../../..", filepath.Join(root, "a", "up")))
	cases := []string{"", "a", "a/b", "inner", "inner/new.txt", "a/new/deeper.txt", "out", "out/x", "dangling", "dangling/x", "a/up/etc", "a/../a", "../x"}
	for _, c := range cases {
		want, wantErr := Resolve(root, c)
		got, gotErr := ResolveIn(localFS{}, root, c)
		if (wantErr == nil) != (gotErr == nil) || want != got {
			t.Errorf("%q: local %q, %v; remote %q, %v", c, want, wantErr, got, gotErr)
			continue
		}
		if wantErr != nil && errors.Is(wantErr, ErrOutsideRoot) != errors.Is(gotErr, ErrOutsideRoot) {
			t.Errorf("%q: local %v, remote %v", c, wantErr, gotErr)
		}
	}
}

func TestRootInNeedsAFolderThatExists(t *testing.T) {
	_, err := RootIn(localFS{}, filepath.Join(t.TempDir(), "missing"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}
