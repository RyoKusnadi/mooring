package files

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ryokusnadi/mooring/internal/paths"
)

// Limits for duplicating a folder, checked before anything is written. Variables so tests can lower them.
var (
	maxCopyEntries       = 10000
	maxCopyBytes   int64 = 1 << 30
	maxCopyDepth         = 64
	maxSkippedList       = 20
)

// CopyResult is the outcome of file.copy. Skipped lists the first links and special files that were
// not copied; SkippedCount is how many there were in all.
type CopyResult struct {
	Path         string   `json:"path"`
	Skipped      []string `json:"skipped"`
	SkippedCount int      `json:"skippedCount"`
}

var errCopyTooBig = apiErr("FILE_TOO_LARGE", "This folder has more than 10,000 items, is over 1 GB or is nested too deep to duplicate here. Copy it with a terminal.")

// survey walks a folder before it is copied, so a copy that is too big fails before it leaves anything behind.
func survey(src string, res *CopyResult, base string) error {
	var entries int
	var bytes int64
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel != "." && strings.Count(rel, string(filepath.Separator)) >= maxCopyDepth {
			return errCopyTooBig
		}
		if entries++; entries > maxCopyEntries {
			return errCopyTooBig
		}
		switch {
		case d.IsDir():
		case d.Type().IsRegular():
			fi, err := d.Info()
			if err != nil {
				return err
			}
			if bytes += fi.Size(); bytes > maxCopyBytes {
				return errCopyTooBig
			}
		default:
			if res.SkippedCount++; len(res.Skipped) < maxSkippedList {
				res.Skipped = append(res.Skipped, path.Join(base, filepath.ToSlash(rel)))
			}
		}
		return nil
	})
}

// Copy duplicates a file or folder inside the project. The target must not exist. Links and special
// files inside a folder are skipped and reported, so a copy never reaches outside the project.
func (s *Service) Copy(projectID, from, to string) (*CopyResult, error) {
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
	info, err := os.Lstat(src)
	if err != nil {
		return nil, pathErr(err, from)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return nil, apiErr("NOT_A_FILE", "Only files and folders can be duplicated.")
	}
	if info.IsDir() && paths.Inside(src, dst) {
		return nil, apiErr("BAD_REQUEST", "A folder cannot be copied into itself.")
	}
	unlock := s.lock(dst)
	defer unlock()
	if _, err := os.Lstat(dst); err == nil {
		return nil, apiErr("ALREADY_EXISTS", "Something with that name already exists.")
	}
	res := &CopyResult{Skipped: []string{}}
	res.Path, _ = clean(to)
	if !info.IsDir() {
		if err := copyFile(src, dst, info.Mode().Perm()); err != nil {
			return nil, pathErr(err, from)
		}
		return res, nil
	}
	if err := survey(src, res, res.Path); err != nil {
		return nil, pathErr(err, from)
	}
	if err := os.Mkdir(dst, info.Mode().Perm()|0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, apiErr("ALREADY_EXISTS", "Something with that name already exists.")
		}
		return nil, pathErr(err, from)
	}
	// From here dst is ours, so a failure removes it and nothing else.
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		fi, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.Mkdir(target, fi.Mode().Perm()|0o700)
		case d.Type().IsRegular():
			return copyFile(p, target, fi.Mode().Perm())
		}
		return nil
	})
	if err != nil {
		_ = os.RemoveAll(dst)
		return nil, pathErr(err, from)
	}
	return res, nil
}

func copyFile(src, dst string, mode os.FileMode) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode|0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return apiErr("ALREADY_EXISTS", "Something with that name already exists.")
		}
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(dst)
	}
	return err
}
