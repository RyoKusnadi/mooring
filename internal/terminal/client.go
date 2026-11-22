package terminal

import (
	"slices"
)

const maxFrame = 64 << 10 // output bytes in one frame

// stateFrame is the hello frame and the reset frame (T tells which).
type stateFrame struct {
	T      string `json:"t"`
	Epoch  string `json:"epoch,omitempty"`
	Pos    uint64 `json:"pos"`
	Reset  bool   `json:"reset"`
	Modes  string `json:"modes"`
	Cols   uint16 `json:"cols"`
	Rows   uint16 `json:"rows"`
	Others int    `json:"others"`
	Exited bool   `json:"exited"`
	Code   int    `json:"code"`
}

type sizeFrame struct {
	T    string `json:"t"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

type exitFrame struct {
	T    string `json:"t"`
	Code int    `json:"code"`
}

// client is one socket watching a terminal. The terminal's lock guards its fields.
type client struct {
	t         *Terminal
	wake      chan struct{} // cap 1: something to send
	cursor    uint64        // next position to send
	acked     uint64        // drawn by the browser
	sizeDirty bool
	exitSent  bool
}

func (c *client) poke() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// attach registers a client and says where it starts. A client at a position
// the ring still holds resumes; any other starts from a reset.
func (t *Terminal) attach(epoch string, pos uint64) (*client, stateFrame) {
	t.mu.Lock()
	c := &client{t: t, wake: make(chan struct{}, 1)}
	f := stateFrame{T: "hello", Epoch: t.stream.Epoch(), Others: len(t.clients)}
	reset := epoch != t.stream.Epoch() || pos < t.stream.Oldest() || pos > t.stream.Pos()
	if reset {
		pos, f.Modes = t.resetPointLocked()
		f.Reset = true
	}
	c.cursor, c.acked, f.Pos = pos, pos, pos
	t.fillLocked(&f)
	t.clients = append(t.clients, c)
	repaint := reset && t.cur.alt() && !t.exited
	t.mu.Unlock()
	if repaint {
		t.repaint()
	}
	return c, f
}

func (t *Terminal) fillLocked(f *stateFrame) {
	f.Cols, f.Rows, f.Exited, f.Code = t.cols, t.rows, t.exited, t.code
}

// resetPointLocked is where a replay starts and the modes to set before it.
func (t *Terminal) resetPointLocked() (uint64, string) {
	oldest := t.stream.Oldest()
	first, _ := t.stream.Since(oldest, 4096)
	skip, tr := safeStart(t.head, first)
	return oldest + uint64(skip), tr.preamble()
}

func (t *Terminal) detach(c *client) {
	t.mu.Lock()
	t.clients = slices.DeleteFunc(t.clients, func(x *client) bool { return x == c })
	if t.owner == c {
		t.owner = nil
	}
	t.flow.Broadcast() // the gating client may have changed
	t.mu.Unlock()
}

// touchLocked makes c the active client.
func (t *Terminal) touchLocked(c *client) {
	if t.owner != c {
		t.owner = c
		t.flow.Broadcast()
	}
	if n := len(t.clients); n > 0 && t.clients[n-1] != c {
		t.clients = append(slices.DeleteFunc(t.clients, func(x *client) bool { return x == c }), c)
		t.flow.Broadcast()
	}
}

func (t *Terminal) ack(c *client, pos uint64) {
	t.mu.Lock()
	if pos > c.acked && pos <= c.cursor {
		c.acked = pos
		t.flow.Broadcast()
	}
	t.mu.Unlock()
}

// input writes the client's keystrokes to the shell and makes it the active client.
func (t *Terminal) input(c *client, data []byte) {
	t.touch(c)
	t.writeInput(data)
}

// touch records that c typed.
func (t *Terminal) touch(c *client) {
	t.mu.Lock()
	t.touchLocked(c)
	t.mu.Unlock()
}

// writeInput can block while the shell is not reading; callers that also read
// acks must not call it from their read loop.
func (t *Terminal) writeInput(data []byte) {
	t.mu.Lock()
	exited := t.exited
	t.mu.Unlock()
	if exited {
		return
	}
	if _, err := t.b.Write(data); err != nil {
		t.log.Warn("terminal: write input", "err", err)
	}
}

// resize sets the PTY size from this client's fitted size. Every client is
// told, the asking one included.
func (t *Terminal) resize(c *client, cols, rows uint16) {
	if cols < minCols || cols > maxCols || rows < 1 || rows > maxRows {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.exited {
		return
	}
	t.touchLocked(c)
	c.sizeDirty = true
	if cols != t.cols || rows != t.rows {
		if err := t.b.Resize(cols, rows); err != nil {
			t.log.Warn("terminal: resize", "err", err)
			return
		}
		t.cols, t.rows = cols, rows
		for _, o := range t.clients {
			o.sizeDirty = true
		}
	}
	t.wakeLocked()
}

// work is the next thing to send to a client; wait means nothing yet.
type work struct {
	state *stateFrame
	size  *sizeFrame
	data  []byte
	pos   uint64
	exit  *exitFrame
	wait  bool
}

func (c *client) next() work {
	t := c.t
	t.mu.Lock()
	defer t.mu.Unlock()
	if c.cursor < t.stream.Oldest() {
		// Fell behind the ring: start over from what it holds.
		f := stateFrame{T: "reset", Reset: true}
		c.cursor, f.Modes = t.resetPointLocked()
		f.Pos = c.cursor
		c.acked = max(c.acked, c.cursor)
		t.fillLocked(&f)
		if t.cur.alt() && !t.exited {
			t.repaint()
		}
		return work{state: &f}
	}
	if c.sizeDirty {
		c.sizeDirty = false
		return work{size: &sizeFrame{T: "size", Cols: t.cols, Rows: t.rows}}
	}
	if c.cursor < t.stream.Pos() {
		data, _ := t.stream.Since(c.cursor, maxFrame)
		pos := c.cursor
		c.cursor += uint64(len(data))
		return work{data: data, pos: pos}
	}
	if t.exited && !c.exitSent {
		c.exitSent = true
		return work{exit: &exitFrame{T: "exit", Code: t.code}}
	}
	return work{wait: true}
}
