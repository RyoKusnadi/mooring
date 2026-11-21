package terminal

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ryokusnadi/mooring/internal/events"
)

const (
	chunkSize  = 16 << 10 // one PTY read
	batchDelay = 4 * time.Millisecond
	maxBatch   = 64 << 10
	drainWait  = 500 * time.Millisecond // output grace after the shell exits
)

// Info is what the browser is told about a terminal.
type Info struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
	Exited    bool   `json:"exited"`
	Code      int    `json:"code"`
	SSH       string `json:"ssh,omitempty"` // the connection a remote shell runs on

	Attention bool `json:"attention,omitempty"` // a signal is waiting for the user
	Bell      bool `json:"bell,omitempty"`      // the signal is a bell alone
}

// Terminal is one shell on a PTY, its output ring and the clients watching it.
type Terminal struct {
	id, projectID string
	ssh           string // connection id of a remote shell, "" for a local one
	m             *Manager
	log           *slog.Logger
	b             Backend
	readDone      chan struct{} // the PTY reached end of output and the last batch is in the ring
	done          chan struct{} // closed once the terminal is marked exited

	mu       sync.Mutex
	flow     *sync.Cond // the PTY reader waits here while the active client is behind
	stream   *events.Stream[byte]
	cur      *tracker // modes at the end of the ring
	head     *tracker // modes at the start of the ring
	cols     uint16
	rows     uint16
	clients  []*client // by recency; the last one is the active client
	owner    *client   // last to type or resize; gates flow control
	reaped   bool      // the shell was waited for
	exited   bool
	code     int
	exitedAt time.Time
	closing  bool
	removed  bool // closed by the user or by shutdown: no exited event

	sig      signalState // Attention from live output
	startDir string      // folder the shell was started in; "" for a remote shell
}

// newTerminal runs the readers and the waiter on a started backend.
func newTerminal(m *Manager, id, projectID, ssh string, b Backend, cols, rows uint16) (*Terminal, error) {
	eb := make([]byte, 8)
	if _, err := rand.Read(eb); err != nil {
		b.Kill()
		_ = b.Close()
		return nil, fmt.Errorf("make epoch: %w", err)
	}
	t := &Terminal{
		id: id, projectID: projectID, ssh: ssh, m: m, b: b, cols: cols, rows: rows,
		log:      m.log.With("terminal", id),
		readDone: make(chan struct{}), done: make(chan struct{}),
		stream: events.NewStream[byte](hex.EncodeToString(eb), m.ringSize),
		cur:    newTracker(), head: newTracker(),
	}
	t.flow = sync.NewCond(&t.mu)
	go t.readLoop()
	go t.run()
	return t, nil
}

func (t *Terminal) info() Info {
	t.mu.Lock()
	defer t.mu.Unlock()
	return Info{ID: t.id, ProjectID: t.projectID, Cols: t.cols, Rows: t.rows, Exited: t.exited, Code: t.code, SSH: t.ssh, Attention: t.sig.on, Bell: t.sig.on && t.sig.bell}
}

// readLoop moves PTY output into batches. It stops reading while the active
// client is too far behind, so the shell blocks on its own writes.
func (t *Terminal) readLoop() {
	defer close(t.readDone)
	chunks := make(chan []byte, 8)
	batched := make(chan struct{})
	go func() {
		defer close(batched)
		t.batch(chunks)
	}()
	for {
		t.waitFlow()
		buf := make([]byte, chunkSize)
		n, err := t.b.Read(buf)
		if n > 0 {
			chunks <- buf[:n]
		}
		if err != nil {
			break
		}
	}
	close(chunks)
	<-batched
}

func (t *Terminal) waitFlow() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for !t.closing && !t.reaped && t.pausedLocked() {
		t.flow.Wait()
	}
}

// pausedLocked: the active client has not drawn what it was sent, by more than the window.
func (t *Terminal) pausedLocked() bool {
	if len(t.clients) == 0 {
		return false
	}
	gate := t.clients[len(t.clients)-1]
	if t.owner != nil {
		gate = t.owner
	}
	return t.stream.Pos()-gate.acked > uint64(t.m.window)
}

// batch collects chunks for about batchDelay, then adds them to the ring as one write.
func (t *Terminal) batch(chunks <-chan []byte) {
	var pending []byte
	for first := range chunks {
		pending = append(pending[:0], first...)
		timer := time.NewTimer(batchDelay)
	collect:
		for len(pending) < maxBatch {
			select {
			case c, ok := <-chunks:
				if !ok {
					break collect
				}
				pending = append(pending, c...)
			case <-timer.C:
				break collect
			}
		}
		timer.Stop()
		t.publish(pending)
	}
}

func (t *Terminal) publish(data []byte) {
	t.mu.Lock()
	evicted := t.stream.AppendSlice(data)
	t.cur.feed(data)
	t.head.feed(evicted)
	changed := t.sig.raise(data, t.m.now())
	t.wakeLocked()
	t.mu.Unlock()
	if changed {
		t.announce()
	}
}

func (t *Terminal) wakeLocked() {
	for _, c := range t.clients {
		c.poke()
	}
}

// run waits for the shell, lets its last output drain, then marks the terminal exited.
func (t *Terminal) run() {
	code := t.b.Wait()
	t.mu.Lock()
	t.reaped = true
	t.flow.Broadcast() // the shell is gone: read what is left, whoever is behind
	t.mu.Unlock()
	select {
	case <-t.readDone:
	case <-time.After(drainWait):
		// Something still holds the terminal open: end what is left in the group.
		t.b.Kill()
		select {
		case <-t.readDone:
		case <-time.After(drainWait):
		}
	}
	_ = t.b.Close()
	t.markExited(code)
}

func (t *Terminal) markExited(code int) {
	t.mu.Lock()
	t.exited, t.code, t.exitedAt = true, code, t.m.now()
	removed := t.removed
	info := Info{ID: t.id, ProjectID: t.projectID, Cols: t.cols, Rows: t.rows, Exited: true, Code: code, SSH: t.ssh}
	t.wakeLocked()
	t.mu.Unlock()
	t.log.Info("terminal: exited", "code", code)
	if !removed {
		t.m.pub.Publish("terminal.exited", map[string]any{"id": info.ID, "projectId": info.ProjectID, "code": code})
	}
	close(t.done)
}
