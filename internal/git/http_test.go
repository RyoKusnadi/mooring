package git

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHTTPReads(t *testing.T) {
	r := newRig(t, opts{})
	mux := http.NewServeMux()
	r.s.Mount(mux, func(h http.Handler) http.Handler { return h })
	r.write("n.txt", "n\n")
	get := func(path string, q url.Values) (int, map[string]any) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", path+"?"+q.Encode(), nil))
		var v map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &v)
		return rec.Code, v
	}
	code, v := get("/api/git/status", url.Values{"project": {"p1"}})
	if code != 200 || v["branch"] != "main" || len(v["files"].([]any)) != 1 {
		t.Fatalf("status: %d %v", code, v)
	}
	code, v = get("/api/git/diff", url.Values{"project": {"p1"}, "path": {"n.txt"}, "staged": {"0"}})
	if code != 200 || len(v["hunks"].([]any)) != 1 {
		t.Fatalf("diff: %d %v", code, v)
	}
	code, v = get("/api/git/diff", url.Values{"project": {"p1"}, "path": {"../x"}})
	if code != 403 || v["ok"] != false {
		t.Fatalf("outside: %d %v", code, v)
	}
	code, _ = get("/api/git/worktrees", url.Values{"project": {"nope"}})
	if code != 404 {
		t.Fatalf("unknown project: %d", code)
	}
	r2 := newRig(t, opts{noRepo: true})
	mux2 := http.NewServeMux()
	r2.s.Mount(mux2, func(h http.Handler) http.Handler { return h })
	rec := httptest.NewRecorder()
	mux2.ServeHTTP(rec, httptest.NewRequest("GET", "/api/git/worktrees?project=p1", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"worktrees":[]`) {
		t.Fatalf("no repo worktrees: %d %s", rec.Code, rec.Body.String())
	}
}

func TestHTTPHistory(t *testing.T) {
	r := newRig(t, opts{})
	mux := http.NewServeMux()
	r.s.Mount(mux, func(h http.Handler) http.Handler { return h })
	r.write("a.txt", "a\n")
	r.commitAll("second")
	get := func(path string, q url.Values) (int, map[string]any) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", path+"?"+q.Encode(), nil))
		var v map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &v)
		return rec.Code, v
	}
	p := url.Values{"project": {"p1"}}
	code, v := get("/api/git/log", p)
	if code != 200 || len(v["commits"].([]any)) != 2 || v["total"] != float64(2) || v["next"] != "" {
		t.Fatalf("log: %d %v", code, v)
	}
	id := v["commits"].([]any)[0].(map[string]any)["id"].(string)
	code, v = get("/api/git/log", url.Values{"project": {"p1"}, "q": {"second"}, "in": {"message"}})
	if code != 200 || len(v["commits"].([]any)) != 1 || v["total"] != float64(-1) {
		t.Fatalf("search: %d %v", code, v)
	}
	if code, _ = get("/api/git/log", url.Values{"project": {"p1"}, "limit": {"x"}}); code != 400 {
		t.Fatalf("bad limit: %d", code)
	}
	if code, v = get("/api/git/log", url.Values{"project": {"p1"}, "cursor": {"0000000000000000.1"}}); code != 409 || v["error"].(map[string]any)["code"] != "LOG_CHANGED" {
		t.Fatalf("stale cursor: %d %v", code, v)
	}
	code, v = get("/api/git/commit", url.Values{"project": {"p1"}, "id": {id}})
	if code != 200 || len(v["files"].([]any)) != 1 {
		t.Fatalf("commit: %d %v", code, v)
	}
	code, v = get("/api/git/diff", url.Values{"project": {"p1"}, "commit": {id}, "path": {"a.txt"}})
	if code != 200 || len(v["hunks"].([]any)) != 1 || v["version"] == "" {
		t.Fatalf("commit diff: %d %v", code, v)
	}
	if code, _ = get("/api/git/branches", p); code != 200 {
		t.Fatalf("branches: %d", code)
	}
	if code, v = get("/api/git/stashes", p); code != 200 || len(v["stashes"].([]any)) != 0 {
		t.Fatalf("stashes: %d %v", code, v)
	}
}
