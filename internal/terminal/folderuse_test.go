package terminal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFolderUse(t *testing.T) {
	base, _ := filepath.EvalSymlinks(t.TempDir())
	proj := filepath.Join(base, "proj")
	if err := os.MkdirAll(filepath.Join(proj, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	m, _ := newManager(t, script("sleep 30"))
	m.Dir = func(string) (string, error) { return proj, nil }

	if _, err := m.Open("p1", "sub", "a", 80, 24); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Open("p2", "", "b", 80, 24); err != nil {
		t.Fatal(err)
	}
	got := m.FolderUse("p1")
	if len(got) != 1 || got[0] != filepath.Join(proj, "sub") {
		t.Fatalf("p1 folders: %v", got)
	}
	if got := m.FolderUse("none"); len(got) != 0 {
		t.Fatalf("other project: %v", got)
	}
	m.Close("a")
	if got := m.FolderUse("p1"); len(got) != 0 {
		t.Fatalf("after close: %v", got)
	}
}
