package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenInFolder(t *testing.T) {
	base, _ := filepath.EvalSymlinks(t.TempDir())
	proj, outside := filepath.Join(base, "proj"), filepath.Join(base, "outside")
	for _, d := range []string{filepath.Join(proj, "sub"), outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(proj, "link")); err != nil {
		t.Fatal(err)
	}
	cfg := script("pwd -P; sleep 30")
	m, _ := newManager(t, cfg)
	m.Dir = func(string) (string, error) { return proj, nil }

	if _, err := m.Open("p1", "sub", "a", 80, 24); err != nil {
		t.Fatalf("open in sub: %v", err)
	}
	term := m.get("a")
	waitFor(t, "pwd output", func() bool { return strings.Contains(term.ringText(), filepath.Join(proj, "sub")) })

	for folder, want := range map[string]string{"..": "OUTSIDE_PROJECT", "link": "OUTSIDE_PROJECT", "missing": "NOT_FOUND", "/etc": "BAD_REQUEST"} {
		_, err := m.Open("p1", folder, "b", 80, 24)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("folder %q: err %v, want %s", folder, err, want)
		}
	}
}
