package files

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadWriteConflict(t *testing.T) {
	r := newRig(t, nil)
	r.write("a.txt", "hello\n")
	got, err := r.s.Read("p1", "a.txt", 0, 0, "")
	if err != nil || got.Content != "hello\n" || !got.Done || got.Binary || got.Version == "" || !got.Writable {
		t.Fatalf("read: %+v %v", got, err)
	}
	w, err := r.s.Write("p1", "a.txt", got.Version, []byte("one\n"))
	if err != nil || w.Version != Version([]byte("one\n")) {
		t.Fatalf("write: %+v %v", w, err)
	}
	// A second save with the old version loses.
	_, err = r.s.Write("p1", "a.txt", got.Version, []byte("two\n"))
	r.wantCode(err, "FILE_CHANGED")
	if b, _ := os.ReadFile(filepath.Join(r.root, "a.txt")); string(b) != "one\n" {
		t.Fatalf("file after refused save: %q", b)
	}
	// Changed underneath by another program.
	r.write("a.txt", "other\n")
	_, err = r.s.Write("p1", "a.txt", w.Version, []byte("three\n"))
	r.wantCode(err, "FILE_CHANGED")
	// New file: empty base. Existing file with an empty base conflicts.
	if _, err = r.s.Write("p1", "sub/new.txt", "", []byte("x")); code(err) != "NOT_FOUND" {
		t.Fatalf("missing parent: %v", err)
	}
	if _, err = r.s.Write("p1", "new.txt", "", []byte("x")); err != nil {
		t.Fatal(err)
	}
	_, err = r.s.Write("p1", "new.txt", "", []byte("y"))
	r.wantCode(err, "FILE_CHANGED")
	_, err = r.s.Write("p1", "gone.txt", "abc", []byte("y"))
	r.wantCode(err, "FILE_CHANGED")
	_, err = r.s.Write("p1", "a.txt", Version([]byte("other\n")), []byte{0xff, 0xfe})
	r.wantCode(err, "BAD_REQUEST")
	_, err = r.s.Read("p1", "", 0, 0, "")
	r.wantCode(err, "IS_A_FOLDER")
	_, err = r.s.Read("p1", "nope", 0, 0, "")
	r.wantCode(err, "NOT_FOUND")
	_, err = r.s.Read("p1", "/etc/passwd", 0, 0, "")
	r.wantCode(err, "BAD_REQUEST")
}

func TestWriteKeepsModeAndLeavesNoTemp(t *testing.T) {
	r := newRig(t, nil)
	r.write("run.sh", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(r.root, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	v := Version([]byte("#!/bin/sh\n"))
	if _, err := r.s.Write("p1", "run.sh", v, []byte("#!/bin/sh\necho\n")); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(r.root, "run.sh"))
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode %v", info.Mode())
	}
	des, _ := os.ReadDir(r.root)
	if len(des) != 1 {
		t.Fatalf("leftover files: %v", des)
	}
}

func TestBigFileReadsInChunks(t *testing.T) {
	r := newRig(t, nil)
	var sb strings.Builder
	for sb.Len() < 5<<20 {
		sb.WriteString("line of text with a multibyte character: héllo wörld ✓\n")
	}
	text := sb.String()
	r.write("big.txt", text)
	start := time.Now()
	var got strings.Builder
	var off int64
	version := ""
	chunks := 0
	for {
		res, err := r.s.Read("p1", "big.txt", off, 0, version)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Content) > MaxChunk {
			t.Fatalf("chunk of %d bytes", len(res.Content))
		}
		got.WriteString(res.Content)
		version, off, chunks = res.Version, res.Next, chunks+1
		if res.Done {
			break
		}
	}
	if got.String() != text || chunks < 5 {
		t.Fatalf("round trip failed: %d chunks, equal=%v", chunks, got.String() == text)
	}
	if d := time.Since(start); d > 5*time.Second { // loose: -race and a busy machine; a quadratic read takes far longer
		t.Fatalf("reading 5 MB in chunks took %v", d)
	}
	// A change between chunks is reported.
	r.write("big.txt", text+"more\n")
	_, err := r.s.Read("p1", "big.txt", off, 0, version)
	r.wantCode(err, "FILE_CHANGED")
	// Over the cap: metadata only.
	r.write("huge.txt", strings.Repeat("a", MaxFile+1))
	res, err := r.s.Read("p1", "huge.txt", 0, 0, "")
	if err != nil || !res.TooLarge || res.Content != "" {
		t.Fatalf("huge: %+v %v", res, err)
	}
	_, err = r.s.Write("p1", "x.txt", "", bytes.Repeat([]byte("a"), MaxFile+1))
	r.wantCode(err, "FILE_TOO_LARGE")
	// An offset inside a character is refused.
	_, err = r.s.Read("p1", "big.txt", 43, 0, "")
	r.wantCode(err, "BAD_REQUEST")
}

func TestBinaryFile(t *testing.T) {
	r := newRig(t, nil)
	r.write("img.bin", "PNG\x00\x01\x02")
	r.write("latin1.txt", "caf\xe9\n")
	for _, n := range []string{"img.bin", "latin1.txt"} {
		res, err := r.s.Read("p1", n, 0, 0, "")
		if err != nil || !res.Binary || res.Content != "" || res.Version == "" {
			t.Fatalf("%s: %+v %v", n, res, err)
		}
	}
}

func TestListSortedMarkedAndLimited(t *testing.T) {
	r := newRig(t, nil)
	r.write(".gitignore", "*.log\n/out/\n!keep.log\n")
	for _, n := range []string{"b.txt", "A.txt", "z.log", "keep.log", ".env", "node_modules/x.js", "out/y", "src/main.go"} {
		r.write(n, "x")
	}
	r.write("src/.gitignore", "gen_*\n")
	r.write("src/gen_a.go", "x")
	res, err := r.s.List("p1", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	byName := map[string]Entry{}
	for _, e := range res.Entries {
		names = append(names, e.Name)
		byName[e.Name] = e
	}
	if got := strings.Join(names, ","); got != "node_modules,out,src,.env,.gitignore,A.txt,b.txt,keep.log,z.log" {
		t.Fatalf("order: %s", got)
	}
	for n, want := range map[string]bool{"node_modules": true, "out": true, "z.log": true, "keep.log": false, "src": false, "b.txt": false} {
		if byName[n].Ignored != want {
			t.Errorf("%s ignored = %v", n, !want)
		}
	}
	if !byName[".env"].Hidden || byName["b.txt"].Hidden {
		t.Error("hidden marks wrong")
	}
	sub, err := r.s.List("p1", "src")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range sub.Entries {
		if e.Name == "gen_a.go" && !e.Ignored {
			t.Error("nested .gitignore not applied")
		}
		if e.Name == "main.go" && e.Ignored {
			t.Error("main.go marked ignored")
		}
	}
	inner, _ := r.s.List("p1", "node_modules")
	if !inner.Entries[0].Ignored {
		t.Error("entries of an ignored folder should be ignored")
	}
	_, err = r.s.List("p1", "b.txt")
	r.wantCode(err, "NOT_A_FOLDER")

	for i := range MaxEntries + 10 {
		r.write("many/"+strings.Repeat("f", 1)+string(rune('a'+i%26))+strings.Repeat("0", 1)+itoa(i), "")
	}
	many, err := r.s.List("p1", "many")
	if err != nil || !many.Truncated || len(many.Entries) != MaxEntries || many.Total != MaxEntries+10 {
		t.Fatalf("limit: %v trunc=%v n=%d total=%d", err, many != nil && many.Truncated, len(many.Entries), many.Total)
	}
}

func itoa(i int) string {
	const d = "0123456789"
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{d[i%10]}, b...)
	}
	return string(b)
}

func TestCreateRenameDelete(t *testing.T) {
	r := newRig(t, nil)
	if _, err := r.s.Create("p1", "d", "dir"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.s.Create("p1", "d/f.txt", "file"); err != nil {
		t.Fatal(err)
	}
	_, err := r.s.Create("p1", "d/f.txt", "file")
	r.wantCode(err, "ALREADY_EXISTS")
	_, err = r.s.Create("p1", "nodir/f.txt", "file")
	r.wantCode(err, "NOT_FOUND")
	r.write("e.txt", "e")
	_, err = r.s.Rename("p1", "d/f.txt", "e.txt")
	r.wantCode(err, "ALREADY_EXISTS")
	if _, err = r.s.Rename("p1", "d/f.txt", "g.txt"); err != nil {
		t.Fatal(err)
	}
	_, err = r.s.Rename("p1", "d", "d/inner")
	r.wantCode(err, "BAD_REQUEST")
	_, err = r.s.Rename("p1", "", "x")
	r.wantCode(err, "PROTECTED")
	if _, err = r.s.Rename("p1", "e.txt", "E.txt"); err != nil {
		t.Fatal(err)
	}
	_, err = r.s.Delete("p1", "", false)
	r.wantCode(err, "PROTECTED")
	_, err = r.s.Delete("p1", "../proj", true)
	r.wantCode(err, "OUTSIDE_PROJECT")
	del, err := r.s.Delete("p1", "d", false)
	if err != nil || !del["trashed"] {
		t.Fatalf("trash: %v %v", del, err)
	}
	if _, err := os.Stat(filepath.Join(r.root, "d")); !os.IsNotExist(err) {
		t.Fatal("d still there")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(r.root), "trash", "d")); err != nil {
		t.Fatalf("not in trash: %v", err)
	}
}

func TestDeleteWithoutTrashNeedsPermanent(t *testing.T) {
	r := newRig(t, func(c *Config) { c.TrashDir = "-" })
	r.write("a.txt", "a")
	_, err := r.s.Delete("p1", "a.txt", false)
	r.wantCode(err, "TRASH_UNAVAILABLE")
	if _, err := os.Stat(filepath.Join(r.root, "a.txt")); err != nil {
		t.Fatal("file was deleted without permission")
	}
	del, err := r.s.Delete("p1", "a.txt", true)
	if err != nil || del["trashed"] {
		t.Fatalf("permanent: %v %v", del, err)
	}
}
