package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatesAndKeepsThreeOldFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "mooring.log")
	w, err := Open(path, 100, 3)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		line := strings.Repeat(string(rune('a'+i)), 60) + "\n"
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return string(b)
	}
	if !strings.HasPrefix(read(path), "f") || !strings.HasPrefix(read(path+".1"), "e") ||
		!strings.HasPrefix(read(path+".2"), "d") || !strings.HasPrefix(read(path+".3"), "c") {
		t.Fatal("unexpected rotation order")
	}
	if _, err := os.Stat(path + ".4"); err == nil {
		t.Fatal("kept more than 3 old files")
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode())
	}
}
