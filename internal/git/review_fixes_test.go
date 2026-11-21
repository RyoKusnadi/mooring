package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwitchRefusesRevisions(t *testing.T) {
	r := newRig(t, opts{})
	r.write("a.txt", "a\n")
	r.commitAll("two")
	for _, name := range []string{"main^", "main~1", "main@{0}", "HEAD", "refs/heads/main"} {
		_, err := r.s.Switch(bg, "p1", "", name)
		r.wantCode(err, "NOT_FOUND")
	}
	if r.git(r.root, "branch", "--show-current") != "main" {
		t.Fatal("HEAD left the branch")
	}
}

func TestDiffIgnoresUserConfig(t *testing.T) {
	r := newRig(t, opts{})
	r.write("f.txt", "a\n\nb\n\nc\n")
	r.commitAll("f")
	r.write("f.txt", "a\n\nB\n\nc\n")
	want := r.diff("f.txt", false)
	cfg := filepath.Join(r.base, "gitconfig")
	conf := "[diff]\n\tsuppressBlankEmpty = true\n\tnoprefix = true\n\tmnemonicPrefix = true\n\trelative = true\n\talgorithm = patience\n[color]\n\tdiff = always\n\tui = always\n[core]\n\tquotePath = false\n"
	if err := os.WriteFile(cfg, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	r.s.cfg.Env = append(append([]string{}, r.env...), "GIT_CONFIG_GLOBAL="+cfg)
	got := r.diff("f.txt", false)
	if got.Version != want.Version || len(got.Hunks[0].Lines) != len(want.Hunks[0].Lines) {
		t.Fatalf("diff changed with user config: %+v", got)
	}
	r.must(r.stageLines("f.txt", false, got, "-b", "+B"))
	if r.blobRaw("f.txt") != "a\n\nB\n\nc\n" {
		t.Fatalf("index %q", r.blobRaw("f.txt"))
	}
}

func TestStashMessageControlChars(t *testing.T) {
	r := newRig(t, opts{})
	r.write("README.md", "x\n")
	_, err := r.s.StashSave(bg, "p1", "", "a\x1fb", false)
	r.wantCode(err, "BAD_REQUEST")
	_, err = r.s.StashSave(bg, "p1", "", "a\nb", false)
	r.wantCode(err, "BAD_REQUEST")
	// A stash made by hand with such a message still lists.
	r.git(r.root, "stash", "push", "-q", "-m", "odd\x1fmessage")
	list := r.stashes()
	if len(list) != 1 || list[0].Message != "odd\x1fmessage" {
		t.Fatalf("list %+v", list)
	}
}

func TestStashConflictOnlyFromApply(t *testing.T) {
	r := newRig(t, opts{})
	r.write("other.txt", "o\n")
	r.commitAll("other")
	r.write("README.md", "stash side\n")
	id := r.save("s", false)
	r.write("README.md", "head side\n")
	r.commitAll("head")
	// An unrelated unmerged file exists; an apply that fails without conflicts is GIT_FAILED.
	r.git(r.root, "checkout", "-q", "-b", "x", "HEAD~1")
	r.write("other.txt", "x\n")
	r.commitAll("x")
	r.git(r.root, "checkout", "-q", "main")
	r.write("other.txt", "m\n")
	r.commitAll("m")
	if _, err := gitOut(r, "merge", "x"); err == nil {
		t.Fatal("no conflict")
	}
	err := r.s.StashApply(bg, "p1", "", id)
	if c := code(err); c == "" {
		t.Fatal("apply succeeded in a merge")
	} else if c == "CONFLICTED" && len(r.unmergedPaths()) > 1 {
		t.Fatalf("unrelated conflict reported")
	}
	if c := code(err); c != "GIT_FAILED" {
		t.Fatalf("code %q (%v)", c, err)
	}
}

func (r *rig) unmergedPaths() []string {
	var p []string
	for p2 := range r.s.unmergedSet(bg, &place{top: r.root}) {
		p = append(p, p2)
	}
	return p
}

func TestLogOrderUsesTipIDs(t *testing.T) {
	r := newRig(t, opts{})
	pl, err := r.s.need("p1", "")
	r.must(err)
	st, err := r.s.readRefs(bg, pl)
	r.must(err)
	r.write("n.txt", "n\n")
	r.commitAll("moved after the refs were read")
	o, err := r.s.order(bg, pl, st)
	r.must(err)
	if o.len() != 1 {
		t.Fatalf("order has %d commits, want the 1 of the refs state it was built for", o.len())
	}
	// The cache is bounded by size.
	big := &logOrder{key: "big", w: 20, raw: make([]byte, cacheBytes+1)}
	r.s.lc.put(&logOrder{key: "small", w: 20, raw: make([]byte, 20)})
	r.s.lc.put(big)
	r.s.lc.mu.Lock()
	n := len(r.s.lc.list)
	r.s.lc.mu.Unlock()
	if n != 1 {
		t.Fatalf("cache holds %d orders over the size cap", n)
	}
}

func TestSearchStoppedTrimmed(t *testing.T) {
	list := make([]LogCommit, 4)
	got, next := trimSearch(list, 3, 10, true)
	if len(got) != 3 || next != "" {
		t.Fatalf("stopped: %d, %q", len(got), next)
	}
	got, next = trimSearch(list, 3, 10, false)
	if len(got) != 3 || next != "13" {
		t.Fatalf("whole: %d, %q", len(got), next)
	}
}
