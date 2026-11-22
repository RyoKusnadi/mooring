package main

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ryokusnadi/mooring/internal/server"
	"github.com/ryokusnadi/mooring/internal/store"
)

// fakeOpen records the address a browser would open and never starts one.
type fakeOpen struct {
	urls []string
	err  error
}

func (f *fakeOpen) env(now time.Time) openEnv {
	return openEnv{
		open:   func(u string) error { f.urls = append(f.urls, u); return f.err },
		now:    func() time.Time { return now },
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

// liveServer starts a real Mooring handler on a loopback port and returns its address.
func liveServer(t *testing.T, dir, token string) string {
	t.Helper()
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	app, err := server.New(server.Config{ListenAddr: l.Addr().String(), Token: token, Static: fstest.MapFS{"index.html": {Data: []byte("x")}},
		Store: st, Home: t.TempDir(), DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(app)
	ts.Listener = l
	ts.Start()
	t.Cleanup(func() { ts.Close(); app.Close(); _ = st.Close() })
	return strings.TrimPrefix(ts.URL, "http://")
}

func TestOpenUsesTheAddressFileAndTheLinkSignsIn(t *testing.T) {
	dir := t.TempDir()
	token, err := server.LoadOrCreateToken(dir)
	if err != nil {
		t.Fatal(err)
	}
	addr := liveServer(t, dir, token)
	if err := writeAddress(dir, addr); err != nil {
		t.Fatal(err)
	}
	f := &fakeOpen{}
	var out bytes.Buffer
	if err := runOpen([]string{"-data", dir}, f.env(time.Now().Add(time.Second)), &out, &out); err != nil {
		t.Fatal(err)
	}
	if len(f.urls) != 1 || !strings.HasPrefix(f.urls[0], "http://"+addr+"/login?code=v1.") {
		t.Fatalf("opened %v, want a link on %s", f.urls, addr)
	}
	if strings.Contains(out.String(), "code=") {
		t.Fatalf("the link must not be printed when the browser opened: %q", out.String())
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(f.urls[0])
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/" || len(res.Cookies()) != 1 {
		t.Fatalf("link: %d to %q with %d cookies", res.StatusCode, res.Header.Get("Location"), len(res.Cookies()))
	}
}

func TestOpenWithNoServerSaysSo(t *testing.T) {
	dir := t.TempDir()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	if err := writeAddress(dir, addr); err != nil {
		t.Fatal(err)
	}
	f := &fakeOpen{}
	err = runOpen([]string{"-data", dir}, f.env(time.Now()), &bytes.Buffer{}, &bytes.Buffer{})
	want := "Mooring is not running at " + addr + ". Start it with `mooring`, or turn on autostart with `mooring autostart on`."
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if len(f.urls) != 0 {
		t.Fatal("no browser without a server")
	}
}

func TestOpenAddrFlagBeatsTheAddressFile(t *testing.T) {
	dir := t.TempDir()
	token, _ := server.LoadOrCreateToken(dir)
	addr := liveServer(t, dir, token)
	if err := writeAddress(dir, "127.0.0.1:1"); err != nil {
		t.Fatal(err)
	}
	f := &fakeOpen{}
	if err := runOpen([]string{"-data", dir, "-addr", addr}, f.env(time.Now()), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if len(f.urls) != 1 || !strings.HasPrefix(f.urls[0], "http://"+addr+"/login?code=") {
		t.Fatalf("opened %v", f.urls)
	}
}

func TestOpenPrintsTheLinkWhenNoBrowserOpens(t *testing.T) {
	dir := t.TempDir()
	token, _ := server.LoadOrCreateToken(dir)
	addr := liveServer(t, dir, token)
	f := &fakeOpen{err: errors.New("no display")}
	var out bytes.Buffer
	if err := runOpen([]string{"-data", dir, "-addr", addr}, f.env(time.Now()), &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "http://"+addr+"/login?code=") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestOpenRefusesANonLocalAddress(t *testing.T) {
	f := &fakeOpen{}
	err := runOpen([]string{"-data", t.TempDir(), "-addr", "example.com:7340"}, f.env(time.Now()), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not a local address") || len(f.urls) != 0 {
		t.Fatalf("err = %v, opened %v", err, f.urls)
	}
}

func TestStartMessagePrintsALinkOnlyToATerminal(t *testing.T) {
	o := serveOpts{token: "tok"}
	var plain bytes.Buffer
	if err := startMessage(&plain, o, "127.0.0.1:7340", time.Now()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain.String(), "code=") || !strings.Contains(plain.String(), "Run `mooring open` to sign in") {
		t.Fatalf("not a terminal: %q", plain.String())
	}
	o.tty = true
	var term bytes.Buffer
	if err := startMessage(&term, o, "127.0.0.1:7340", time.Now()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(term.String(), "http://127.0.0.1:7340/login?code=v1.") {
		t.Fatalf("terminal: %q", term.String())
	}
	if isTerminal(&bytes.Buffer{}) {
		t.Fatal("a buffer is not a terminal")
	}
}

func TestTokenNewWritesADifferentToken(t *testing.T) {
	dir := t.TempDir()
	var first, second bytes.Buffer
	if err := run([]string{"token", "-data", dir}, &first, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"token", "-new", "-data", dir}, &second, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var again bytes.Buffer
	if err := run([]string{"token", "-data", dir}, &again, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if first.String() == second.String() || second.String() != again.String() {
		t.Fatalf("tokens: %q, %q, %q", first.String(), second.String(), again.String())
	}
	if err := run([]string{"-new", "-data", dir}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("-new without token must fail")
	}
}

func TestOpenRefusesAnotherProgramAtTheAddress(t *testing.T) {
	dir := t.TempDir()
	imposter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer imposter.Close()
	addr := strings.TrimPrefix(imposter.URL, "http://")
	if err := writeAddress(dir, addr); err != nil {
		t.Fatal(err)
	}
	f := &fakeOpen{}
	err := runOpen([]string{"-data", dir}, f.env(time.Now()), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "Something else is answering at "+addr) || len(f.urls) != 0 {
		t.Fatalf("err = %v, opened %v", err, f.urls)
	}
	// A server for a different token answers with a different proof.
	other := liveServer(t, t.TempDir(), "another-token")
	err = runOpen([]string{"-data", dir, "-addr", other}, f.env(time.Now()), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "Something else") || len(f.urls) != 0 {
		t.Fatalf("other token: err = %v, opened %v", err, f.urls)
	}
}

func TestWriteAddressReplacesAnOpenFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, addressFileName)
	if err := os.WriteFile(path, []byte("127.0.0.1:1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeAddress(dir, "127.0.0.1:2"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	data, _ := os.ReadFile(path)
	if err != nil || info.Mode().Perm() != 0o600 || string(data) != "127.0.0.1:2\n" {
		t.Fatalf("mode %v, data %q, err %v", info.Mode(), data, err)
	}
}

func TestOpenRefusesARelayToTheRealServer(t *testing.T) {
	dir := t.TempDir()
	token, _ := server.LoadOrCreateToken(dir)
	real := liveServer(t, dir, token)
	relay := httptest.NewServer(httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: real}))
	defer relay.Close()
	f := &fakeOpen{}
	err := runOpen([]string{"-data", dir, "-addr", strings.TrimPrefix(relay.URL, "http://")}, f.env(time.Now()), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "Something else") || len(f.urls) != 0 {
		t.Fatalf("relay: err = %v, opened %v", err, f.urls)
	}
}
