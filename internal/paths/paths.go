// Package paths holds the one function that decides whether a path from the
// browser may be used: resolve it, then check it against the root.
package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrOutsideRoot is returned when a path resolves outside the root folder.
var ErrOutsideRoot = errors.New("OUTSIDE_ROOT: path is outside the allowed folder")

// Resolve turns p (absolute, or relative to root) into a clean absolute path
// with symlinks resolved, and requires the result to be root or inside it.
// A path that does not exist yet is resolved through its deepest existing
// parent, so a symlinked parent cannot be used to escape the root.
// The returned path is on the real file system, so callers should compare it
// with Root(root), not with the root they passed in.
func Resolve(root, p string) (string, error) {
	realRoot, err := Root(root)
	if err != nil {
		return "", err
	}
	if p == "" {
		p = realRoot
	} else if !filepath.IsAbs(p) {
		p = filepath.Join(realRoot, p)
	}
	resolved, err := evalDeepest(filepath.Clean(p))
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", p, err)
	}
	if !Inside(realRoot, resolved) {
		return "", ErrOutsideRoot
	}
	return resolved, nil
}

// Root returns the root folder with symlinks resolved.
func Root(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root %q: %w", root, err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve root %q: %w", root, err)
	}
	return real, nil
}

// Inside reports whether p is root or below it. Both must be clean and absolute.
func Inside(root, p string) bool {
	if p == root {
		return true
	}
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(p, prefix)
}

// evalDeepest resolves symlinks in the longest existing prefix of p and
// appends the rest unchanged.
func evalDeepest(p string) (string, error) {
	var rest []string
	cur := p
	for {
		real, err := filepath.EvalSymlinks(cur)
		if err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				real = filepath.Join(real, rest[i])
			}
			return real, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		// A link whose target is missing would be followed by a later write,
		// possibly to a place outside the root, so it is not "not created yet".
		if _, lerr := os.Lstat(cur); lerr == nil {
			return "", fmt.Errorf("%s is a link to a missing target: %w", cur, ErrOutsideRoot)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", err
		}
		rest = append(rest, filepath.Base(cur))
		cur = parent
	}
}
