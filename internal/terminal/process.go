package terminal

import "time"

// kill hangs up the shell, then kills it if it lingers.
func (t *Terminal) kill() {
	t.mu.Lock()
	t.closing = true
	t.flow.Broadcast()
	exited := t.exited
	t.mu.Unlock()
	if exited {
		return
	}
	t.signal(false)
	go func() {
		select {
		case <-t.done:
		case <-time.After(t.m.killGrace):
			t.signal(true)
		}
	}()
}

// signal hangs up the shell, or kills it when force is set. It does nothing
// once the shell has been reaped.
func (t *Terminal) signal(force bool) {
	t.mu.Lock()
	reaped := t.reaped
	t.mu.Unlock()
	if reaped {
		return
	}
	if force {
		t.b.Kill()
	} else {
		t.b.Hangup()
	}
}

// repaint asks a full-screen program to redraw: SIGWINCH to the foreground
// group, and when nothing is written soon, a one-row size nudge on the PTY
// alone (clients are not told).
func (t *Terminal) repaint() {
	go func() {
		t.mu.Lock()
		if t.exited {
			t.mu.Unlock()
			return
		}
		before := t.stream.Pos()
		t.mu.Unlock()
		t.b.Winch()
		select {
		case <-time.After(t.m.repaintWait):
		case <-t.done:
			return
		}
		t.mu.Lock()
		moved, cols, rows := t.stream.Pos() != before, t.cols, t.rows
		t.mu.Unlock()
		if moved || rows < 2 {
			return
		}
		_ = t.b.Resize(cols, rows-1)
		time.Sleep(20 * time.Millisecond)
		t.mu.Lock()
		defer t.mu.Unlock()
		if !t.exited {
			_ = t.b.Resize(t.cols, t.rows) // the real size, even if someone resized meanwhile
		}
	}()
}
