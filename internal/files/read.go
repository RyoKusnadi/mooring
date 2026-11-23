package files

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf8"
)

// Chunk sizes for reads.
const (
	MaxChunk = 1 << 20
	minChunk = 4 << 10
	sniffLen = 8000
)

// ReadResult is one chunk of a file.
type ReadResult struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Version  string `json:"version"`
	Binary   bool   `json:"binary"`
	TooLarge bool   `json:"tooLarge"`
	Writable bool   `json:"writable"`
	Offset   int64  `json:"offset"`
	Next     int64  `json:"next"`
	Done     bool   `json:"done"`
	Content  string `json:"content"`
}

// Version is the first 16 bytes of the SHA-256 of data, in hex.
func Version(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16])
}

// IsText reports whether data counts as text: no NUL in its first bytes and valid UTF-8.
func IsText(data []byte) bool {
	head := data
	if len(head) > sniffLen {
		head = head[:sniffLen]
	}
	return bytes.IndexByte(head, 0) < 0 && utf8.Valid(data)
}

// ReadWhole reads a file up to max bytes. It fails when the file is larger.
func ReadWhole(real string, max int64) ([]byte, error) {
	// O_NONBLOCK so opening a FIFO does not wait for a writer; it is refused below.
	f, err := os.OpenFile(real, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if info, err := f.Stat(); err != nil {
		return nil, err
	} else if !info.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
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

// ErrNotRegular is returned for a FIFO, socket or device, which could block a read.
var ErrNotRegular = errors.New("not a regular file")

// WriteAtomic replaces the file at real with data: a temporary file next to it
// is renamed into place, and the mode is kept (0644 for a new file).
func WriteAtomic(real string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(real); err == nil {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(real)
	var tmp *os.File
	var err error
	for range 5 {
		var b [6]byte
		_, _ = rand.Read(b[:])
		name := filepath.Join(dir, ".mooring-"+hex.EncodeToString(b[:])+".tmp")
		tmp, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	if tmp == nil {
		return fmt.Errorf("create temporary file in %s: %w", dir, err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), real); err != nil {
		return err
	}
	ok = true
	return nil
}

// Read returns one chunk of a project file. version, when set, must still be the file's version.
func (s *Service) Read(projectID, rel string, offset int64, limit int, version string) (*ReadResult, error) {
	root, err := s.root(projectID)
	if err != nil {
		return nil, err
	}
	real, err := s.follow(root, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(real)
	if err != nil {
		return nil, pathErr(err, rel)
	}
	if info.IsDir() {
		return nil, apiErr("IS_A_FOLDER", "That is a folder, not a file.")
	}
	if !info.Mode().IsRegular() {
		return nil, pathErr(ErrNotRegular)
	}
	c, _ := clean(rel)
	res := &ReadResult{Path: c, Size: info.Size(), Writable: writable(real, info)}
	if info.Size() > MaxFile {
		res.TooLarge = true
		return res, nil
	}
	data, err := ReadWhole(real, MaxFile)
	if errors.Is(err, errTooLarge) {
		res.TooLarge = true
		return res, nil
	}
	if err != nil {
		return nil, pathErr(err, rel)
	}
	return Chunk(res, data, offset, limit, version)
}

// Chunk fills res from the whole content of a file: its size and version, binary
// detection, and the chunk of text at offset. version, when set, must be the content's.
func Chunk(res *ReadResult, data []byte, offset int64, limit int, version string) (*ReadResult, error) {
	res.Size, res.Version = int64(len(data)), Version(data)
	if version != "" && version != res.Version {
		return nil, apiErr("FILE_CHANGED", "The file changed while it was being read.")
	}
	if !IsText(data) {
		res.Binary = true
		return res, nil
	}
	if limit <= 0 || limit > MaxChunk {
		limit = MaxChunk
	}
	limit = max(limit, minChunk)
	size := int64(len(data))
	if offset < 0 || offset > size || (offset < size && !utf8.RuneStart(data[offset])) {
		return nil, apiErr("BAD_REQUEST", "The offset is not the start of a character.")
	}
	end := min(offset+int64(limit), size)
	for end < size && !utf8.RuneStart(data[end]) {
		end--
	}
	res.Offset, res.Next, res.Done = offset, end, end == size
	res.Content = string(data[offset:end])
	return res, nil
}

func writable(real string, info os.FileInfo) bool {
	if info.Mode().Perm()&0o222 == 0 {
		return false
	}
	f, err := os.OpenFile(real, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// WriteResult is the outcome of a save.
type WriteResult struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Size    int64  `json:"size"`
}

// WriteReal saves data at a resolved path under the same lock as Write, without a
// version check: an agent's write has no base version. Open editors learn of it
// from the watcher.
func (s *Service) WriteReal(real string, data []byte) error {
	defer s.lock(real)()
	return WriteAtomic(real, data)
}

// Write saves data as the new content of a file whose current version is base
// ("" for a file that must not exist yet).
func (s *Service) Write(projectID, rel, base string, data []byte) (*WriteResult, error) {
	if len(data) > MaxFile {
		return nil, apiErr("FILE_TOO_LARGE", "The file is over 10 MB.")
	}
	if !utf8.Valid(data) {
		return nil, apiErr("BAD_REQUEST", "The text is not valid UTF-8.")
	}
	root, err := s.root(projectID)
	if err != nil {
		return nil, err
	}
	real, err := s.follow(root, rel)
	if err != nil {
		return nil, err
	}
	defer s.lock(real)()
	cur, err := currentVersion(real)
	if err != nil {
		return nil, err
	}
	if cur != base {
		if cur == "" {
			return nil, apiErr("FILE_CHANGED", "The file was deleted since you opened it.")
		}
		return nil, apiErr("FILE_CHANGED", "The file changed on disk since you opened it.")
	}
	if err := WriteAtomic(real, data); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apiErr("NOT_FOUND", "The folder for that file does not exist.")
		}
		return nil, pathErr(err)
	}
	c, _ := clean(rel)
	return &WriteResult{Path: c, Version: Version(data), Size: int64(len(data))}, nil
}

// currentVersion is "" when the file does not exist.
func currentVersion(real string) (string, error) {
	info, err := os.Stat(real)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", pathErr(err)
	}
	if info.IsDir() {
		return "", apiErr("IS_A_FOLDER", "That is a folder, not a file.")
	}
	data, err := ReadWhole(real, MaxFile)
	if errors.Is(err, errTooLarge) {
		return "", apiErr("FILE_TOO_LARGE", "The file on disk is over 10 MB.")
	}
	if err != nil {
		return "", pathErr(err)
	}
	return Version(data), nil
}
