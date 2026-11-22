package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestFilesOverHTTPAndWatchOverSocket(t *testing.T) {
	ts, url := liveServer(t)
	hdr := sessionCookieHeader(t, ts)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.CloseNow() }()
	send := func(v string) {
		if err := ws.Write(ctx, websocket.MessageText, []byte(v)); err != nil {
			t.Fatal(err)
		}
	}
	next := func() map[string]any {
		_, data, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		return m
	}
	// until reads frames until one satisfies pred.
	until := func(pred func(map[string]any) bool) map[string]any {
		for {
			if f := next(); pred(f) {
				return f
			}
		}
	}
	send(`{"t":"hello","epoch":"","pos":0}`)
	home := next()["snapshot"].(map[string]any)["home"].(string)
	send(`{"t":"cmd","id":"c1","name":"project.create","args":{"parent":"` + home + `","name":"demo"}}`)
	res := until(func(f map[string]any) bool { return f["t"] == "result" && f["id"] == "c1" })
	if res["ok"] != true {
		t.Fatalf("create project: %v", res)
	}
	pid := res["value"].(map[string]any)["id"].(string)
	proj := filepath.Join(home, "demo")
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(proj, "out")); err != nil {
		t.Fatal(err)
	}

	call := func(method, path, id, body string) (int, string) {
		req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
		req.Header = hdr.Clone()
		if id != "" {
			req.Header.Set("Mooring-Command-Id", id)
		}
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	q := "?project=" + pid

	// Without a session nothing is served.
	if resp, err := http.Get(ts.URL + "/api/files/list" + q); err != nil || resp.StatusCode != 401 {
		t.Fatalf("no session: %v %v", resp, err)
	}
	// Save a new file, then repeat the same command id: the first outcome, one write.
	st, body := call("PUT", "/api/files/content"+q+"&path=a.txt&base=", "w1", "hello\n")
	if st != 200 || !strings.Contains(body, `"ok":true`) {
		t.Fatalf("write: %d %s", st, body)
	}
	if err := os.WriteFile(filepath.Join(proj, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st2, body2 := call("PUT", "/api/files/content"+q+"&path=a.txt&base=", "w1", "hello\n")
	if st2 != 200 || body2 != body {
		t.Fatalf("repeat: %d %s", st2, body2)
	}
	if b, _ := os.ReadFile(filepath.Join(proj, "a.txt")); string(b) != "changed\n" {
		t.Fatalf("the command ran twice: %q", b)
	}
	if st, _ := call("PUT", "/api/files/content"+q+"&path=a.txt&base=", "w1", "different"); st != 409 {
		t.Fatalf("same id, other body: %d", st)
	}
	if st, body := call("PUT", "/api/files/content"+q+"&path=a.txt&base=nope", "w2", "x"); st != 409 || !strings.Contains(body, "FILE_CHANGED") {
		t.Fatalf("conflict: %d %s", st, body)
	}
	if st, body := call("GET", "/api/files/read"+q+"&path=a.txt", "", ""); st != 200 || !strings.Contains(body, `"content":"changed\n"`) {
		t.Fatalf("read: %d %s", st, body)
	}
	if st, body := call("GET", "/api/files/list"+q+"&path=", "", ""); st != 200 || !strings.Contains(body, `"unsafe":"outside"`) {
		t.Fatalf("list: %d %s", st, body)
	}
	for _, p := range []string{"/api/files/list" + q + "&path=out", "/api/files/read" + q + "&path=out/secret.txt"} {
		if st, body := call("GET", p, "", ""); st != 403 || !strings.Contains(body, "OUTSIDE_PROJECT") {
			t.Fatalf("%s: %d %s", p, st, body)
		}
	}
	if st, _ := call("PUT", "/api/files/content"+q+"&path=out/new.txt&base=", "w3", "x"); st != 403 {
		t.Fatalf("write through link: %d", st)
	}
	if _, err := os.Stat(filepath.Join(outside, "new.txt")); err == nil {
		t.Fatal("wrote outside the project")
	}
	if st, body := call("GET", "/api/files/search"+q+"&q=changed", "", ""); st != 200 || !strings.Contains(body, `"path":"a.txt"`) || !strings.Contains(body, `"t":"done"`) {
		t.Fatalf("search: %d %s", st, body)
	}

	// Watch a file, change it from outside, see the event with the new version.
	send(`{"t":"cmd","id":"c2","name":"file.watch","args":{"projectId":"` + pid + `","holder":"win1","dirs":[""],"files":["a.txt"]}}`)
	if r := until(func(f map[string]any) bool { return f["t"] == "result" && f["id"] == "c2" }); r["ok"] != true {
		t.Fatalf("watch: %v", r)
	}
	if err := os.WriteFile(filepath.Join(proj, "a.txt"), []byte("by an agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ev := until(func(f map[string]any) bool { return f["t"] == "event" && f["kind"] == "file.changed" })
	if ev["data"].(map[string]any)["path"] != "a.txt" || ev["data"].(map[string]any)["version"] == "" {
		t.Fatalf("event: %v", ev)
	}
	send(`{"t":"cmd","id":"c3","name":"file.watch","args":{"projectId":"` + pid + `","holder":"win1","files":["out/secret.txt"]}}`)
	r := until(func(f map[string]any) bool { return f["t"] == "result" && f["id"] == "c3" })
	if r["ok"] != false || r["error"].(map[string]any)["code"] != "OUTSIDE_PROJECT" {
		t.Fatalf("watch through link: %v", r)
	}
}
