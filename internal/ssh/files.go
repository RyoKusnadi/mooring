package ssh

import (
	"errors"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/ryokusnadi/mooring/internal/files"
	"github.com/ryokusnadi/mooring/internal/paths"
)

// Entry is one item of a remote folder (as files.Entry, without ignored).
type Entry struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Size   int64  `json:"size"`
	MTime  int64  `json:"mtime"`
	Hidden bool   `json:"hidden"`
	Link   bool   `json:"link"`
	Unsafe string `json:"unsafe,omitempty"`
}

// Listing is one remote folder.
type Listing struct {
	Path      string  `json:"path"`
	Entries   []Entry `json:"entries"`
	Truncated bool    `json:"truncated"`
	Total     int     `json:"total"`
}

func kindOf(fi os.FileInfo) string {
	switch {
	case fi.IsDir():
		return "dir"
	case fi.Mode().IsRegular():
		return "file"
	}
	return "other"
}

// List lists one folder.
func (m *Manager) List(connID, rel string) (*Listing, error) {
	r, err := m.fsys(connID)
	if err != nil {
		return nil, err
	}
	real, err := r.follow(rel)
	if err != nil {
		return nil, err
	}
	infos, err := r.cl.ReadDir(real)
	if err != nil {
		if fi, serr := r.cl.Stat(real); serr == nil && !fi.IsDir() {
			return nil, apiErr("NOT_A_FOLDER", "That is a file, not a folder.")
		}
		return nil, mapErr(err)
	}
	dir, _ := clean(rel)
	out := make([]Entry, 0, len(infos))
	for _, fi := range infos {
		e := Entry{Name: fi.Name(), Hidden: strings.HasPrefix(fi.Name(), "."), Kind: kindOf(fi), Size: fi.Size(), MTime: fi.ModTime().UnixMilli()}
		if fi.Mode()&os.ModeSymlink != 0 {
			e.Link = true
			e.Kind, e.Unsafe = r.linkKind(path.Join(real, fi.Name()), &e)
		}
		if e.Kind == "dir" {
			e.Size = 0
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.Kind == "dir") != (b.Kind == "dir") {
			return a.Kind == "dir"
		}
		if la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name); la != lb {
			return la < lb
		}
		return a.Name < b.Name
	})
	res := &Listing{Path: dir, Total: len(out)}
	if len(out) > files.MaxEntries {
		out, res.Truncated = out[:files.MaxEntries], true
	}
	res.Entries = out
	return res, nil
}

// linkKind is the kind of a link's target, or "link" and why it cannot be used.
func (r *remoteFS) linkKind(full string, e *Entry) (kind, unsafe string) {
	fi, err := r.cl.Stat(full)
	if err != nil {
		return "link", "broken"
	}
	if _, err := paths.ResolveIn(r.cl, r.root, full); err != nil {
		return "link", "outside"
	}
	e.Size, e.MTime = fi.Size(), fi.ModTime().UnixMilli()
	return kindOf(fi), ""
}

func (r *remoteFS) readAll(real string, max int64) ([]byte, error) {
	f, err := r.cl.Open(real)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, errTooLarge
	}
	return data, nil
}

var errTooLarge = errors.New("file is too large")

// Read returns one chunk of a remote file. version, when set, must still be the file's version.
func (m *Manager) Read(connID, rel string, offset int64, limit int, version string) (*files.ReadResult, error) {
	r, err := m.fsys(connID)
	if err != nil {
		return nil, err
	}
	real, err := r.follow(rel)
	if err != nil {
		return nil, err
	}
	fi, err := r.cl.Stat(real)
	if err != nil {
		return nil, mapErr(err)
	}
	if fi.IsDir() {
		return nil, apiErr("IS_A_FOLDER", "That is a folder, not a file.")
	}
	if !fi.Mode().IsRegular() {
		return nil, apiErr("BAD_REQUEST", "That is a pipe, socket or device, not a file.")
	}
	c, _ := clean(rel)
	res := &files.ReadResult{Path: c, Size: fi.Size(), Writable: fi.Mode().Perm()&0o222 != 0}
	if fi.Size() > files.MaxFile {
		res.TooLarge = true
		return res, nil
	}
	data, err := r.readAll(real, files.MaxFile)
	if errors.Is(err, errTooLarge) {
		res.TooLarge = true
		return res, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return files.Chunk(res, data, offset, limit, version)
}
