package git

import (
	"os"
	"path/filepath"
	"testing"
)

func (r *rig) stashes() []Stash {
	r.t.Helper()
	list, err := r.s.Stashes(bg, "p1", "")
	r.must(err)
	return list
}

func (r *rig) save(msg string, untracked bool) string {
	r.t.Helper()
	v, err := r.s.StashSave(bg, "p1", "", msg, untracked)
	r.must(err)
	return v["id"]
}

func TestStashSave(t *testing.T) {
	r := newRig(t, opts{})
	_, err := r.s.StashSave(bg, "p1", "", "", false)
	r.wantCode(err, "NOTHING_TO_STASH")
	r.write("new.txt", "untracked\n")
	_, err = r.s.StashSave(bg, "p1", "", "", false)
	r.wantCode(err, "NOTHING_TO_STASH")
	if r.read("new.txt") != "untracked\n" {
		t.Fatal("untracked file touched")
	}
	r.write("README.md", "changed\n")
	r.write("staged.txt", "s\n")
	r.git(r.root, "add", "staged.txt")
	id := r.save("", false)
	if r.read("README.md") != "hello\n" || len(r.status("").Files) != 1 {
		t.Fatalf("files not restored: %+v", r.status("").Files)
	}
	list := r.stashes()
	if len(list) != 1 || list[0].ID != id || list[0].Index != 0 || list[0].Branch != "main" || list[0].Message == "" || list[0].Time == 0 {
		t.Fatalf("list %+v", list)
	}
	info := r.detail(id)
	fm := fileMap(info)
	if len(info.Files) != 2 || fm["README.md"].Change != "modified" || fm["staged.txt"].Change != "added" {
		t.Fatalf("stash files %+v", info.Files)
	}

	// With untracked files, and a message.
	r.write("u.txt", "u1\nu2\n")
	r.write("README.md", "again\n")
	id2 := r.save("my work", true)
	if _, err := os.Stat(filepath.Join(r.root, "u.txt")); !os.IsNotExist(err) {
		t.Fatal("untracked file was not removed")
	}
	list = r.stashes()
	if len(list) != 2 || list[0].ID != id2 || list[0].Message != "my work" || list[1].ID != id {
		t.Fatalf("list %+v", list)
	}
	info = r.detail(id2)
	fm = fileMap(info)
	if len(info.Files) != 3 || fm["new.txt"].Change != "added" || fm["u.txt"].Change != "added" || fm["README.md"].Change != "modified" {
		t.Fatalf("stash2 files %+v", info.Files)
	}
	d, err := r.s.CommitDiff(bg, "p1", "", id2, "u.txt")
	r.must(err)
	if len(d.Hunks) != 1 || len(d.Hunks[0].Lines) != 2 || d.Hunks[0].Lines[1][1] != "u2" {
		t.Fatalf("untracked stash diff %+v", d)
	}
	_, err = r.s.StashSave(bg, "p1", "", string(make([]byte, 1001)), false)
	r.wantCode(err, "BAD_REQUEST")
}

func TestStashApplyPopDrop(t *testing.T) {
	r := newRig(t, opts{})
	r.write("README.md", "stashed\n")
	a := r.save("a", false)
	r.write("README.md", "second\n")
	b := r.save("b", false)
	// Another stash shifts the indexes; the first is still found by id.
	if list := r.stashes(); list[0].ID != b || list[1].ID != a || list[1].Index != 1 {
		t.Fatalf("list %+v", list)
	}
	r.must(r.s.StashApply(bg, "p1", "", a))
	if r.read("README.md") != "stashed\n" || len(r.stashes()) != 2 {
		t.Fatal("apply must keep the stash")
	}
	r.git(r.root, "checkout", "--", "README.md")
	r.must(r.s.StashDrop(bg, "p1", "", a))
	if list := r.stashes(); len(list) != 1 || list[0].ID != b {
		t.Fatalf("after drop %+v", list)
	}
	r.wantCode(r.s.StashDrop(bg, "p1", "", a), "NOT_FOUND")
	r.wantCode(r.s.StashApply(bg, "p1", "", "abc"), "BAD_REQUEST")
	r.must(r.s.StashPop(bg, "p1", "", b))
	if r.read("README.md") != "second\n" || len(r.stashes()) != 0 {
		t.Fatal("pop did not apply and drop")
	}
}

func TestStashConflict(t *testing.T) {
	r := newRig(t, opts{})
	r.write("README.md", "from stash\n")
	id := r.save("c", false)
	r.write("README.md", "from commit\n")
	r.commitAll("other change")
	err := r.s.StashApply(bg, "p1", "", id)
	r.wantCode(err, "CONFLICTED")
	if !r.status("").Conflicted || len(r.stashes()) != 1 {
		t.Fatal("conflict state or stash lost")
	}
	r.git(r.root, "checkout", "-q", "--ours", "README.md")
	r.git(r.root, "reset", "-q", "--hard")
	err = r.s.StashPop(bg, "p1", "", id)
	r.wantCode(err, "CONFLICTED")
	if len(r.stashes()) != 1 || r.stashes()[0].ID != id {
		t.Fatal("pop dropped the stash after a conflict")
	}
	// Local changes in the way: git refuses and the stash stays.
	r.git(r.root, "reset", "-q", "--hard")
	r.write("README.md", "local edit\n")
	err = r.s.StashApply(bg, "p1", "", id)
	if c := code(err); c != "GIT_FAILED" && c != "CONFLICTED" {
		t.Fatalf("code %q", c)
	}
	if len(r.stashes()) != 1 {
		t.Fatal("stash lost")
	}
}

func TestStashBusy(t *testing.T) {
	r := newRig(t, opts{})
	r.write("README.md", "x\n")
	id := r.save("", false)
	r.chats[r.root] = ChatUse{ID: "c", Live: true}
	r.write("README.md", "y\n")
	_, err := r.s.StashSave(bg, "p1", "", "", false)
	r.wantCode(err, "BUSY")
	for _, f := range []func(context_ string) error{
		func(id string) error { return r.s.StashApply(bg, "p1", "", id) },
		func(id string) error { return r.s.StashPop(bg, "p1", "", id) },
		func(id string) error { return r.s.StashDrop(bg, "p1", "", id) },
	} {
		r.wantCode(f(id), "BUSY")
	}
	if len(r.stashes()) != 1 {
		t.Fatal("stash changed while busy")
	}
}

func TestStashSubfolderScope(t *testing.T) {
	r := newRig(t, opts{subfolder: "app"})
	r.write("README.md", "outside\n")
	r.write("app/inner.txt", "inside\n")
	if _, err := r.s.StashSave(bg, "p1", "", "", false); err != nil {
		t.Fatal(err)
	}
	if r.read("README.md") != "outside\n" || r.read("app/inner.txt") != "inner\n" {
		t.Fatal("stash from a subfolder place must only take that folder")
	}
}
