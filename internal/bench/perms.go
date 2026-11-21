package bench

import (
	"path/filepath"
	"strings"

	"github.com/ryokusnadi/mooring/internal/chat"
)

// choosePermission answers a permission request without a person: allowed when
// every file the tool names is inside the task folder (or it names none), rejected
// otherwise. It returns "" when the agent offered no fitting option.
func choosePermission(dir string, p chat.OpenPermission) (optionID string, allowed bool) {
	allowed = true
	for _, path := range p.Paths {
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		rel, err := filepath.Rel(dir, filepath.Clean(path))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			allowed = false
		}
	}
	want := []string{"allow_once", "allow_always"}
	if !allowed {
		want = []string{"reject_once", "reject_always"}
	}
	for _, kind := range want {
		for _, o := range p.Options {
			if o.Kind == kind {
				return o.ID, allowed
			}
		}
	}
	return "", allowed
}
