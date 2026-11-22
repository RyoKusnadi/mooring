package files

import (
	"context"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// hold makes uploads wait inside the command until release is called.
func hold(t *testing.T) (release func()) {
	t.Helper()
	gate := make(chan struct{})
	beforePlace = func() { <-gate }
	t.Cleanup(func() { beforePlace = nil })
	var once bool
	return func() {
		if !once {
			once = true
			close(gate)
		}
	}
}

func (r *rig) noLeftovers() {
	r.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(r.leftovers()) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.t.Fatalf("temp files left: %v", r.leftovers())
}

func TestUploadStillRunningAnswers202ThenTheRetryGetsTheResult(t *testing.T) {
	r := transferRig(t)
	old := pendingAfter
	pendingAfter = 60 * time.Millisecond
	t.Cleanup(func() { pendingAfter = old })
	release := hold(t)
	defer release()
	if w := r.upload("slow.txt", "id-slow", "data", "&replace=0"); w.Code != 202 || !strings.Contains(w.Body.String(), "PENDING") {
		t.Fatalf("first: %d %s", w.Code, w.Body.String())
	}
	// The same id again while the first still runs must not start a second write.
	if w := r.upload("slow.txt", "id-slow", "data", "&replace=0"); w.Code != 202 {
		t.Fatalf("retry while running: %d %s", w.Code, w.Body.String())
	}
	release()
	var w *httptest.ResponseRecorder
	for i := 0; i < 50; i++ {
		if w = r.upload("slow.txt", "id-slow", "data", "&replace=0"); w.Code == 200 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if w.Code != 200 {
		t.Fatalf("after release: %d %s", w.Code, w.Body.String())
	}
	if b, _ := os.ReadFile(filepath.Join(r.root, "slow.txt")); string(b) != "data" {
		t.Fatalf("content %q", b)
	}
	r.noLeftovers()
}

func TestAClientThatLeavesRightAfterSubmitStillGetsTheFileStored(t *testing.T) {
	r := transferRig(t)
	release := hold(t)
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("PUT", "/api/files/upload?project=p1&path=gone.txt&replace=0", strings.NewReader("kept")).WithContext(ctx)
	req.Header.Set("Mooring-Command-Id", "id-gone")
	mux := http.NewServeMux()
	r.s.Mount(mux, func(h http.Handler) http.Handler { return h })
	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(httptest.NewRecorder(), req)
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done
	release()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(filepath.Join(r.root, "gone.txt")); err == nil && string(b) == "kept" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if b, _ := os.ReadFile(filepath.Join(r.root, "gone.txt")); string(b) != "kept" {
		t.Fatalf("file not stored: %q", b)
	}
	r.noLeftovers()
	if w := r.upload("gone.txt", "id-gone", "kept", "&replace=0"); w.Code != 200 {
		t.Fatalf("replay: %d %s", w.Code, w.Body.String())
	}
}

type mustNotRead struct{ t *testing.T }

func (m mustNotRead) Read([]byte) (int, error) {
	m.t.Error("the body was read")
	return 0, os.ErrClosed
}

func TestABadCommandIdIsRefusedBeforeTheBodyIsRead(t *testing.T) {
	r := transferRig(t)
	for _, id := range []string{"", "has space", strings.Repeat("a", 129)} {
		if w := r.serve("PUT", "upload", "x.txt", id, "same-origin", mustNotRead{t}, "&replace=0"); w.Code != 400 {
			t.Errorf("id %q: %d", id, w.Code)
		}
	}
	r.noLeftovers()
}

func TestReplaceWithoutATrashKeepsTheOldFile(t *testing.T) {
	r := transferRig(t)
	blocked := filepath.Join(r.root, "..", "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.s.trash = filepath.Join(blocked, "trash")
	r.write("a.txt", "old")
	w := r.upload("a.txt", "id-1", "new", "&replace=1")
	if w.Code != 409 || !strings.Contains(w.Body.String(), "TRASH_UNAVAILABLE") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if b, _ := os.ReadFile(filepath.Join(r.root, "a.txt")); string(b) != "old" {
		t.Fatalf("old file changed: %q", b)
	}
	r.noLeftovers()
}

func TestReplaceNeverFollowsALinkAtTheTarget(t *testing.T) {
	r := transferRig(t)
	r.link(r.outside+"/secret.txt", "leak.txt")
	r.link("nowhere", "dangling.txt")
	for _, name := range []string{"leak.txt", "dangling.txt"} {
		if w := r.upload(name, "id-"+name[:3], "mine", "&replace=1"); w.Code != 200 {
			t.Fatalf("%s: %d %s", name, w.Code, w.Body.String())
		}
		if b, _ := os.ReadFile(filepath.Join(r.root, name)); string(b) != "mine" {
			t.Fatalf("%s holds %q", name, b)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(r.outside, "secret.txt")); string(b) != "TOPSECRET needle\n" {
		t.Fatalf("the file behind the link changed: %q", b)
	}
	r.noLeftovers()
}

func TestDownloadNamesTheClickedFileAndKeepsControlCharactersOutOfTheHeader(t *testing.T) {
	r := transferRig(t)
	odd := "a\r\nb\"c é.txt"
	r.write("sub/b.txt", "x")
	r.write(odd, "y")
	r.link("sub/b.txt", "alias.txt")
	for want, path := range map[string]string{odd: odd, "alias.txt": "alias.txt"} {
		w := r.serve("GET", "download", path, "", "same-origin", nil, "")
		if w.Code != 200 {
			t.Fatalf("%q: %d", path, w.Code)
		}
		cd := w.Header().Get("Content-Disposition")
		if strings.ContainsAny(cd, "\r\n") {
			t.Fatalf("raw control character in %q", cd)
		}
		_, params, err := mime.ParseMediaType(cd)
		if err != nil || params["filename"] != want {
			t.Fatalf("%q gave %q (%v)", cd, params["filename"], err)
		}
	}
}

func TestListHidesUploadTempFilesAndSweepsOldOnes(t *testing.T) {
	r := transferRig(t)
	r.write("keep.txt", "x")
	r.write(tempPrefix+"fresh", "x")
	r.write(tempPrefix+"stale", "x")
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(filepath.Join(r.root, tempPrefix+"stale"), old, old); err != nil {
		t.Fatal(err)
	}
	res, err := r.s.List("p1", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Entries {
		if strings.HasPrefix(e.Name, tempPrefix) {
			t.Fatalf("temp file listed: %s", e.Name)
		}
	}
	if _, err := os.Stat(filepath.Join(r.root, tempPrefix+"stale")); err == nil {
		t.Fatal("the stale temp file stayed")
	}
	if _, err := os.Stat(filepath.Join(r.root, tempPrefix+"fresh")); err != nil {
		t.Fatal("a fresh temp file was removed")
	}
}
