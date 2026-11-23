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
	dir = realPath(dir)
	for _, path := range p.Paths {
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		rel, err := filepath.Rel(dir, realPath(path))
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

// realPath resolves links in the part of path that exists, so one folder named
// two ways (on macOS /var is /private/var) compares equal. A file the agent is
// about to create does not exist yet; its missing tail is kept as written.
func realPath(path string) string {
	path = filepath.Clean(path)
	tail := ""
	for {
		if real, err := filepath.EvalSymlinks(path); err == nil {
			return filepath.Join(real, tail)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return filepath.Join(path, tail)
		}
		tail = filepath.Join(filepath.Base(path), tail)
		path = parent
	}
}
