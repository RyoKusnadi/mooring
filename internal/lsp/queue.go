package lsp

import "sync"

// queue is an unbounded list of messages for a server's stdin. A writer that
// waits on a full pipe must never stop the reader from draining stdout, so
// nothing writes to stdin directly.
type queue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  [][]byte
	closed bool
}

func newQueue() *queue {
	q := &queue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *queue) push(b []byte) {
	q.mu.Lock()
	if !q.closed {
		q.items = append(q.items, b)
		q.cond.Signal()
	}
	q.mu.Unlock()
}

// close lets pop return what is queued, then false.
func (q *queue) close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

func (q *queue) pop() ([]byte, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return nil, false
	}
	b := q.items[0]
	q.items[0] = nil
	q.items = q.items[1:]
	return b, true
}
