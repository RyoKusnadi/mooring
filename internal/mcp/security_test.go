package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRegistryTextNeverReachesACommandLine(t *testing.T) {
	bad := []string{"x; rm -rf /", "$(id)", "a b", "--flag", "@scope/../x", "x\nnode"}
	for _, name := range bad {
		for _, kind := range []string{"npm", "pypi", "oci"} {
			if _, _, ok := runTemplate(kind, name, "1.0.0", nil); ok {
				t.Errorf("%s accepted %q", kind, name)
			}
		}
	}
	if _, _, ok := runTemplate("npm", "pkg", "1.0.0 --evil", nil); ok {
		t.Error("version with a space accepted")
	}
	cmd, args, ok := runTemplate("npm", "@scope/pkg", "1.2.3", nil)
	if !ok || cmd != "npx" || strings.Join(args, " ") != "-y @scope/pkg@1.2.3" {
		t.Fatalf("npm template: %v %v %v", ok, cmd, args)
	}
	r := newRig(t)
	for _, a := range []SaveArgs{
		{Server: ToolServer{Name: "x", Kind: "npm", Command: "sh", Args: []string{"-c", "id"}}, Allow: true},
		{Server: ToolServer{Name: "y", Kind: "npm", Command: "npx", Args: []string{"-y", "pkg@1.0.0", "--extra"}}, Allow: true},
	} {
		if _, err := r.svc.Save(context.Background(), a); err == nil || !strings.Contains(err.Error(), "BAD_REQUEST") {
			t.Errorf("save accepted a command that is not the template: %v", err)
		}
	}
}

func TestPrivateAddressRefusedInTheDialer(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.2.3.4", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "::1", "fd00::1", "fe80::1", "::ffff:10.0.0.1", "0.0.0.0"} {
		if !blocked(netip.MustParseAddr(s)) {
			t.Errorf("%s not blocked", s)
		}
	}
	if blocked(netip.MustParseAddr("93.184.216.34")) {
		t.Error("a public address is blocked")
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer ts.Close()
	// The dialer itself refuses, so a name that resolves to a private address cannot get around it.
	if _, err := newClient(false, 5*time.Second).Get(ts.URL); err == nil || !isPrivateErr(err) {
		t.Fatalf("dial to loopback: %v", err)
	}
	resp, err := newClient(true, 5*time.Second).Get(ts.URL)
	if err != nil {
		t.Fatalf("allowPrivate: %v", err)
	}
	_ = resp.Body.Close()
}

func TestRedirectsAreCheckedAgain(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer target.Close()
	hop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer hop.Close()
	// Both are loopback, so a client that may use private addresses follows; one that may not never starts.
	if _, err := newClient(false, 5*time.Second).Get(hop.URL); err == nil {
		t.Fatal("redirect chain started at a private address")
	}
	c := newClient(true, 5*time.Second)
	next, _ := http.NewRequest("GET", target.URL, nil)
	via := make([]*http.Request, maxRedirects+1)
	for i := range via {
		via[i] = next
	}
	if err := c.CheckRedirect(next, via); err == nil {
		t.Fatal("too many redirects allowed")
	}
	httpsReq, _ := http.NewRequest("GET", "http://example.com/", nil)
	prev, _ := http.NewRequest("GET", "https://example.com/", nil)
	if err := newClient(false, time.Second).CheckRedirect(httpsReq, []*http.Request{prev}); err == nil {
		t.Fatal("https to http redirect allowed")
	}
}

func TestResponseSizeCap(t *testing.T) {
	if _, err := readCapped(strings.NewReader(strings.Repeat("a", 2000)), 1000); err == nil {
		t.Fatal("body over the cap was read")
	}
	if b, err := readCapped(strings.NewReader("small"), 1000); err != nil || string(b) != "small" {
		t.Fatalf("small body: %v", err)
	}
}

func TestSecretsStayOutOfSQLiteEventsAndLogs(t *testing.T) {
	r := newRig(t)
	a := r.stdioArgs("keeper", map[string]string{"API_KEY": "s3cret-value-1"})
	ts := r.save(a)
	if _, err := r.svc.Check(context.Background(), ts.ID); err != nil {
		t.Fatal(err)
	}
	rows, err := r.st.DB.Query(`SELECT name, kind, command, args, env, headers, url, source, tools FROM tool_servers`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		cols := make([]string, 9)
		ptrs := make([]any, 9)
		for i := range cols {
			ptrs[i] = &cols[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.Join(cols, "|"), "s3cret-value-1") {
			t.Fatalf("secret in SQLite: %v", cols)
		}
	}
	if strings.Contains(r.pub.all(), "s3cret-value-1") {
		t.Fatal("secret in events")
	}
	if v, _ := r.sec.Get(envKey(ts.ID, "API_KEY")); v != "s3cret-value-1" {
		t.Fatal("secret not in the secret store")
	}
	list, _ := r.svc.List()
	if !list[0].Env[1].Set {
		t.Fatalf("set flag missing: %+v", list[0].Env)
	}
	if !Secret("toolServer.save") || !Secret("toolServer.signInFinish") || Secret("toolServer.check") {
		t.Fatal("secret command names")
	}
}

func TestStateIsSingleUseAndExpires(t *testing.T) {
	now := time.Now()
	p := &pendingSignIns{m: map[string]*pendingSignIn{}}
	p.add("s1", &pendingSignIn{id: "a"}, now)
	if p.take("s1", now.Add(signInLife+time.Second)) != nil {
		t.Fatal("state older than 10 minutes accepted")
	}
	p.add("s2", &pendingSignIn{id: "a"}, now)
	if p.take("s2", now.Add(time.Minute)) == nil || p.take("s2", now.Add(time.Minute)) != nil {
		t.Fatal("state is not single use")
	}
	r := newRig(t)
	resp, err := http.Get(r.web.URL + callbackPath + "?state=nope&code=x")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("callback with an unknown state: %d", resp.StatusCode)
	}
	// The callback works without a session cookie but the Host guard in server still applies (see server tests).
	ts := r.save(r.remoteArgs("rem"))
	authURL, _, err := r.svc.SignIn(context.Background(), ts.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(authURL, "code_challenge_method=S256") || !strings.Contains(authURL, "state=") || !strings.Contains(authURL, "resource=") {
		t.Fatalf("authorize url: %s", authURL)
	}
	if strings.Contains(authURL, "code_verifier") {
		t.Fatal("verifier in the url")
	}
}

func TestStdioCheckKillsTheProcessGroupAndScrubsStderr(t *testing.T) {
	r := newRig(t)
	pidFile := filepath.Join(t.TempDir(), "pid")
	r.svc.FirstTimeout, r.svc.Timeout = 1500*time.Millisecond, 1500*time.Millisecond
	ts := r.save(r.stdioArgs("hang", map[string]string{"FAKE_MCP_HANG": "1", "FAKE_MCP_PIDFILE": pidFile}))
	start := time.Now()
	if _, err := r.svc.Check(context.Background(), ts.ID); err == nil {
		t.Fatal("a hanging server passed the check")
	}
	if time.Since(start) > 8*time.Second {
		t.Fatalf("check took %s", time.Since(start))
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(string(raw))
	deadline := time.Now().Add(3 * time.Second)
	for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if syscall.Kill(pid, 0) == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatal("grandchild survived the check")
	}

	bad := r.save(r.stdioArgs("noisy", map[string]string{"FAKE_MCP_FAIL": "1", "FAKE_MCP_SECRET": "hunter2-token"}))
	_, err = r.svc.Check(context.Background(), bad.ID)
	if err == nil || !strings.Contains(err.Error(), "CHECK_FAILED") {
		t.Fatalf("failing server: %v", err)
	}
	if strings.Contains(err.Error(), "hunter2-token") {
		t.Fatal("secret in the error text")
	}
	if len(err.Error()) > maxStderr+1024 {
		t.Fatalf("error text is %d bytes", len(err.Error()))
	}
}

func TestSearchAndEmbeddedCatalog(t *testing.T) {
	r := newRig(t)
	cat, err := r.svc.reg.get()
	if err != nil || len(cat.entries) < 100 {
		t.Fatalf("embedded catalog: %v", err)
	}
	page, next := cat.search("", 0, 10)
	if len(page) != 10 || next != 10 {
		t.Fatalf("page: %d next %d", len(page), next)
	}
}
