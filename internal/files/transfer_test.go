package files

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/store"
)

func transferRig(t *testing.T) *rig {
	t.Helper()
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	led, err := ledger.New(context.Background(), st.DB, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return newRig(t, func(c *Config) { c.Ledger = led })
}

func (r *rig) serve(method, route, path, id, site string, body io.Reader, extra string) *httptest.ResponseRecorder {
	r.t.Helper()
	mux := http.NewServeMux()
	r.s.Mount(mux, func(h http.Handler) http.Handler { return h })
	req := httptest.NewRequest(method, "/api/files/"+route+"?project=p1&path="+url.QueryEscape(path)+extra, body)
	if id != "" {
		req.Header.Set("Mooring-Command-Id", id)
	}
	if site != "" {
		req.Header.Set("Sec-Fetch-Site", site)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func (r *rig) upload(path, id, content, extra string) *httptest.ResponseRecorder {
	return r.serve("PUT", "upload", path, id, "same-origin", strings.NewReader(content), extra)
}

func (r *rig) leftovers() []string {
	var out []string
	_ = filepath.WalkDir(r.root, func(p string, d os.DirEntry, _ error) error {
		if strings.HasPrefix(d.Name(), ".mooring-upload-") {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestDownloadSendsAnAttachment(t *testing.T) {
	r := transferRig(t)
	r.write("d/my file.bin", "\x00\x01bytes")
	w := r.serve("GET", "download", "d/my file.bin", "", "same-origin", nil, "")
	h := w.Header()
	if w.Code != 200 || w.Body.String() != "\x00\x01bytes" {
		t.Fatalf("got %d %q", w.Code, w.Body.String())
	}
	if !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") || !strings.Contains(h.Get("Content-Disposition"), "my file.bin") ||
		h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("headers: %v", h)
	}
}

func TestDownloadRefusals(t *testing.T) {
	r := transferRig(t)
	r.write("a.txt", "x")
	r.write("sub/b.txt", "x")
	r.link(r.outside+"/secret.txt", "leak")
	r.link("sub/b.txt", "inner")
	for name, tc := range map[string]struct {
		path, site string
		want       int
	}{
		"cross site":    {"a.txt", "cross-site", 403},
		"outside":       {"../outside/secret.txt", "", 403},
		"link out":      {"leak", "", 403},
		"folder":        {"sub", "", 400},
		"missing":       {"nope", "", 404},
		"absolute path": {"/etc/passwd", "", 400},
		"link inside":   {"inner", "", 200},
	} {
		if w := r.serve("GET", "download", tc.path, "", tc.site, nil, ""); w.Code != tc.want {
			t.Errorf("%s: got %d %s", name, w.Code, w.Body.String())
		}
	}
}

func TestUploadWritesOnceAndRefusesOverwrite(t *testing.T) {
	r := transferRig(t)
	if w := r.upload("sub-missing/a.txt", "id-0", "x", "&replace=0"); w.Code != 404 {
		t.Fatalf("missing folder: %d", w.Code)
	}
	r.write("sub/keep", "")
	if w := r.upload("sub/a.txt", "id-1", "hello", "&replace=0"); w.Code != 200 {
		t.Fatalf("first: %d %s", w.Code, w.Body.String())
	}
	if w := r.upload("sub/a.txt", "id-1", "hello", "&replace=0"); w.Code != 200 {
		t.Fatalf("same id again should replay: %d %s", w.Code, w.Body.String())
	}
	if b, _ := os.ReadFile(filepath.Join(r.root, "sub/a.txt")); string(b) != "hello" {
		t.Fatalf("content %q", b)
	}
	w := r.upload("sub/a.txt", "id-2", "other", "&replace=0")
	if w.Code != 409 || !strings.Contains(w.Body.String(), "ALREADY_EXISTS") {
		t.Fatalf("overwrite: %d %s", w.Code, w.Body.String())
	}
	if b, _ := os.ReadFile(filepath.Join(r.root, "sub/a.txt")); string(b) != "hello" {
		t.Fatalf("file changed: %q", b)
	}
	if l := r.leftovers(); len(l) != 0 {
		t.Fatalf("temp files left: %v", l)
	}
}

func TestUploadReplaceMovesTheOldFileToTheTrash(t *testing.T) {
	r := transferRig(t)
	r.write("a.txt", "old")
	if w := r.upload("a.txt", "id-1", "new", "&replace=1"); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if b, _ := os.ReadFile(filepath.Join(r.root, "a.txt")); string(b) != "new" {
		t.Fatalf("content %q", b)
	}
	trashed, _ := filepath.Glob(filepath.Join(r.s.trash, "a.txt*"))
	if len(trashed) != 1 {
		t.Fatalf("trash: %v", trashed)
	}
	if b, _ := os.ReadFile(trashed[0]); string(b) != "old" {
		t.Fatalf("trashed content %q", b)
	}
}

func TestUploadRefusals(t *testing.T) {
	r := transferRig(t)
	r.write("sub/x", "")
	r.link(r.outside, "out")
	for name, tc := range map[string]struct {
		path, site string
		want       int
	}{
		"outside":        {"../outside/new.txt", "", 403},
		"through a link": {"out/new.txt", "", 403},
		"cross site":     {"new.txt", "cross-site", 403},
		"onto a folder":  {"sub", "", 400},
	} {
		if w := r.serve("PUT", "upload", tc.path, "id-"+strings.ReplaceAll(name, " ", "-"), tc.site, strings.NewReader("x"), "&replace=1"); w.Code != tc.want {
			t.Errorf("%s: got %d %s", name, w.Code, w.Body.String())
		}
	}
	if _, err := os.Stat(filepath.Join(r.outside, "new.txt")); err == nil {
		t.Fatal("wrote outside the project")
	}
	if l := r.leftovers(); len(l) != 0 {
		t.Fatalf("temp files left: %v", l)
	}
}

type zeros struct{ left int64 }

func (z *zeros) Read(p []byte) (int, error) {
	if z.left <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > z.left {
		n = z.left
	}
	z.left -= n
	return int(n), nil
}

func TestUploadOverTheCapIsRefusedWhileStreaming(t *testing.T) {
	r := transferRig(t)
	w := r.serve("PUT", "upload", "big.bin", "id-big", "same-origin", &zeros{left: MaxUpload + 1}, "&replace=0")
	if w.Code != 413 {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(r.root, "big.bin")); err == nil {
		t.Fatal("file exists")
	}
	if l := r.leftovers(); len(l) != 0 {
		t.Fatalf("temp files left: %v", l)
	}
}

type brokenBody struct{ sent bool }

func (b *brokenBody) Read(p []byte) (int, error) {
	if !b.sent {
		b.sent = true
		return copy(p, "partial"), nil
	}
	return 0, io.ErrUnexpectedEOF
}

func TestAnAbortedUploadLeavesNoFile(t *testing.T) {
	r := transferRig(t)
	w := r.serve("PUT", "upload", "cut.txt", "id-cut", "same-origin", &brokenBody{}, "&replace=0")
	if w.Code == 200 {
		t.Fatalf("an aborted upload succeeded: %s", w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(r.root, "cut.txt")); err == nil {
		t.Fatal("partial file left")
	}
	if l := r.leftovers(); len(l) != 0 {
		t.Fatalf("temp files left: %v", l)
	}
	if w := r.upload("cut.txt", "id-cut2", "whole", "&replace=0"); w.Code != 200 {
		t.Fatalf("retry: %d", w.Code)
	}
}

func TestUploadKeepsTheBytesIntact(t *testing.T) {
	r := transferRig(t)
	data := bytes.Repeat([]byte{0, 255, 10, 13, 7}, 100000)
	if w := r.serve("PUT", "upload", "bin.dat", "id-bin", "same-origin", bytes.NewReader(data), "&replace=0"); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	back := r.serve("GET", "download", "bin.dat", "", "same-origin", nil, "")
	if !bytes.Equal(back.Body.Bytes(), data) {
		t.Fatal("download differs from upload")
	}
}
