package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// MaxUpload is the largest file the upload route takes.
const MaxUpload = 100 << 20

// tempPrefix starts the names of the temporary files an upload makes. The tree hides them.
const tempPrefix = ".mooring-upload-"

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// pendingAfter is how long an upload may run before the browser is told to ask again. Tests shorten it.
var pendingAfter = ledger.PendingAfter

// beforePlace runs inside the command, before the file is placed. Tests use it to hold an upload.
var beforePlace func()

func refuseCrossSite(w http.ResponseWriter, r *http.Request) bool {
	if !crossSite(r) {
		return false
	}
	reply(w, http.StatusForbidden, map[string]any{"ok": false, "error": apiErr("OUTSIDE_PROJECT", "This file can only be used from Mooring.")})
	return true
}

// httpDownload sends one regular file as an attachment. The path is resolved through links to a
// place inside the project, so a link that leaves it is refused.
func (s *Service) httpDownload(w http.ResponseWriter, r *http.Request) {
	if refuseCrossSite(w, r) {
		return
	}
	q := r.URL.Query()
	root, err := s.root(q.Get("project"))
	if err != nil {
		s.fail(w, err)
		return
	}
	real, err := s.follow(root, q.Get("path"))
	if err != nil {
		s.fail(w, err)
		return
	}
	f, err := os.OpenFile(real, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		s.fail(w, pathErr(err, q.Get("path")))
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		s.fail(w, apiErr("NOT_A_FILE", "Only files can be downloaded, not folders."))
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(q.Get("path"))}))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h.Set("Cache-Control", "private, no-store")
	h.Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	if r.Method == http.MethodHead {
		return
	}
	if n, err := io.CopyN(w, f, info.Size()); err != nil {
		s.log.Warn("files: file changed while it was sent", "path", q.Get("path"), "sent", n, "size", info.Size(), "err", err)
	}
}

// httpUpload stores the request body as the file at path. The body goes to a temporary file in
// the target folder first, counted as it arrives, and is renamed into place only when complete.
func (s *Service) httpUpload(w http.ResponseWriter, r *http.Request) {
	if refuseCrossSite(w, r) {
		return
	}
	q := r.URL.Query()
	replace := q.Get("replace") == "1"
	id := r.Header.Get("Mooring-Command-Id")
	if !idPattern.MatchString(id) {
		s.fail(w, apiErr("BAD_REQUEST", "The command id must be 1 to 128 letters, digits, dashes or underscores."))
		return
	}
	root, err := s.root(q.Get("project"))
	if err != nil {
		s.fail(w, err)
		return
	}
	dst, err := s.entry(root, q.Get("path"))
	if err != nil {
		s.fail(w, err)
		return
	}
	tmp, sum, size, err := s.spool(w, r, filepath.Dir(dst))
	if err != nil {
		s.fail(w, err)
		return
	}
	args, _ := json.Marshal(map[string]any{
		"projectId": q.Get("project"), "path": q.Get("path"), "replace": replace, "sha": hex.EncodeToString(sum), "size": size,
	})
	var ran atomic.Bool
	call := s.cfg.Ledger.Submit(id, "file.upload", args, false,
		func(context.Context) (any, error) {
			ran.Store(true)
			if beforePlace != nil {
				beforePlace()
			}
			return s.place(tmp, dst, q.Get("path"), replace, size)
		})
	// place owns the temporary file once it runs. When the id was already known it never runs, and the
	// file is removed once that earlier command is over (not now: it may still be starting).
	go func() {
		<-call.Done()
		if !ran.Load() {
			_ = os.Remove(tmp)
		}
	}()
	ctx, cancel := context.WithTimeout(r.Context(), pendingAfter)
	defer cancel()
	out, ok := call.Wait(ctx, pendingAfter, func() {})
	if !ok {
		if r.Context().Err() == nil {
			reply(w, http.StatusAccepted, map[string]any{"ok": false, "error": apiErr("PENDING", "The upload is still running. Send the same id again.")})
		}
		return
	}
	// The command is over: a replayed id leaves no temporary file behind once the reply is sent.
	if !ran.Load() {
		_ = os.Remove(tmp)
	}
	if !out.OK {
		s.fail(w, out.Error)
		return
	}
	reply(w, 200, map[string]any{"ok": true, "value": out.Value})
}

// spool copies the body to a new temporary file in dir and returns its path, hash and size. A body
// over the cap, or one that stops early, leaves no file behind.
func (s *Service) spool(w http.ResponseWriter, r *http.Request, dir string) (string, []byte, int64, error) {
	tmp, err := os.CreateTemp(dir, tempPrefix+"*")
	if err != nil {
		return "", nil, 0, pathErr(err)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), http.MaxBytesReader(w, r.Body, MaxUpload))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return "", nil, 0, apiErr("FILE_TOO_LARGE", "The file is over 100 MB. Copy large files with a terminal.")
		}
		return "", nil, 0, fmt.Errorf("upload body: %w", err)
	}
	return tmp.Name(), h.Sum(nil), n, nil
}

// place moves the finished temporary file to dst. Without replace an existing item is never touched;
// with replace a file there goes to the trash first.
func (s *Service) place(tmp, dst, rel string, replace bool, size int64) (any, error) {
	unlock := s.lock(dst)
	defer unlock()
	fail := func(err error) (any, error) {
		_ = os.Remove(tmp)
		return nil, err
	}
	info, err := os.Lstat(dst)
	var old string
	switch {
	case err == nil && info.IsDir():
		return fail(apiErr("IS_A_FOLDER", "%s is a folder.", filepath.Base(dst)))
	case err == nil && !replace:
		return fail(apiErr("ALREADY_EXISTS", "%s already exists.", filepath.Base(dst)))
	case err == nil:
		// The old file steps aside in its own folder, so it can come back if anything below fails.
		old = filepath.Join(filepath.Dir(dst), tempPrefix+"old-"+strconv.FormatInt(time.Now().UnixNano(), 36))
		if err := os.Rename(dst, old); err != nil {
			return fail(pathErr(err, rel))
		}
	case !errors.Is(err, os.ErrNotExist):
		return fail(pathErr(err, rel))
	}
	restore := func(err error) (any, error) {
		if old != "" {
			_ = os.Remove(dst)
			_ = os.Rename(old, dst)
		}
		return fail(err)
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return restore(pathErr(err))
	}
	// A link fails if something appeared meanwhile; a rename would overwrite it.
	if err := os.Link(tmp, dst); err == nil {
		_ = os.Remove(tmp)
	} else if errors.Is(err, os.ErrExist) {
		return restore(apiErr("ALREADY_EXISTS", "%s already exists.", filepath.Base(dst)))
	} else if err := os.Rename(tmp, dst); err != nil {
		return restore(pathErr(err))
	}
	if old != "" {
		if err := moveToTrashAs(s.trash, s.log, old, filepath.Base(dst)); err != nil {
			// No trash: put the old file back and report it, so nothing is lost.
			_ = os.Remove(dst)
			_ = os.Rename(old, dst)
			return nil, err
		}
	}
	c, _ := clean(rel)
	return map[string]any{"path": c, "size": size}, nil
}

// sweepTemp removes a leftover upload file that is over a day old (a crash or a kill left it).
func sweepTemp(full string) {
	if info, err := os.Lstat(full); err == nil && time.Since(info.ModTime()) > 24*time.Hour {
		_ = os.Remove(full)
	}
}
