package bench

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ryokusnadi/mooring/internal/chat"
)

func TestChoosePermission(t *testing.T) {
	opts := []chat.Option{{ID: "a", Kind: "allow_once"}, {ID: "r", Kind: "reject_once"}}
	cases := []struct {
		paths []string
		want  string
	}{
		{nil, "a"},
		{[]string{"main.go"}, "a"},
		{[]string{"/work/x/internal/a.go"}, "a"},
		{[]string{"/etc/passwd"}, "r"},
		{[]string{"../outside.txt"}, "r"},
		{[]string{"ok.go", "/work/y/b.go"}, "r"},
	}
	for _, c := range cases {
		got, _ := choosePermission("/work/x", chat.OpenPermission{Paths: c.paths, Options: opts})
		if got != c.want {
			t.Errorf("%v: got %q, want %q", c.paths, got, c.want)
		}
	}
	if got, _ := choosePermission("/work/x", chat.OpenPermission{Options: []chat.Option{{ID: "r", Kind: "reject_once"}}}); got != "" {
		t.Errorf("no allow option: got %q", got)
	}
}

// The task folder and the agent's paths may name the same place through a link,
// as /var and /private/var do on macOS.
func TestChoosePermissionThroughALink(t *testing.T) {
	real := t.TempDir()
	if err := os.MkdirAll(filepath.Join(real, "task", "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("no symlinks here:", err)
	}
	opts := []chat.Option{{ID: "a", Kind: "allow_once"}, {ID: "r", Kind: "reject_once"}}
	cases := []struct{ dir, path, want string }{
		{filepath.Join(link, "task"), filepath.Join(real, "task", "internal", "a.go"), "a"},
		{filepath.Join(real, "task"), filepath.Join(link, "task", "internal", "new", "b.go"), "a"},
		{filepath.Join(link, "task"), filepath.Join(real, "other.go"), "r"},
	}
	for _, c := range cases {
		if got, _ := choosePermission(c.dir, chat.OpenPermission{Paths: []string{c.path}, Options: opts}); got != c.want {
			t.Errorf("dir %s, path %s: got %q, want %q", c.dir, c.path, got, c.want)
		}
	}
}
