package files

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// xdgTrash is the freedesktop.org home trash: $XDG_DATA_HOME/Trash/files, or
// ~/.local/share/Trash/files when XDG_DATA_HOME is unset or not absolute.
func xdgTrash(home string) string {
	if d := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(d) {
		return filepath.Join(d, "Trash", "files")
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share", "Trash", "files")
}

// freedesktop reports whether trash is a freedesktop.org trash (…/Trash/files).
// Such a trash keeps a .trashinfo per item in …/Trash/info, so a file manager can restore it.
func freedesktop(trash string) bool {
	return filepath.Base(trash) == "files" && filepath.Base(filepath.Dir(trash)) == "Trash"
}

// writeTrashInfo records where the item that becomes dst came from. It fails when
// the record exists, so two items never share one. The caller removes it if the move fails.
func writeTrashInfo(dst, from string, now time.Time) (string, error) {
	dir := filepath.Join(filepath.Dir(filepath.Dir(dst)), "info")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("trash info folder: %w", err)
	}
	p := filepath.Join(dir, filepath.Base(dst)+".trashinfo")
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("trash info: %w", err)
	}
	body := "[Trash Info]\nPath=" + (&url.URL{Path: from}).EscapedPath() + "\nDeletionDate=" + now.Format("2006-01-02T15:04:05") + "\n"
	_, err = f.WriteString(body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(p)
		return "", fmt.Errorf("trash info: %w", err)
	}
	return p, nil
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}
