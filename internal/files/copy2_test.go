package files

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyOfATooBigFolderFailsBeforeAnythingIsMade(t *testing.T) {
	r := newRig(t, nil)
	old := maxCopyEntries
	maxCopyEntries = 5
	t.Cleanup(func() { maxCopyEntries = old })
	for i := 0; i < 6; i++ {
		r.write(fmt.Sprintf("big/f%d", i), "x")
	}
	_, err := r.s.Copy("p1", "big", "big copy")
	r.wantCode(err, "FILE_TOO_LARGE")
	if _, err := os.Lstat(filepath.Join(r.root, "big copy")); err == nil {
		t.Fatal("a half-made copy was left")
	}
	maxCopyEntries = old
	maxCopyDepth = 2
	t.Cleanup(func() { maxCopyDepth = 64 })
	r.write("deep/a/b/c/d.txt", "x")
	_, err = r.s.Copy("p1", "deep", "deep copy")
	r.wantCode(err, "FILE_TOO_LARGE")
	maxCopyDepth = 64
	maxCopyBytes = 3
	t.Cleanup(func() { maxCopyBytes = 1 << 30 })
	r.write("fat/a", "abcd")
	_, err = r.s.Copy("p1", "fat", "fat copy")
	r.wantCode(err, "FILE_TOO_LARGE")
}

func TestCopyListsOnlyTheFirstSkippedItemsAndCountsAll(t *testing.T) {
	r := newRig(t, nil)
	r.write("d/a.txt", "x")
	for i := 0; i < 30; i++ {
		if err := os.Symlink("a.txt", filepath.Join(r.root, "d", fmt.Sprintf("l%02d", i))); err != nil {
			t.Fatal(err)
		}
	}
	res, err := r.s.Copy("p1", "d", "d2")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) != maxSkippedList || res.SkippedCount != 30 {
		t.Fatalf("listed %d, counted %d", len(res.Skipped), res.SkippedCount)
	}
}

func TestCopyKeepsFolderModesAndLeavesAnExistingTargetAlone(t *testing.T) {
	r := newRig(t, nil)
	r.write("d/sub/a.txt", "x")
	if err := os.Chmod(filepath.Join(r.root, "d", "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := r.s.Copy("p1", "d", "d2"); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(filepath.Join(r.root, "d2", "sub")); fi.Mode().Perm() != 0o750 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
	r.write("keep/mine.txt", "mine")
	_, err := r.s.Copy("p1", "d", "keep")
	r.wantCode(err, "ALREADY_EXISTS")
	if b, _ := os.ReadFile(filepath.Join(r.root, "keep", "mine.txt")); string(b) != "mine" {
		t.Fatal("an existing folder was touched")
	}
}

func TestCopyIntoItselfThroughALinkedParentIsRefused(t *testing.T) {
	r := newRig(t, nil)
	r.write("d/a.txt", "x")
	r.link("d", "alias")
	if _, err := r.s.Copy("p1", "d", "alias/inner"); err == nil {
		t.Fatal("copied a folder into itself")
	}
	if _, err := os.Lstat(filepath.Join(r.root, "d", "inner")); err == nil {
		t.Fatal("a copy was made inside the source")
	}
}
