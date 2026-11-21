package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fileMap(info *CommitInfo) map[string]CommitFile {
	m := map[string]CommitFile{}
	for _, f := range info.Files {
		m[f.Path] = f
	}
	return m
}

func (r *rig) detail(id string) *CommitInfo {
	r.t.Helper()
	info, err := r.s.CommitDetail(bg, "p1", "", id)
	r.must(err)
	return info
}

func TestCommitDetail(t *testing.T) {
	r := newRig(t, opts{})
	// Root commit.
	root := r.detail(r.git(r.root, "rev-parse", "HEAD"))
	if len(root.Files) != 1 || root.Files[0].Path != "README.md" || root.Files[0].Change != "added" || len(root.Commit.Parents) != 0 || root.Message != "first" || root.Committer != "Test <test@example.com>" {
		t.Fatalf("root %+v", root)
	}
	rd, err := r.s.CommitDiff(bg, "p1", "", root.Commit.ID, "README.md")
	r.must(err)
	if len(rd.Hunks) != 1 || rd.Hunks[0].Lines[0][1] != "hello" || rd.Version == "" {
		t.Fatalf("root diff %+v", rd)
	}
	// Rename and binary, the message with a body.
	r.write("big.txt", strings.Repeat("some content line\n", 30))
	r.commitAll("big")
	r.git(r.root, "mv", "big.txt", "sub-big.txt")
	r.write("sub-big.txt", strings.Repeat("some content line\n", 30)+"extra\n")
	if err := os.WriteFile(filepath.Join(r.root, "bin.dat"), []byte("a\x00b"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.write("README.md", "hello\nworld\n")
	r.commitAll("rename and more\n\nlong body here")
	info := r.detail(r.git(r.root, "rev-parse", "--short", "HEAD"))
	if info.Message != "rename and more\n\nlong body here" || info.Commit.Subject != "rename and more" || len(info.Commit.Refs) != 1 {
		t.Fatalf("info %+v", info)
	}
	fm := fileMap(info)
	if f := fm["sub-big.txt"]; f.Change != "renamed" || f.OrigPath != "big.txt" {
		t.Fatalf("rename %+v", f)
	}
	if !fm["bin.dat"].Binary || fm["bin.dat"].Change != "added" || fm["README.md"].Change != "modified" || fm["README.md"].Binary {
		t.Fatalf("files %+v", info.Files)
	}
	d, err := r.s.CommitDiff(bg, "p1", "", info.Commit.ID, "sub-big.txt")
	r.must(err)
	if d.OrigPath != "big.txt" || len(d.Hunks) != 1 || d.Hunks[0].Lines[len(d.Hunks[0].Lines)-1][1] != "extra" {
		t.Fatalf("rename diff %+v", d)
	}
	bd, err := r.s.CommitDiff(bg, "p1", "", info.Commit.ID, "bin.dat")
	r.must(err)
	if !bd.Binary {
		t.Fatalf("binary diff %+v", bd)
	}
	none, err := r.s.CommitDiff(bg, "p1", "", info.Commit.ID, "nothing.txt")
	r.must(err)
	if len(none.Hunks) != 0 {
		t.Fatal("hunks for a file the commit did not touch")
	}
	// Merge: against the first parent.
	r.git(r.root, "checkout", "-q", "-b", "side", "HEAD~1")
	r.write("side.txt", "s\n")
	r.commitAll("side")
	r.git(r.root, "checkout", "-q", "main")
	r.git(r.root, "merge", "--no-ff", "-q", "-m", "merge side", "side")
	m := r.detail(r.git(r.root, "rev-parse", "HEAD"))
	if len(m.Commit.Parents) != 2 || len(m.Files) != 1 || m.Files[0].Path != "side.txt" || m.Files[0].Change != "added" {
		t.Fatalf("merge %+v", m)
	}
	// Errors.
	_, err = r.s.CommitDetail(bg, "p1", "", "nonsense!")
	r.wantCode(err, "BAD_REQUEST")
	_, err = r.s.CommitDetail(bg, "p1", "", "deadbeef")
	r.wantCode(err, "NOT_FOUND")
	_, err = r.s.CommitDiff(bg, "p1", "", m.Commit.ID, "../x")
	r.wantCode(err, "BAD_REQUEST")
}
