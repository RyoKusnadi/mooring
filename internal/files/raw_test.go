package files

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const tinyPNG = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

func (r *rig) raw(project, path, site string) *httptest.ResponseRecorder {
	r.t.Helper()
	return r.rawWith("GET", project, path, site)
}

func (r *rig) rawWith(method, project, path, site string) *httptest.ResponseRecorder {
	r.t.Helper()
	mux := http.NewServeMux()
	r.s.Mount(mux, func(h http.Handler) http.Handler { return h })
	req := httptest.NewRequest(method, "/api/files/raw?project="+url.QueryEscape(project)+"&path="+url.QueryEscape(path), nil)
	if site != "" {
		req.Header.Set("Sec-Fetch-Site", site)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestRawServesAnImageWithSafeHeaders(t *testing.T) {
	r := newRig(t, nil)
	r.write("a.png", tinyPNG)
	w := r.raw("p1", "a.png", "same-origin")
	if w.Code != 200 || w.Body.String() != tinyPNG {
		t.Fatalf("got %d %q", w.Code, w.Body.String())
	}
	h := w.Header()
	if h.Get("Content-Type") != "image/png" || h.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(h.Get("Content-Security-Policy"), "sandbox") || h.Get("ETag") == "" || !strings.Contains(h.Get("Cache-Control"), "private") {
		t.Fatalf("headers: %v", h)
	}
	again := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/files/raw?project=p1&path=a.png", nil)
	req.Header.Set("If-None-Match", h.Get("ETag"))
	mux := http.NewServeMux()
	r.s.Mount(mux, func(h http.Handler) http.Handler { return h })
	mux.ServeHTTP(again, req)
	if again.Code != http.StatusNotModified {
		t.Fatalf("revalidation: %d", again.Code)
	}
}

func TestRawSvgGetsTheSandboxPolicy(t *testing.T) {
	r := newRig(t, nil)
	r.write("v.svg", `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	w := r.raw("p1", "v.svg", "")
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("got %d %v", w.Code, w.Header())
	}
	if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "sandbox") {
		t.Fatalf("csp %q", csp)
	}
}

func TestRawRefusesWhatIsNotAnAllowedImage(t *testing.T) {
	r := newRig(t, nil)
	r.write("notes.txt", "hello")
	r.write("fake.png", "<script>alert(1)</script>")
	r.write("page.png.html", "<svg></svg>")
	r.write("masked.svg", "just text")
	for _, p := range []string{"notes.txt", "fake.png", "page.png.html", "masked.svg"} {
		if w := r.raw("p1", p, ""); w.Code != 415 {
			t.Fatalf("%s: got %d, want 415", p, w.Code)
		}
	}
}

func TestRawRefusesPathsOutsideTheProject(t *testing.T) {
	r := newRig(t, nil)
	r.write(r.outside+"/out.png", tinyPNG)
	r.link(r.outside+"/out.png", "link.png")
	for _, p := range []string{"../outside/out.png", "link.png"} {
		if w := r.raw("p1", p, ""); w.Code != 403 {
			t.Fatalf("%s: got %d, want 403", p, w.Code)
		}
	}
	if w := r.raw("nope", "a.png", ""); w.Code != 404 {
		t.Fatalf("unknown project: %d", w.Code)
	}
}

func TestRawRefusesCrossSiteRequests(t *testing.T) {
	r := newRig(t, nil)
	r.write("a.png", tinyPNG)
	for _, site := range []string{"cross-site", "same-site"} {
		if w := r.raw("p1", "a.png", site); w.Code != 403 {
			t.Fatalf("%s: got %d, want 403", site, w.Code)
		}
	}
}

func TestRawRefusesAnImageOverTheLimit(t *testing.T) {
	r := newRig(t, nil)
	f, err := os.Create(filepath.Join(r.root, "big.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxImage + 1); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if w := r.raw("p1", "big.png", ""); w.Code != 413 || !strings.Contains(w.Body.String(), "50 MB") {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
}

func TestRawHeadSendsHeadersAndNoBody(t *testing.T) {
	r := newRig(t, nil)
	r.write("a.png", tinyPNG)
	w := r.rawWith("HEAD", "p1", "a.png", "")
	if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("Content-Length") == "" {
		t.Fatalf("got %d, %d body bytes, %v", w.Code, w.Body.Len(), w.Header())
	}
}

func TestRawRefusesASymlinkedFolderLeavingTheProject(t *testing.T) {
	r := newRig(t, nil)
	r.write(r.outside+"/deep.png", tinyPNG)
	r.link(r.outside, "pics")
	if w := r.raw("p1", "pics/deep.png", ""); w.Code != 403 {
		t.Fatalf("got %d, want 403", w.Code)
	}
}

func TestRawServesAPdfCheckedByItsHeader(t *testing.T) {
	r := newRig(t, nil)
	r.write("doc.pdf", "%PDF-1.4\n%fake body\n")
	r.write("fake.pdf", "<html><script>alert(1)</script></html>")
	r.write("pic.pdf", tinyPNG)
	w := r.raw("p1", "doc.pdf", "same-origin")
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("got %d %v", w.Code, w.Header())
	}
	if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") {
		t.Fatalf("csp %q", csp)
	}
	for _, p := range []string{"fake.pdf", "pic.pdf"} {
		if w := r.raw("p1", p, ""); w.Code != 415 {
			t.Fatalf("%s: got %d, want 415", p, w.Code)
		}
	}
	// A PNG named .png that happens to hold the PDF marker stays a PNG.
	r.write("odd.png", tinyPNG+"%PDF-")
	if w := r.raw("p1", "odd.png", ""); w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("odd.png: %d %v", w.Code, w.Header())
	}
}
