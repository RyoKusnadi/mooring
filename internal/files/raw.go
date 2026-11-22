package files

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// MaxImage is the largest image the raw route serves.
const MaxImage = 50 << 20

// imageTypes lists the files the raw route may serve, by extension. The content must also look
// like that type, so a renamed text or script file is refused.
var imageTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".avif": "image/avif", ".bmp": "image/bmp", ".ico": "image/x-icon", ".svg": "image/svg+xml",
}

// ImageType is the content type of an image Mooring may show, named name and starting
// with head, or "" when the name or the bytes are not one of the allowed kinds.
func ImageType(name string, head []byte) string {
	want := imageTypes[strings.ToLower(filepath.Ext(name))]
	if want == "" || sniffImage(head) != want {
		return ""
	}
	return want
}

// sniffImage names the image type of the first bytes of a file, or "".
func sniffImage(head []byte) string {
	has := func(off int, s string) bool { return len(head) >= off+len(s) && string(head[off:off+len(s)]) == s }
	switch {
	case has(0, "\x89PNG\r\n\x1a\n"):
		return "image/png"
	case has(0, "\xff\xd8\xff"):
		return "image/jpeg"
	case has(0, "GIF87a"), has(0, "GIF89a"):
		return "image/gif"
	case has(0, "RIFF") && has(8, "WEBP"):
		return "image/webp"
	case has(4, "ftypavif"), has(4, "ftypavis"):
		return "image/avif"
	case has(0, "BM"):
		return "image/bmp"
	case has(0, "\x00\x00\x01\x00"):
		return "image/x-icon"
	}
	text := bytes.TrimPrefix(head, []byte("\xef\xbb\xbf"))
	if bytes.Contains(text, []byte("<svg")) && IsText(text) {
		return "image/svg+xml"
	}
	return ""
}

// crossSite is true for a request another website caused.
func crossSite(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
		return false
	}
	return true
}

// httpRaw serves one image file as it is, for an <img> in the file tab. Anything that is not an
// image of the allowed kinds is refused. The security headers keep an SVG opened on its own from
// running script.
func (s *Service) httpRaw(w http.ResponseWriter, r *http.Request) {
	if crossSite(r) {
		reply(w, http.StatusForbidden, map[string]any{"ok": false, "error": apiErr("OUTSIDE_PROJECT", "This file can only be opened from Mooring.")})
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
	want := imageTypes[strings.ToLower(filepath.Ext(real))]
	if want == "" {
		s.fail(w, apiErr("NOT_AN_IMAGE", "This is not an image Mooring can show."))
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
		s.fail(w, apiErr("NOT_A_FILE", "That is not a regular file."))
		return
	}
	if info.Size() > MaxImage {
		s.fail(w, apiErr("FILE_TOO_LARGE", "The image is over 50 MB, which is more than Mooring shows."))
		return
	}
	head := make([]byte, 4096)
	n, _ := io.ReadFull(f, head)
	if sniffImage(head[:n]) != want {
		s.fail(w, apiErr("NOT_AN_IMAGE", "This file does not hold the image its name says."))
		return
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		s.fail(w, pathErr(err, q.Get("path")))
		return
	}
	etag := fmt.Sprintf(`"%x-%x"`, info.Size(), info.ModTime().UnixNano())
	h := w.Header()
	h.Set("Content-Type", want)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	h.Set("Cache-Control", "private, no-cache")
	h.Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	if r.Method == http.MethodHead {
		return
	}
	// Send what was measured, no more: a file that grows meanwhile is cut at its size, and one that
	// shrinks ends the reply short, which the browser sees as a failed load.
	if n, err := io.CopyN(w, f, info.Size()); err != nil {
		s.log.Warn("files: image changed while it was sent", "path", q.Get("path"), "sent", n, "size", info.Size(), "err", err)
	}
}
