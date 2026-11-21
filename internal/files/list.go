package files

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// MaxEntries is how many entries one listing returns.
const MaxEntries = 5000

// Entry is one item of a folder.
type Entry struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Size    int64  `json:"size"`
	MTime   int64  `json:"mtime"`
	Hidden  bool   `json:"hidden"`
	Ignored bool   `json:"ignored"`
	Link    bool   `json:"link"`
	Unsafe  string `json:"unsafe,omitempty"`
}

// Listing is one folder.
type Listing struct {
	Path      string  `json:"path"`
	Entries   []Entry `json:"entries"`
	Truncated bool    `json:"truncated"`
	Total     int     `json:"total"`
}

func limitReader(r io.Reader) io.Reader { return io.LimitReader(r, 1<<20) }

// List lists one folder of a project.
func (s *Service) List(projectID, rel string) (*Listing, error) {
	root, err := s.root(projectID)
	if err != nil {
		return nil, err
	}
	real, err := s.follow(root, rel)
	if err != nil {
		return nil, err
	}
	des, err := os.ReadDir(real)
	if err != nil {
		if info, serr := os.Stat(real); serr == nil && !info.IsDir() {
			return nil, apiErr("NOT_A_FOLDER", "That is a file, not a folder.")
		}
		return nil, pathErr(err, rel)
	}
	dir, _ := clean(rel)
	ign := ignoreFor(root, dir)
	parentIgnored := dir != "" && ign.anyIgnored(dir, true)
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		if de.Name() == ".git" {
			continue
		}
		if strings.HasPrefix(de.Name(), tempPrefix) {
			sweepTemp(filepath.Join(real, de.Name()))
			continue
		}
		e := Entry{Name: de.Name(), Hidden: strings.HasPrefix(de.Name(), ".")}
		switch t := de.Type(); {
		case t&os.ModeSymlink != 0:
			e.Link = true
			e.Kind, e.Unsafe = s.linkKind(root, path.Join(dir, de.Name()), filepath.Join(real, de.Name()))
		case t.IsDir():
			e.Kind = "dir"
		case t.IsRegular():
			e.Kind = "file"
		default:
			e.Kind = "other"
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.Kind == "dir") != (b.Kind == "dir") {
			return a.Kind == "dir"
		}
		if a.Kind == "dir" && a.Hidden != b.Hidden {
			return !a.Hidden
		}
		if la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name); la != lb {
			return la < lb
		}
		return a.Name < b.Name
	})
	res := &Listing{Path: dir, Total: len(out)}
	if len(out) > MaxEntries {
		out, res.Truncated = out[:MaxEntries], true
	}
	for i := range out {
		e := &out[i]
		e.Ignored = parentIgnored || ign.ignored(path.Join(dir, e.Name), e.Kind == "dir")
		if e.Unsafe != "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(real, e.Name)); err == nil {
			e.MTime = info.ModTime().UnixMilli()
			if e.Kind != "dir" {
				e.Size = info.Size()
			}
		}
	}
	res.Entries = out
	return res, nil
}

// linkKind is the kind of a link's target, or "link" and why it cannot be used.
func (s *Service) linkKind(root, rel, full string) (kind, unsafe string) {
	info, err := os.Stat(full)
	if err != nil {
		return "link", "broken"
	}
	if _, err := s.follow(root, rel); err != nil {
		return "link", "outside"
	}
	switch {
	case info.IsDir():
		return "dir", ""
	case info.Mode().IsRegular():
		return "file", ""
	}
	return "other", ""
}
