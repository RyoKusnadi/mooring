package files

import (
	"bytes"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// runWalk searches without ripgrep: a walk that skips hidden entries, ignored
// folders, links, big and binary files.
func (r *searchRun) runWalk() error {
	start := relToRoot(r.root, r.dir)
	if start == "." {
		start = ""
	}
	r.walk(start, ignoreFor(r.root, start))
	return nil
}

func (r *searchRun) stopped() bool {
	return r.full || r.emitErr != nil || r.ctx.Err() != nil
}

func (r *searchRun) walk(rel string, ign ignoreSet) {
	des, err := os.ReadDir(filepath.Join(r.root, filepath.FromSlash(rel)))
	if err != nil {
		return
	}
	for _, de := range des {
		if r.stopped() {
			return
		}
		name := de.Name()
		if strings.HasPrefix(name, ".") || de.Type()&os.ModeSymlink != 0 {
			continue
		}
		child := path.Join(rel, name)
		switch {
		case de.IsDir():
			if ign.ignored(child, true) {
				continue
			}
			r.walk(child, ign.child(r.root, child))
		case de.Type().IsRegular():
			if ign.ignored(child, false) || (r.glob != nil && r.glob.match(child, false) == r.glob.neg) {
				continue
			}
			r.searchFile(child)
		}
	}
}

func (r *searchRun) searchFile(rel string) {
	real, err := r.s.follow(r.root, rel)
	if err != nil {
		return
	}
	data, err := ReadWhole(real, maxSearchFile)
	if errors.Is(err, errTooLarge) || err != nil {
		return
	}
	if bytes.IndexByte(data[:min(len(data), sniffLen)], 0) >= 0 {
		return
	}
	found := 0
	for n, line := range bytes.Split(data, []byte("\n")) {
		if r.stopped() || found >= perFileMatch {
			return
		}
		locs := r.re.FindAllIndex(line, -1)
		if len(locs) == 0 {
			continue
		}
		ranges := make([][2]int, len(locs))
		for i, l := range locs {
			ranges[i] = [2]int{l[0], l[1]}
		}
		found++
		if !r.add(rel, n+1, string(line), ranges) {
			return
		}
	}
}
