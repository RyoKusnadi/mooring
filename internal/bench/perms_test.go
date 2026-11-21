package bench

import (
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
