package lsp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// waitWatched waits until the server watches the folder of its open document. The watch starts in
// the background after didOpen, so a file written before then is not seen.
func (e *env) waitWatched(t *testing.T, k instKey) {
	t.Helper()
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		e.svc.mu.Lock()
		i := e.svc.insts[k]
		e.svc.mu.Unlock()
		if i != nil {
			i.mu.Lock()
			n := len(i.watched)
			i.mu.Unlock()
			if n > 0 {
				return
			}
		}
	}
	t.Fatal("the folder of the open document is not watched")
}

func TestFilesChangedOnDiskReachTheServer(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	e.allow("go")
	a := e.write("a.go", "package a\n")
	b := e.write("b.go", "package a\n")
	other := e.write("other/c.go", "")
	c := e.connect("go", "")
	c.handshake(1, map[string]any{})
	uri := uriOf(a)
	c.open(uri, "package a\n")
	c.diagnostics(uri, func(d []any) bool { return len(d) == 0 })
	e.waitWatched(t, instKey{"p1", "", "go"})

	// A file an agent edits in the same folder gives errors in the open document.
	if err := os.WriteFile(b, []byte("BREAKS_OTHERS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := c.diagnostics(uri, func(d []any) bool { return len(d) == 1 })
	if d[0].(map[string]any)["message"] != "broken by b.go" {
		t.Fatalf("diagnostic: %v", d)
	}
	// Fixing the file clears it again.
	if err := os.WriteFile(b, []byte("fine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.diagnostics(uri, func(d []any) bool { return len(d) == 0 })
	// A deleted file clears it too, a new one raises it.
	n := filepath.Join(e.root, "new.go")
	if err := os.WriteFile(n, []byte("BREAKS_OTHERS"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.diagnostics(uri, func(d []any) bool { return len(d) == 1 })
	if err := os.Remove(n); err != nil {
		t.Fatal(err)
	}
	c.diagnostics(uri, func(d []any) bool { return len(d) == 0 })
	// Folders without an open document are not watched.
	if err := os.WriteFile(other, []byte("BREAKS_OTHERS"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.write("poke.go", "BREAKS_OTHERS")
	c.diagnostics(uri, func(d []any) bool { return len(d) == 1 && d[0].(map[string]any)["message"] == "broken by poke.go" })
}

func TestWatchEndsWithTheServer(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	e.allow("go")
	c := e.connect("go", "")
	c.handshake(1, map[string]any{})
	c.open(uriOf(e.write("a.go", "")), "x")
	e.svc.stopKey(instKey{"p1", "", "go"}, "closed")
	_ = c.waitClose()
	e.pub.wait(t, "lsp.stopped", nil)
	e.watcher.CloseProject("p1")
	// A change after the stop reaches nobody and breaks nothing.
	e.write("b.go", "BREAKS_OTHERS")
}
