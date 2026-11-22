package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestAccountsListRequiresSession(t *testing.T) {
	ts, _ := liveServer(t)
	res, err := http.Get(ts.URL + "/api/accounts")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("without a session: %d", res.StatusCode)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/api/accounts", nil)
	for k, v := range sessionCookieHeader(t, ts) {
		req.Header[k] = v
	}
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	buf := make([]byte, 4096)
	n, _ := res.Body.Read(buf)
	if res.StatusCode != 200 || !strings.Contains(string(buf[:n]), `"default:claude"`) {
		t.Fatalf("list: %d %s", res.StatusCode, buf[:n])
	}
}
