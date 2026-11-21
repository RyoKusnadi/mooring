// Package logfile is a log file that rotates by size.
package logfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Writer appends to a file. When a write would take the file past Max bytes,
// the file becomes name.1, name.1 becomes name.2, and so on up to Keep old
// files; the oldest is deleted.
type Writer struct {
	mu   sync.Mutex
	path string
	max  int64
	keep int
	f    *os.File
	size int64
}

// Open opens (creating it and its folder if needed) the log file at path.
func Open(path string, max int64, keep int) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create log folder: %w", err)
	}
	w := &Writer{path: path, max: max, keep: keep}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("stat log file: %w", err)
	}
	w.f, w.size = f, info.Size()
	return nil
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size > 0 && w.size+int64(len(p)) > w.max {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *Writer) rotate() error {
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", w.path, w.keep))
	for i := w.keep - 1; i >= 1; i-- {
		if err := os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rotate log file: %w", err)
		}
	}
	if w.keep > 0 {
		if err := os.Rename(w.path, w.path+".1"); err != nil {
			return fmt.Errorf("rotate log file: %w", err)
		}
	} else if err := os.Remove(w.path); err != nil {
		return fmt.Errorf("rotate log file: %w", err)
	}
	return w.open()
}

// Close closes the file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}
