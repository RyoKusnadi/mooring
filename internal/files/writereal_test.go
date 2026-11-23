package files

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// WriteReal waits for the lock an editor save holds, so an agent's write cannot
// land between the save's version check and its rename.
func TestWriteRealWaitsForTheSaveLock(t *testing.T) {
	r := newRig(t, nil)
	r.write("a.txt", "one\n")
	real, err := filepath.EvalSymlinks(filepath.Join(r.root, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	unlock := r.s.lock(real)
	done := make(chan error, 1)
	go func() { done <- r.s.WriteReal(real, []byte("agent\n")) }()
	select {
	case err := <-done:
		t.Fatalf("wrote while the save lock was held: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(real); string(b) != "agent\n" {
		t.Fatalf("file: %q", b)
	}
}
