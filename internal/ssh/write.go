package ssh

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path"

	"github.com/ryokusnadi/mooring/internal/files"
)

// versionOf is the version of an existing remote file, "" when it is too large to be one the browser read.
func (r *remoteFS) versionOf(real string, fi os.FileInfo) (string, error) {
	if fi.Size() > files.MaxFile {
		return "", nil
	}
	data, err := r.readAll(real, files.MaxFile)
	if errors.Is(err, errTooLarge) {
		return "", nil
	}
	if err != nil {
		return "", mapErr(err)
	}
	return files.Version(data), nil
}

// Write saves data as the new content of a file whose current version is base
// ("" for a file that must not exist yet). The file is written next to its
// place and renamed over it.
func (m *Manager) Write(connID, rel, base string, data []byte) (*files.WriteResult, error) {
	if len(data) > files.MaxFile {
		return nil, apiErr("FILE_TOO_LARGE", "The file is over 10 MB.")
	}
	r, err := m.fsys(connID)
	if err != nil {
		return nil, err
	}
	c, err := clean(rel)
	if err != nil {
		return nil, err
	}
	real, err := r.follow(rel)
	if err != nil {
		return nil, err
	}
	if c == "" {
		return nil, apiErr("IS_A_FOLDER", "That is a folder, not a file.")
	}
	defer m.lockPath(connID + "\x00" + real)()
	fi, err := r.cl.Stat(real)
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, mapErr(err)
	}
	if exists && fi.IsDir() {
		return nil, apiErr("IS_A_FOLDER", "That is a folder, not a file.")
	}
	if err := r.checkBase(real, fi, exists, base); err != nil {
		return nil, err
	}
	mode := os.FileMode(0o644)
	if exists {
		mode = fi.Mode().Perm()
	}
	if err := r.replace(real, data, mode, exists, base); err != nil {
		return nil, err
	}
	return &files.WriteResult{Path: c, Version: files.Version(data), Size: int64(len(data))}, nil
}

func (r *remoteFS) checkBase(real string, fi os.FileInfo, exists bool, base string) error {
	if base == "" {
		if exists {
			return apiErr("ALREADY_EXISTS", "Something with that name already exists.")
		}
		return nil
	}
	if !exists {
		return apiErr("FILE_CHANGED", "The file was deleted since you opened it.")
	}
	cur, err := r.versionOf(real, fi)
	if err != nil {
		return err
	}
	if cur != base {
		return apiErr("FILE_CHANGED", "The file changed on the server since you opened it.")
	}
	return nil
}

func (r *remoteFS) replace(real string, data []byte, mode os.FileMode, exists bool, base string) error {
	cl := r.cl
	_, posix := cl.HasExtension("posix-rename@openssh.com")
	if exists && !posix {
		// No atomic replace here: check again just before writing in place.
		fi, err := cl.Stat(real)
		if err != nil {
			return mapErr(err)
		}
		if err := r.checkBase(real, fi, true, base); err != nil {
			return err
		}
		return r.writeFile(real, data, os.O_WRONLY|os.O_TRUNC, mode)
	}
	tmp := ""
	for range 5 {
		var b [6]byte
		_, _ = rand.Read(b[:])
		name := path.Join(path.Dir(real), ".mooring-"+hex.EncodeToString(b[:])+".tmp")
		if err := r.writeFile(name, data, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode); err == nil {
			tmp = name
			break
		} else if !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	if tmp == "" {
		return apiErr("INTERNAL", "A temporary file could not be created next to the file.")
	}
	fail := func(err error) error {
		_ = cl.Remove(tmp)
		return err
	}
	if exists {
		if err := cl.PosixRename(tmp, real); err != nil {
			return fail(mapErr(err))
		}
		return nil
	}
	// A new file: a plain rename refuses to replace one that appeared meanwhile.
	if err := cl.Rename(tmp, real); err != nil {
		if _, serr := cl.Lstat(real); serr == nil {
			return fail(apiErr("ALREADY_EXISTS", "Something with that name already exists."))
		}
		return fail(mapErr(err))
	}
	return nil
}

// writeFile writes data to name. A file this call creates (O_EXCL) gets mode
// before the first byte and is removed if the write fails; an existing file
// that is truncated is never removed.
func (r *remoteFS) writeFile(name string, data []byte, flags int, mode os.FileMode) error {
	created := flags&os.O_EXCL != 0
	f, err := r.cl.OpenFile(name, flags)
	if err != nil {
		return mapErr(err)
	}
	drop := func() {
		if created {
			_ = r.cl.Remove(name)
		}
	}
	if created {
		if err := f.Chmod(mode); err != nil {
			_ = f.Close()
			drop()
			return mapErr(err)
		}
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		drop()
		return mapErr(err)
	}
	if err := f.Close(); err != nil {
		drop()
		return mapErr(err)
	}
	return nil
}

// lockPath serialises writes to one remote path.
func (m *Manager) lockPath(key string) func() {
	h := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		h = (h ^ uint32(key[i])) * 16777619
	}
	mu := &m.writes[h%uint32(len(m.writes))]
	mu.Lock()
	return mu.Unlock
}
