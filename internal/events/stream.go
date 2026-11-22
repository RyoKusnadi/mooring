// Package events carries commands, results and events over one WebSocket.
package events

// Stream numbers items and keeps the most recent ones so a client that
// reconnects can be caught up. Terminals reuse it with bytes instead of events. It is not safe for concurrent use; the owner locks.
type Stream[T any] struct {
	epoch string
	pos   uint64
	buf   []T // ring; item number n lives at (n-1) % len(buf)
}

// NewStream returns a stream that keeps the last size items.
func NewStream[T any](epoch string, size int) *Stream[T] {
	return &Stream[T]{epoch: epoch, buf: make([]T, size)}
}

// Epoch identifies this run of the stream. Positions mean nothing across epochs.
func (s *Stream[T]) Epoch() string { return s.epoch }

// Pos is the number of the latest item; 0 before the first.
func (s *Stream[T]) Pos() uint64 { return s.pos }

// Append adds an item and returns its position.
func (s *Stream[T]) Append(v T) uint64 {
	s.pos++
	s.buf[(s.pos-1)%uint64(len(s.buf))] = v
	return s.pos
}

// Resume decides how to catch up a client that last saw (epoch, pos). When
// the epoch matches and pos is neither ahead nor older than the buffer, it
// returns the items after pos and true. Otherwise the client must reset from
// a snapshot, and it returns false.
func (s *Stream[T]) Resume(epoch string, pos uint64) ([]T, bool) {
	size := uint64(len(s.buf))
	if epoch != s.epoch || pos > s.pos || s.pos-pos > size {
		return nil, false
	}
	out := make([]T, 0, s.pos-pos)
	for n := pos + 1; n <= s.pos; n++ {
		out = append(out, s.buf[(n-1)%size])
	}
	return out, true
}

// Oldest is the position just before the oldest item still held: a client at
// this position can resume without a reset.
func (s *Stream[T]) Oldest() uint64 {
	if size := uint64(len(s.buf)); s.pos > size {
		return s.pos - size
	}
	return 0
}

// AppendSlice adds items in order and returns the ones pushed out of the buffer.
func (s *Stream[T]) AppendSlice(vs []T) (evicted []T) {
	size := uint64(len(s.buf))
	for len(vs) > 0 {
		n := min(uint64(len(vs)), size)
		if over := s.pos + n; over > size {
			// Items numbered (pos-size, over-size] leave; they are the oldest held.
			from := max(s.pos, size) - size
			for k := from + 1; k <= over-size; k++ {
				if k <= s.pos {
					evicted = append(evicted, s.buf[(k-1)%size])
				}
			}
		}
		for _, v := range vs[:n] {
			s.pos++
			s.buf[(s.pos-1)%size] = v
		}
		vs = vs[n:]
	}
	return evicted
}

// Since returns up to limit items after pos, or false when pos is older than the
// buffer or ahead of the stream.
func (s *Stream[T]) Since(pos uint64, limit int) ([]T, bool) {
	if pos > s.pos || pos < s.Oldest() {
		return nil, false
	}
	size := uint64(len(s.buf))
	n := min(s.pos-pos, uint64(limit))
	out := make([]T, 0, n)
	for k := pos + 1; k <= pos+n; k++ {
		out = append(out, s.buf[(k-1)%size])
	}
	return out, true
}
