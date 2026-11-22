package files

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/ryokusnadi/mooring/internal/paths"
)

// CreateResult is the outcome of file.create.
type CreateResult struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Version string `json:"version,omitempty"`
}

// Create makes an empty file or a folder; the parent must exist.
func (s *Service) Create(projectID, rel, kind string) (*CreateResult, error) {
	if kind != "file" && kind != "dir" {
		return nil, apiErr("BAD_REQUEST", "The kind must be file or dir.")
	}
	root, err := s.root(projectID)
	if err != nil {
		return nil, err
	}
	full, err := s.entry(root, rel)
	if err != nil {
		return nil, err
	}
	res := &CreateResult{Kind: kind}
	res.Path, _ = clean(rel)
	if kind == "dir" {
		err = os.Mkdir(full, 0o755)
	} else {
		var f *os.File
		if f, err = os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644); err == nil {
			err = f.Close()
		}
		res.Version = Version(nil)
	}
	if errors.Is(err, os.ErrExist) {
		return nil, apiErr("ALREADY_EXISTS", "Something with that name already exists.")
	}
	if err != nil {
		return nil, pathErr(err)
	}
	return res, nil
}

// Rename renames or moves an item inside the project. The target must not exist.
func (s *Service) Rename(projectID, from, to string) (map[string]string, error) {
	root, err := s.root(projectID)
	if err != nil {
		return nil, err
	}
	src, err := s.entry(root, from)
	if err != nil {
		return nil, err
	}
	dst, err := s.entry(root, to)
	if err != nil {
		return nil, err
	}
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return nil, pathErr(err)
	}
	if srcInfo.IsDir() && paths.Inside(src, dst) {
		return nil, apiErr("BAD_REQUEST", "A folder cannot be moved into itself.")
	}
	if dstInfo, err := os.Lstat(dst); err == nil {
		if !os.SameFile(srcInfo, dstInfo) || filepath.Base(src) == filepath.Base(dst) {
			return nil, apiErr("ALREADY_EXISTS", "Something with that name already exists.")
		}
	}
	if err := os.Rename(src, dst); err != nil {
		return nil, pathErr(err)
	}
	c, _ := clean(to)
	return map[string]string{"path": c}, nil
}

// Delete moves an item to the trash, or removes it for good when permanent is set.
func (s *Service) Delete(projectID, rel string, permanent bool) (map[string]bool, error) {
	root, err := s.root(projectID)
	if err != nil {
		return nil, err
	}
	full, err := s.entry(root, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return nil, pathErr(err, rel)
	}
	if !permanent {
		if err := s.toTrash(full); err != nil {
			return nil, err
		}
		return map[string]bool{"trashed": true}, nil
	}
	if info.IsDir() {
		err = os.RemoveAll(full)
	} else {
		err = os.Remove(full)
	}
	if err != nil {
		return nil, pathErr(err)
	}
	return map[string]bool{"trashed": false}, nil
}

func (s *Service) toTrash(full string) error { return MoveToTrash(s.trash, s.log, full) }

// TrashPath is the trash folder for a home folder and a TrashDir override
// ("-" means none); "" means this system has no trash.
func TrashPath(home, override string) string {
	switch {
	case override == "-":
		return ""
	case override != "":
		return override
	case runtime.GOOS == "darwin" && home != "":
		return filepath.Join(home, ".Trash")
	case runtime.GOOS == "linux":
		return xdgTrash(home)
	}
	return ""
}

// MoveToTrash moves full into the trash folder trash. It answers
// TRASH_UNAVAILABLE when there is no trash or the move fails.
func MoveToTrash(trash string, log *slog.Logger, full string) error {
	return moveToTrashAs(trash, log, full, filepath.Base(full))
}

// moveToTrashAs is MoveToTrash with the name the item gets in the trash.
func moveToTrashAs(trash string, log *slog.Logger, full, name string) error {
	unavailable := apiErr("TRASH_UNAVAILABLE", "This server has no trash for that item. Delete it for good only if you are sure.")
	if trash == "" {
		return unavailable
	}
	if err := os.MkdirAll(trash, 0o700); err != nil {
		log.Warn("files: trash folder", "err", err)
		return unavailable
	}
	dst := filepath.Join(trash, name)
	fd := freedesktop(trash)
	if _, err := os.Lstat(dst); err == nil || fd && exists(filepath.Join(filepath.Dir(trash), "info", name+".trashinfo")) {
		dst += " " + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	info := ""
	if fd {
		var err error
		if info, err = writeTrashInfo(dst, full, time.Now()); err != nil {
			log.Warn("files: trash info", "err", err)
			return unavailable
		}
	}
	if err := os.Rename(full, dst); err != nil {
		log.Warn("files: move to trash", "err", err, "cross_device", errors.Is(err, syscall.EXDEV))
		if info != "" {
			_ = os.Remove(info)
		}
		return unavailable
	}
	return nil
}
