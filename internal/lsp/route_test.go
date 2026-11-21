package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOneOwnerPerFileWhateverTheSpelling(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	e.allow("go")
	a, b := e.connect("go", ""), e.connect("go", "")
	a.handshake(1, map[string]any{})
	b.handshake(1, map[string]any{})
	file := e.write("a.go", "")
	if err := os.Symlink(file, filepath.Join(e.root, "alias.go")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(file)
	plain := uriOf(file)
	dot := "file://" + dir + "/./a.go"
	enc := "file://" + dir + "/%61.go"
	alias := "file://" + filepath.Join(e.root, "alias.go")
	// Two sockets have no order between them, so each step waits for a round
	// trip through the server before the other socket goes on.
	id := 10
	step := func(c *client, do func()) {
		do()
		id++
		c.state(id)
	}
	step(a, func() { a.open(plain, "one") })
	step(b, func() { b.open(dot, "two") })
	step(a, func() { a.open(enc, "three") })
	step(b, func() { b.open(alias, "four") })
	step(a, func() { a.change(plain, "dropped: a no longer owns it") })
	b.change(enc, "kept")
	seen, open, _ := b.state(2)
	opens, closes := 0, 0
	for _, s := range seen {
		switch {
		case strings.HasPrefix(s, "textDocument/didOpen"):
			opens++
		case strings.HasPrefix(s, "textDocument/didClose"):
			closes++
		}
	}
	if opens != 4 || closes != 3 || len(open) != 1 {
		t.Fatalf("opens %d closes %d open %v; seen %v", opens, closes, open, seen)
	}
	changes := 0
	for _, s := range seen {
		if strings.HasPrefix(s, "textDocument/didChange") {
			changes++
		}
	}
	if changes != 1 {
		t.Fatalf("%d changes reached the server, want 1", changes)
	}
}

func TestEveryURIInTheParamsIsChecked(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	e.allow("go")
	c := e.connect("go", "")
	c.handshake(1, map[string]any{})
	good := uriOf(e.write("a.go", ""))
	bad := "file:///etc/passwd"
	cases := map[string]any{
		"call hierarchy item": map[string]any{"item": map[string]any{"uri": bad}},
		"rename files":        map[string]any{"files": []any{map[string]any{"oldUri": good, "newUri": bad}}},
		"uri list":            map[string]any{"uris": []any{good, bad}},
		"number uri":          map[string]any{"textDocument": map[string]any{"uri": 5}},
		"missing uri":         map[string]any{"textDocument": map[string]any{}},
		"other scheme":        map[string]any{"textDocument": map[string]any{"uri": "http://x/a.go"}},
	}
	id := 10
	for name, params := range cases {
		id++
		res := c.call(id, "workspace/willRenameFiles", params)
		errObj, _ := res["error"].(map[string]any)
		if errObj == nil || errObj["code"] != float64(-32602) {
			t.Errorf("%s: got %v, want error -32602", name, res)
		}
	}
	res := c.call(99, "textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": good}, "position": map[string]any{"line": 0, "character": 0}})
	if res["error"] != nil {
		t.Fatalf("a file inside the place was refused: %v", res)
	}
}

func TestRequestsPastTheHoldLimitGetAnError(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	e.allow("go")
	c := e.connect("go", "")
	uri := uriOf(e.write("a.go", ""))
	for n := 1; n <= maxHeld+1; n++ {
		c.send(map[string]any{"jsonrpc": "2.0", "id": n, "method": "textDocument/hover",
			"params": map[string]any{"textDocument": map[string]any{"uri": uri}}})
	}
	res := c.wait(func(m map[string]any) bool { return m["id"] == float64(maxHeld+1) })
	if res["error"] == nil {
		t.Fatalf("no error for the request past the limit: %v", res)
	}
}

func TestCrashIsNoticedWhenAChildKeepsStdout(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	e.allow("go")
	c := e.connect("go", "")
	c.handshake(1, map[string]any{})
	pid := e.pidOf("go")
	c.open(uriOf(e.write("a.go", "")), "ORPHAN")
	done := make(chan struct{})
	go func() { _ = c.waitClose(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the crash was not noticed while a child held stdout")
	}
	e.pub.wait(t, "lsp.stopped", func(m map[string]any) bool { return m["reason"] == "crashed" })
	waitDead(t, pid)
}
