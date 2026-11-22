package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/fakemcp"
)

func TestHeadersAndTokenStayOnTheirOrigin(t *testing.T) {
	var leaked atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.Header.Get("X-Api-Key") != "" || q.Header.Get("Authorization") != "" {
			leaked.Add(1)
		}
	}))
	defer other.Close()
	hop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		http.Redirect(w, q, other.URL+"/mcp", http.StatusTemporaryRedirect)
	}))
	defer hop.Close()
	r := newRig(t)
	ts := r.save(SaveArgs{Server: ToolServer{Name: "redir", Kind: "http", URL: hop.URL + "/mcp", AllowPrivate: true,
		Headers: []Var{{Name: "X-Api-Key"}}}, Secrets: &SecretValues{Headers: map[string]string{"X-Api-Key": "key-123"}}, Allow: true})
	if _, err := r.svc.Check(context.Background(), ts.ID); err == nil {
		t.Fatal("a redirect to another origin passed the check")
	}
	if leaked.Load() != 0 {
		t.Fatal("headers reached another origin")
	}
}

func TestTokenEndpointTroubleKeepsTheTokens(t *testing.T) {
	status := atomic.Int32{}
	status.Store(503)
	body := `{"error":"invalid_grant"}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()
	r := newRig(t)
	saved := r.save(r.remoteArgs("tok"))
	rc, _ := r.svc.get(saved.ID)
	rc.AllowPrivate = true
	tok := tokenRecord{Access: "old", Refresh: "ref", Expires: time.Now().Add(-time.Hour).UnixMilli(), TokenURL: ts.URL, ClientID: "c", Resource: "x"}
	if err := r.svc.saveToken(saved.ID, tok); err != nil {
		t.Fatal(err)
	}
	r.svc.setSignIn(saved.ID, "signedIn")
	for _, code := range []int{503, 429, 502} {
		status.Store(int32(code))
		if _, err := r.svc.accessToken(context.Background(), rc, false); err == nil || err == errSignIn {
			t.Fatalf("status %d: %v", code, err)
		}
		if _, ok := r.svc.loadToken(saved.ID); !ok {
			t.Fatalf("status %d deleted the tokens", code)
		}
	}
	status.Store(400)
	if _, err := r.svc.accessToken(context.Background(), rc, false); err != errSignIn {
		t.Fatalf("invalid_grant: %v", err)
	}
	if _, ok := r.svc.loadToken(saved.ID); ok {
		t.Fatal("refused refresh kept the tokens")
	}
}

func TestEditDuringCheckAndCheckLimits(t *testing.T) {
	r := newRig(t)
	a := r.save(r.stdioArgs("one", nil))
	_, gen, release, err := r.svc.beginCheck(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := r.svc.beginCheck(a.ID); err == nil || !strings.Contains(err.Error(), "BUSY") {
		t.Fatalf("second check of one server: %v", err)
	}
	edit := r.stdioArgs("one", map[string]string{"X": "1"})
	edit.Server.ID = a.ID
	r.save(edit)
	if r.svc.gen[a.ID] == gen {
		t.Fatal("an edit did not change the generation")
	}
	release()
	var releases []func()
	for i := 0; i < maxChecks; i++ {
		s := r.save(r.stdioArgs("s"+string(rune('a'+i)), nil))
		_, _, rel, err := r.svc.beginCheck(s.ID)
		if err != nil {
			t.Fatalf("check %d: %v", i, err)
		}
		releases = append(releases, rel)
	}
	extra := r.save(r.stdioArgs("extra", nil))
	if _, _, _, err := r.svc.beginCheck(extra.ID); err == nil || !strings.Contains(err.Error(), "BUSY") {
		t.Fatalf("fifth check: %v", err)
	}
	for _, rel := range releases {
		rel()
	}
}

func TestPendingSignInDiesWithTheAddress(t *testing.T) {
	r := newRig(t)
	ts := r.save(r.remoteArgs("rem"))
	authURL, _, err := r.svc.SignIn(context.Background(), ts.ID)
	if err != nil {
		t.Fatal(err)
	}
	edit := r.remoteArgs("rem")
	edit.Server.ID = ts.ID
	edit.Server.URL = r.fts.URL + "/mcp?other=1"
	r.save(edit)
	back, err := fakemcp.Approve(authURL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(back)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("callback after an address change: %d", resp.StatusCode)
	}
	// Signing out also forgets a waiting sign-in.
	authURL, _, err = r.svc.SignIn(context.Background(), ts.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.SignOut(ts.ID); err != nil {
		t.Fatal(err)
	}
	back, _ = fakemcp.Approve(authURL)
	resp, err = http.Get(back)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("callback after sign-out: %d", resp.StatusCode)
	}
}

func TestStopGroup(t *testing.T) {
	mark := filepath.Join(t.TempDir(), "term")
	cmd := exec.Command("sh", "-c", `trap 'touch "$0"; exit 0' TERM; while :; do sleep 0.1; done`, mark)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	time.Sleep(200 * time.Millisecond)
	stopGroup(cmd.Process.Pid, exited, true)
	if _, err := os.Stat(mark); err != nil {
		t.Fatal("a graceful stop did not send SIGTERM first")
	}
	// After the leader has been reaped nothing is signalled and the call returns at once.
	start := time.Now()
	stopGroup(cmd.Process.Pid, exited, true)
	if time.Since(start) > time.Second {
		t.Fatal("stopGroup waited for a process that was gone")
	}
}

func TestLookupFailureIsRefused(t *testing.T) {
	err := checkAddress(context.Background(), "https://no-such-host.invalid/mcp", false)
	if err == nil || !strings.Contains(err.Error(), "BAD_REQUEST") {
		t.Fatalf("unresolvable name: %v", err)
	}
}

func TestMoreBlockedRanges(t *testing.T) {
	for _, s := range []string{"::2", "2002:c000:204::1", "2001::1", "64:ff9b:1::5"} {
		if !blocked(netip.MustParseAddr(s)) {
			t.Errorf("%s not blocked", s)
		}
	}
}

func TestSignInSiteGetsNoPrivateException(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{}")) }))
	defer ts.Close()
	r := newRig(t)
	rc := rec{ToolServer: ToolServer{URL: "https://tools.example.com/mcp", AllowPrivate: true}}
	if _, err := r.svc.clientFor(rc, ts.URL).Get(ts.URL); err == nil || !isPrivateErr(err) {
		t.Fatalf("other host reached a private address: %v", err)
	}
	rc.URL = ts.URL + "/mcp"
	resp, err := r.svc.clientFor(rc, ts.URL).Get(ts.URL)
	if err != nil {
		t.Fatalf("same host: %v", err)
	}
	_ = resp.Body.Close()
}
