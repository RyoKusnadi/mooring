package ssh

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func fileEnv(t *testing.T) *env {
	t.Helper()
	e := newEnv(t, envOpts{})
	e.mustConnect("c1", e.saveHost("password", ""))
	return e
}

func put(t *testing.T, e *env, rel, text string) string {
	t.Helper()
	p := filepath.Join(e.sftp, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o640); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListReadWriteRoundTrip(t *testing.T) {
	e := fileEnv(t)
	put(t, e, "b.txt", "bee")
	put(t, e, "A.txt", "ay")
	put(t, e, ".hidden", "h")
	put(t, e, "dir/inner.txt", "in")
	l, err := e.m.List("c1", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, en := range l.Entries {
		names = append(names, en.Name)
	}
	if got := strings.Join(names, ","); got != "dir,.hidden,A.txt,b.txt" || l.Total != 4 {
		t.Fatalf("entries %s total %d", got, l.Total)
	}
	if !l.Entries[1].Hidden || l.Entries[0].Kind != "dir" {
		t.Errorf("entries: %+v", l.Entries)
	}
	if _, err := e.m.List("c1", "b.txt"); code(err) != "NOT_A_FOLDER" {
		t.Errorf("list a file: %v", err)
	}
	if _, err := e.m.List("c1", "nope"); code(err) != "NOT_FOUND" {
		t.Errorf("list missing: %v", err)
	}
	if _, err := e.m.List("c1", "/etc"); code(err) != "BAD_REQUEST" {
		t.Errorf("absolute path: %v", err)
	}

	r, err := e.m.Read("c1", "dir/inner.txt", 0, 0, "")
	if err != nil || r.Content != "in" || r.Version == "" || r.Binary {
		t.Fatalf("read: %+v %v", r, err)
	}
	w, err := e.m.Write("c1", "dir/inner.txt", r.Version, []byte("changed"))
	if err != nil || w.Size != 7 {
		t.Fatalf("write: %+v %v", w, err)
	}
	if b, _ := os.ReadFile(filepath.Join(e.sftp, "dir/inner.txt")); string(b) != "changed" {
		t.Errorf("file holds %q", b)
	}
	if fi, _ := os.Stat(filepath.Join(e.sftp, "dir/inner.txt")); fi.Mode().Perm() != 0o640 {
		t.Errorf("mode %v, want 0640", fi.Mode().Perm())
	}
	if kids, _ := os.ReadDir(filepath.Join(e.sftp, "dir")); len(kids) != 1 {
		t.Errorf("a temporary file was left: %v", kids)
	}
	r2, _ := e.m.Read("c1", "dir/inner.txt", 0, 0, "")
	if r2.Version != w.Version {
		t.Errorf("version after write %q, read %q", w.Version, r2.Version)
	}

	// A new file: base is empty and the file must not exist.
	if _, err := e.m.Write("c1", "dir/new.txt", "", []byte("n")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.Write("c1", "dir/new.txt", "", []byte("again")); code(err) != "ALREADY_EXISTS" {
		t.Errorf("write over an existing file with no base: %v", err)
	}
	if _, err := e.m.Write("c1", "dir/gone.txt", "abc", []byte("x")); code(err) != "FILE_CHANGED" {
		t.Errorf("write to a deleted file: %v", err)
	}
	if _, err := e.m.Write("c1", "dir", "", []byte("x")); code(err) != "IS_A_FOLDER" {
		t.Errorf("write to a folder: %v", err)
	}
}

func TestRemoteChangeAfterReadIsFileChanged(t *testing.T) {
	e := fileEnv(t)
	p := put(t, e, "note.txt", "first")
	r, err := e.m.Read("c1", "note.txt", 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("someone else"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = e.m.Write("c1", "note.txt", r.Version, []byte("mine"))
	if code(err) != "FILE_CHANGED" {
		t.Fatalf("got %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "someone else" {
		t.Errorf("the changed file was overwritten: %q", b)
	}
	if kids, _ := os.ReadDir(e.sftp); len(kids) != 1 {
		t.Errorf("leftover files: %v", kids)
	}
}

func TestConcurrentWritesOneWins(t *testing.T) {
	e := fileEnv(t)
	put(t, e, "n.txt", "base")
	r, _ := e.m.Read("c1", "n.txt", 0, 0, "")
	var wg sync.WaitGroup
	res := make([]error, 4)
	for i := range res {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, res[i] = e.m.Write("c1", "n.txt", r.Version, []byte{byte('a' + i)})
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range res {
		if err == nil {
			ok++
		} else if code(err) != "FILE_CHANGED" {
			t.Errorf("unexpected: %v", err)
		}
	}
	if ok != 1 {
		t.Errorf("%d writes won, want 1", ok)
	}
}

func TestOutsideRootThroughRemoteSymlink(t *testing.T) {
	e := fileEnv(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	put(t, e, "ok.txt", "fine")
	if err := os.Symlink(outside, filepath.Join(e.sftp, "out")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(e.sftp, "peek")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("ok.txt", filepath.Join(e.sftp, "inside")); err != nil {
		t.Fatal(err)
	}
	for name, run := range map[string]func() error{
		"read through a folder link": func() error { _, err := e.m.Read("c1", "out/secret.txt", 0, 0, ""); return err },
		"read a file link":           func() error { _, err := e.m.Read("c1", "peek", 0, 0, ""); return err },
		"list a folder link":         func() error { _, err := e.m.List("c1", "out"); return err },
		"write through a link":       func() error { _, err := e.m.Write("c1", "out/new.txt", "", []byte("x")); return err },
		"dot dot":                    func() error { _, err := e.m.Read("c1", "../../etc/passwd", 0, 0, ""); return err },
		"mkdir through a link":       func() error { return e.m.Mkdir("c1", "out/d") },
		"delete through a link":      func() error { return e.m.Delete("c1", "out/secret.txt", false) },
		"rename out":                 func() error { _, err := e.m.Rename("c1", "ok.txt", "out/ok.txt"); return err },
		"download":                   func() error { _, _, _, err := e.m.Open("c1", "peek"); return err },
	} {
		if got := code(run()); got != "OUTSIDE_ROOT" {
			t.Errorf("%s: %s", name, got)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "new.txt")); err == nil {
		t.Error("a file was written outside the root")
	}
	if r, err := e.m.Read("c1", "inside", 0, 0, ""); err != nil || r.Content != "fine" {
		t.Errorf("a link inside the root: %v %+v", err, r)
	}
	l, _ := e.m.List("c1", "")
	for _, en := range l.Entries {
		if en.Name == "out" && (en.Unsafe != "outside" || !en.Link) {
			t.Errorf("the link out: %+v", en)
		}
	}
}

func TestRenameDeleteMkdirDownload(t *testing.T) {
	e := fileEnv(t)
	put(t, e, "a.txt", "aaa")
	put(t, e, "full/x.txt", "x")
	if err := e.m.Mkdir("c1", "made"); err != nil {
		t.Fatal(err)
	}
	if err := e.m.Mkdir("c1", "made"); code(err) != "ALREADY_EXISTS" {
		t.Errorf("mkdir twice: %v", err)
	}
	if _, err := e.m.Rename("c1", "a.txt", "made/b.txt"); err != nil {
		t.Fatal(err)
	}
	put(t, e, "c.txt", "c")
	if _, err := e.m.Rename("c1", "c.txt", "made/b.txt"); code(err) != "ALREADY_EXISTS" {
		t.Errorf("rename onto: %v", err)
	}
	name, size, rd, err := e.m.Open("c1", "made/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(rd)
	_ = rd.Close()
	if name != "b.txt" || size != 3 || string(data) != "aaa" {
		t.Errorf("download %s %d %q", name, size, data)
	}
	if err := e.m.Delete("c1", "full", false); code(err) != "BAD_REQUEST" {
		t.Errorf("delete a full folder: %v", err)
	}
	if err := e.m.Delete("c1", "full", true); err != nil {
		t.Fatal(err)
	}
	if err := e.m.Delete("c1", "made/b.txt", false); err != nil {
		t.Fatal(err)
	}
	if err := e.m.Delete("c1", "made", false); err != nil {
		t.Fatal(err)
	}
	if err := e.m.Delete("c1", "made", false); code(err) != "NOT_FOUND" {
		t.Errorf("delete twice: %v", err)
	}
	if _, err := e.m.Read("nope", "x", 0, 0, ""); code(err) != "NOT_CONNECTED" {
		t.Errorf("unknown connection: %v", err)
	}
}

func TestHostRootAndNoSFTP(t *testing.T) {
	e := fileEnv(t)
	put(t, e, "sub/in.txt", "i")
	put(t, e, "top.txt", "t")
	h, _ := e.m.SaveHost(Host{Name: "r", Address: "127.0.0.1", Port: e.fake.Port(), User: "u", Auth: "password", Root: filepath.Join(e.sftp, "sub")})
	e.mustConnect("c2", h)
	l, err := e.m.List("c2", "")
	if err != nil || len(l.Entries) != 1 || l.Entries[0].Name != "in.txt" {
		t.Fatalf("list in the root: %+v %v", l, err)
	}
	if _, err := e.m.Read("c2", "../top.txt", 0, 0, ""); code(err) != "OUTSIDE_ROOT" {
		t.Errorf("leaving the root: %v", err)
	}
}
