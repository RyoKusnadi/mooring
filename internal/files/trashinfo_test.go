package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFreedesktopTrashKeepsInfo(t *testing.T) {
	r := newRig(t, func(c *Config) { c.TrashDir = filepath.Join(t.TempDir(), "Trash", "files") })
	r.write("a b.txt", "one")
	for range 2 {
		if del, err := r.s.Delete("p1", "a b.txt", false); err != nil || !del["trashed"] {
			t.Fatalf("trash: %v %v", del, err)
		}
		r.write("a b.txt", "two")
	}
	trash := r.s.trash
	items, _ := os.ReadDir(trash)
	infos, _ := os.ReadDir(filepath.Join(filepath.Dir(trash), "info"))
	if len(items) != 2 || len(infos) != 2 {
		t.Fatalf("items %v, infos %v", items, infos)
	}
	for _, it := range items {
		b, err := os.ReadFile(filepath.Join(filepath.Dir(trash), "info", it.Name()+".trashinfo"))
		want := "Path=" + strings.ReplaceAll(filepath.Join(r.root, "a b.txt"), " ", "%20") + "\n"
		if err != nil || !strings.HasPrefix(string(b), "[Trash Info]\n") || !strings.Contains(string(b), want) || !strings.Contains(string(b), "DeletionDate=") {
			t.Fatalf("info for %s: %q %v", it.Name(), b, err)
		}
	}
}

func TestXDGTrash(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	if got := xdgTrash("/home/u"); got != "/home/u/.local/share/Trash/files" {
		t.Fatalf("default: %s", got)
	}
	t.Setenv("XDG_DATA_HOME", "/data")
	if got := xdgTrash("/home/u"); got != "/data/Trash/files" || !freedesktop(got) {
		t.Fatalf("xdg: %s", got)
	}
	t.Setenv("XDG_DATA_HOME", "relative")
	if got := xdgTrash(""); got != "" {
		t.Fatalf("no home: %s", got)
	}
}
