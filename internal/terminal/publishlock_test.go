package terminal

import (
	"testing"
	"time"
)

// snapshotter takes the manager's lock inside Publish, as the events hub does
// when a page connects while an event is being published.
type snapshotter struct{ m *Manager }

func (s *snapshotter) Publish(string, any) {
	if s.m != nil {
		s.m.Snapshot()
	}
}

func TestOpenDoesNotPublishUnderTheLock(t *testing.T) {
	pub := &snapshotter{}
	dir := t.TempDir()
	m := New(Config{Pub: pub, Shell: "/bin/sh", Dir: func(string) (string, error) { return dir, nil }})
	pub.m = m
	done := make(chan error, 1)
	go func() {
		_, err := m.Open("p1", "", "t1", 80, 24)
		done <- err
	}()
	select {
	case err := <-done:
		m.Shutdown()
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		// No Shutdown here: it would wait on the lock that is stuck.
		t.Fatal("Open published an event while holding the manager's lock")
	}
}
