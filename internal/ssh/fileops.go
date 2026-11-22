package ssh

import (
	"errors"
	"io/fs"
	"os"
	"path"
)

// Rename renames or moves an item inside the remote folder. The target must not exist.
func (m *Manager) Rename(connID, from, to string) (map[string]string, error) {
	r, err := m.fsys(connID)
	if err != nil {
		return nil, err
	}
	src, err := r.entry(from)
	if err != nil {
		return nil, err
	}
	dst, err := r.entry(to)
	if err != nil {
		return nil, err
	}
	if _, err := r.cl.Lstat(src); err != nil {
		return nil, mapErr(err)
	}
	if _, err := r.cl.Lstat(dst); err == nil {
		return nil, apiErr("ALREADY_EXISTS", "Something with that name already exists.")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, mapErr(err)
	}
	if err := r.cl.Rename(src, dst); err != nil {
		return nil, mapErr(err)
	}
	f, _ := clean(from)
	t, _ := clean(to)
	return map[string]string{"from": f, "to": t}, nil
}

// Delete removes a file, a link, or a folder (empty, unless recursive). There is no trash.
func (m *Manager) Delete(connID, rel string, recursive bool) error {
	r, err := m.fsys(connID)
	if err != nil {
		return err
	}
	full, err := r.entry(rel)
	if err != nil {
		return err
	}
	fi, err := r.cl.Lstat(full)
	if err != nil {
		return mapErr(err)
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir():
		err = r.cl.Remove(full)
	case recursive:
		err = r.cl.RemoveAll(full)
	default:
		if kids, rerr := r.cl.ReadDir(full); rerr == nil && len(kids) > 0 {
			return apiErr("BAD_REQUEST", "That folder is not empty.")
		}
		err = r.cl.RemoveDirectory(full)
	}
	return mapErr(err)
}

// Mkdir makes a folder; its parent must exist.
func (m *Manager) Mkdir(connID, rel string) error {
	r, err := m.fsys(connID)
	if err != nil {
		return err
	}
	full, err := r.entry(rel)
	if err != nil {
		return err
	}
	if _, err := r.cl.Lstat(full); err == nil {
		return apiErr("ALREADY_EXISTS", "Something with that name already exists.")
	}
	return mapErr(r.cl.Mkdir(full))
}

// Open opens a file for download. It returns the file's name, size and a reader.
func (m *Manager) Open(connID, rel string) (name string, size int64, rd interface {
	Read([]byte) (int, error)
	Close() error
}, err error) {
	r, err := m.fsys(connID)
	if err != nil {
		return "", 0, nil, err
	}
	real, err := r.follow(rel)
	if err != nil {
		return "", 0, nil, err
	}
	fi, err := r.cl.Stat(real)
	if err != nil {
		return "", 0, nil, mapErr(err)
	}
	if fi.IsDir() {
		return "", 0, nil, apiErr("IS_A_FOLDER", "That is a folder, not a file.")
	}
	if !fi.Mode().IsRegular() {
		return "", 0, nil, apiErr("BAD_REQUEST", "That is a pipe, socket or device, not a file.")
	}
	f, err := r.cl.Open(real)
	if err != nil {
		return "", 0, nil, mapErr(err)
	}
	return path.Base(real), fi.Size(), f, nil
}
