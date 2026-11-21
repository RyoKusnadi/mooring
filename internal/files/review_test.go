package files

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// Watching the project folder as a file must not watch the folder above it.
func TestWatchProjectFolderAsFileIsSkipped(t *testing.T) {
	r := newRig(t, nil)
	res, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "w1", Files: []string{"", "."}})
	if err != nil || res.Files != 0 || len(res.Skipped) != 2 || res.Skipped[0].Code != "IS_A_FOLDER" {
		t.Fatalf("watch: %+v %v", res, err)
	}
	r.s.w.mu.Lock()
	defer r.s.w.mu.Unlock()
	for d := range r.s.w.refs {
		if d == filepath.Dir(r.root) || !filepath.IsAbs(d) {
			t.Fatalf("watches %s, outside the project", d)
		}
	}
}

// A watched file costs its folder's entries on kqueue, like a watched folder.
func TestWatchFileInHugeFolderIsSkipped(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("inotify costs one watch per folder")
	}
	r := newRig(t, nil)
	big := filepath.Join(r.root, "big")
	if err := os.MkdirAll(big, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range maxFolderHandles + 1 {
		if err := os.WriteFile(filepath.Join(big, "f"+itoa(i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "w1", Files: []string{"big/f1"}})
	if err != nil || res.Files != 0 || len(res.Skipped) != 1 || res.Skipped[0].Code != "LIMIT_REACHED" {
		t.Fatalf("watch: %+v %v", res, err)
	}
}

// Many mid-sized folders together must stay under the handle budget.
func TestWatchHandleBudget(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("inotify costs one watch per folder")
	}
	r := newRig(t, nil)
	var dirs []string
	for d := range maxWatchHandles/maxFolderHandles + 1 {
		dir := filepath.Join(r.root, "d"+itoa(d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := range maxFolderHandles {
			if err := os.WriteFile(filepath.Join(dir, "f"+itoa(i)), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		dirs = append(dirs, "d"+itoa(d))
	}
	_, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "w1", Dirs: dirs})
	r.wantCode(err, "LIMIT_REACHED")
}

// A FIFO in the project must be refused, not block the read forever.
func TestReadFIFOIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no FIFOs")
	}
	r := newRig(t, nil)
	if err := syscall.Mkfifo(filepath.Join(r.root, "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := r.s.Read("p1", "pipe", 0, 0, "")
		done <- err
	}()
	select {
	case err := <-done:
		r.wantCode(err, "NOT_A_FILE")
	case <-time.After(3 * time.Second):
		t.Fatal("reading a FIFO blocked")
	}
	if _, err := ReadWhole(filepath.Join(r.root, "pipe"), 10); err != ErrNotRegular {
		t.Fatalf("ReadWhole: %v", err)
	}
}
