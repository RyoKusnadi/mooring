package paths

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
)

// FS is the part of a file system the resolver needs. An SFTP client satisfies it.
type FS interface {
	Lstat(p string) (os.FileInfo, error)
	ReadLink(p string) (string, error)
}

const maxLinks = 40

// ResolveIn is Resolve for a slash-separated file system that is not the local
// one. root and p are absolute, or p is relative to root. Links are followed
// one component at a time with Lstat and ReadLink, so the remote side's own
// path rules decide where they lead. The result is inside the resolved root.
func ResolveIn(fsys FS, root, p string) (string, error) {
	realRoot, err := RootIn(fsys, root)
	if err != nil {
		return "", err
	}
	if p == "" {
		p = realRoot
	} else if !path.IsAbs(p) {
		p = path.Join(realRoot, p)
	}
	resolved, err := evalIn(fsys, path.Clean(p))
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", p, err)
	}
	if !Inside(realRoot, resolved) {
		return "", ErrOutsideRoot
	}
	return resolved, nil
}

// RootIn returns the root folder with its links resolved. It must exist.
func RootIn(fsys FS, root string) (string, error) {
	if !path.IsAbs(root) {
		return "", fmt.Errorf("resolve root %q: not an absolute path", root)
	}
	real, err := evalIn(fsys, path.Clean(root))
	if err != nil {
		return "", fmt.Errorf("resolve root %q: %w", root, err)
	}
	if _, err := fsys.Lstat(real); err != nil {
		return "", fmt.Errorf("resolve root %q: %w", root, err)
	}
	return real, nil
}

type part struct {
	name   string
	linked bool // came from the target of a link
}

func parts(p string, linked bool) []part {
	var out []part
	for _, n := range strings.Split(p, "/") {
		out = append(out, part{n, linked})
	}
	return out
}

// evalIn resolves every link of p. Components that do not exist yet are kept
// as they are, but a link to a missing target is an escape, as locally: a
// later write would create the target wherever the link points.
func evalIn(fsys FS, p string) (string, error) {
	pending := parts(strings.TrimPrefix(p, "/"), false)
	cur := "/"
	links := 0
	for len(pending) > 0 {
		cp := pending[0]
		pending = pending[1:]
		switch cp.name {
		case "", ".":
			continue
		case "..":
			cur = path.Dir(cur)
			continue
		}
		next := path.Join(cur, cp.name)
		fi, err := fsys.Lstat(next)
		if errors.Is(err, fs.ErrNotExist) {
			if cp.linked {
				return "", fmt.Errorf("%s is a link to a missing target: %w", cur, ErrOutsideRoot)
			}
			rest := []string{next}
			for _, r := range pending {
				rest = append(rest, r.name)
			}
			return path.Join(rest...), nil
		}
		if err != nil {
			return "", err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			cur = next
			continue
		}
		if links++; links > maxLinks {
			return "", fmt.Errorf("%s: too many links", next)
		}
		target, err := fsys.ReadLink(next)
		if err != nil {
			return "", err
		}
		if path.IsAbs(target) {
			cur = "/"
		}
		pending = append(parts(target, true), pending...)
	}
	return cur, nil
}
