package accounts

import (
	"testing"
	"time"
)

// Snapshot must not hold a connection while it asks for another: with the pool
// used up, every request waited forever (four browsers signing in at once did it).
func TestSnapshotNeedsOneConnectionAtATime(t *testing.T) {
	r := newRig(t)
	r.s.DB.SetMaxOpenConns(1)
	done := make(chan error, 1)
	go func() {
		_, err := r.s.Snapshot()
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Snapshot waited for a second connection")
	}
}
