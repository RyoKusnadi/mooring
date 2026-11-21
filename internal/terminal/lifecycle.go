package terminal

import "time"

// sweep forgets terminals that exited more than the keep time ago.
func (m *Manager) sweep() {
	m.mu.Lock()
	var old []*Terminal
	for id, t := range m.terms {
		t.mu.Lock()
		if t.exited && m.now().Sub(t.exitedAt) >= m.keep {
			old = append(old, t)
			delete(m.terms, id)
		}
		t.mu.Unlock()
	}
	m.mu.Unlock()
	for _, t := range old {
		m.discard(t)
	}
}

// Snapshot lists terminals by project for the events snapshot.
func (m *Manager) Snapshot() map[string][]Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string][]Info{}
	for _, t := range m.terms {
		out[t.projectID] = append(out[t.projectID], t.info())
	}
	return out
}

// Shutdown hangs up every shell and waits briefly for the processes to end.
// Terminals are not restarted by the next server run.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	close(m.stop)
	all := make([]*Terminal, 0, len(m.terms))
	for _, t := range m.terms {
		all = append(all, t)
	}
	m.terms = map[string]*Terminal{}
	m.mu.Unlock()
	for _, t := range all {
		t.mu.Lock()
		t.removed = true
		t.mu.Unlock()
		t.kill()
	}
	deadline := time.After(shutdownWait)
	for _, t := range all {
		select {
		case <-t.done:
		case <-deadline:
			t.signal(true)
			m.log.Warn("terminal: still running at shutdown", "id", t.id)
			return
		}
	}
}
