package files

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func isFile(path, version string) func(published) bool {
	return func(p published) bool {
		var e fileEvent
		return json.Unmarshal(p.Data, &e) == nil && e.Path == path && e.Version == version
	}
}

func TestWatchSeesExternalWriteAndRenameSave(t *testing.T) {
	r := newRig(t, nil)
	r.write("a.txt", "one\n")
	res, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "w1", Dirs: []string{""}, Files: []string{"a.txt"}})
	if err != nil || res.Dirs != 1 || res.Files != 1 {
		t.Fatalf("watch: %+v %v", res, err)
	}
	// Another program (an agent) writes in place.
	r.write("a.txt", "two\n")
	r.rec.wait(t, "file.changed", isFile("a.txt", Version([]byte("two\n"))))
	r.rec.wait(t, "file.dir", func(p published) bool { return string(p.Data) == `{"path":"","projectId":"p1"}` })

	// An editor saves through a temporary file renamed over the target.
	tmp := filepath.Join(r.root, "a.txt.swp")
	if err := os.WriteFile(tmp, []byte("three\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(r.root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	r.rec.wait(t, "file.changed", isFile("a.txt", Version([]byte("three\n"))))

	// Deleted.
	if err := os.Remove(filepath.Join(r.root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	r.rec.wait(t, "file.changed", func(p published) bool {
		var e fileEvent
		return json.Unmarshal(p.Data, &e) == nil && e.Deleted && e.Path == "a.txt"
	})
}

func TestWatchOwnSaveIsOneEventAndNoTempNames(t *testing.T) {
	r := newRig(t, nil)
	r.write("a.txt", "one\n")
	if _, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "w1", Dirs: []string{""}, Files: []string{"a.txt"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.s.Write("p1", "a.txt", Version([]byte("one\n")), []byte("saved\n")); err != nil {
		t.Fatal(err)
	}
	r.rec.wait(t, "file.changed", isFile("a.txt", Version([]byte("saved\n"))))
	time.Sleep(150 * time.Millisecond)
	if n := len(r.rec.all("file.changed")); n != 1 {
		t.Fatalf("%d file.changed events for one save", n)
	}
}

func TestWatchBurstIsBatched(t *testing.T) {
	r := newRig(t, func(c *Config) { c.Debounce = 100 * time.Millisecond })
	if _, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "w1", Dirs: []string{""}}); err != nil {
		t.Fatal(err)
	}
	for i := range 50 {
		r.write("f"+itoa(i)+".txt", "x")
	}
	r.rec.wait(t, "file.dir", func(published) bool { return true })
	time.Sleep(300 * time.Millisecond)
	if n := len(r.rec.all("file.dir")); n > 3 {
		t.Fatalf("%d file.dir events for one burst", n)
	}
}

func TestWatchLimitsAndRelease(t *testing.T) {
	r := newRig(t, nil)
	for i := range maxWatchDirs + 1 {
		if err := os.MkdirAll(filepath.Join(r.root, "d"+itoa(i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var dirs []string
	for i := range maxHolderSet {
		dirs = append(dirs, "d"+itoa(i))
	}
	if _, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "h1", Dirs: dirs}); err != nil {
		t.Fatal(err)
	}
	var more []string
	for i := maxHolderSet; i < 2*maxHolderSet; i++ {
		more = append(more, "d"+itoa(i))
	}
	if _, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "h2", Dirs: more}); err != nil {
		t.Fatal(err)
	}
	var third []string
	for i := 2 * maxHolderSet; i <= maxWatchDirs; i++ {
		third = append(third, "d"+itoa(i))
	}
	_, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "h3", Dirs: third})
	r.wantCode(err, "LIMIT_REACHED")
	_, err = r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "h4", Dirs: append(dirs, more[0])})
	r.wantCode(err, "BAD_REQUEST")
	// Releasing a holder frees its folders.
	if _, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "h1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "h3", Dirs: third}); err != nil {
		t.Fatal(err)
	}
	// Missing folder is skipped, not fatal.
	res, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "h5", Dirs: []string{"nope"}})
	if err != nil || len(res.Skipped) != 1 || res.Skipped[0].Code != "NOT_FOUND" {
		t.Fatalf("skip: %+v %v", res, err)
	}
}

func TestWatchLeaseExpiresAndProjectCloses(t *testing.T) {
	r := newRig(t, func(c *Config) { c.Lease = 80 * time.Millisecond })
	if _, err := r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "w1", Dirs: []string{""}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		r.s.w.mu.Lock()
		n := len(r.s.w.holders) + len(r.s.w.refs)
		r.s.w.mu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("holder was not dropped after its lease")
		}
		time.Sleep(20 * time.Millisecond)
	}
	r2 := newRig(t, nil)
	if _, err := r2.s.Watch(WatchArgs{ProjectID: "p1", Holder: "w1", Dirs: []string{""}}); err != nil {
		t.Fatal(err)
	}
	r2.s.CloseProject("p1")
	r2.write("late.txt", "x")
	time.Sleep(150 * time.Millisecond)
	if n := len(r2.rec.all("file.dir")); n != 0 {
		t.Fatalf("%d events after the project closed", n)
	}
}
