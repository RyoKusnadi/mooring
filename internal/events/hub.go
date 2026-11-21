package events

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

const (
	// BufferSize is how many recent events a reconnecting client can replay.
	BufferSize = 1024
	// QueueSize is how many events may wait for one slow socket.
	QueueSize = 256
)

// Event is one numbered event.
type Event struct {
	Pos  uint64          `json:"pos"`
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

// Hub numbers events and fans them out to connected sockets.
type Hub struct {
	log *slog.Logger

	mu     sync.Mutex
	stream *Stream[Event]
	subs   map[*Subscriber]struct{}
}

// NewHub returns a hub with a fresh random epoch.
func NewHub(log *slog.Logger) (*Hub, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("make epoch: %w", err)
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Hub{
		log:    log,
		stream: NewStream[Event](hex.EncodeToString(b), BufferSize),
		subs:   map[*Subscriber]struct{}{},
	}, nil
}

// Subscriber receives live events through Queue. If it falls QueueSize events
// behind it is dropped: Dropped is closed and nothing more is queued.
type Subscriber struct {
	Queue   chan Event
	Dropped chan struct{}
	hub     *Hub
	dropped bool
}

// Publish records an event and queues it for every subscriber. It never blocks.
func (h *Hub) Publish(kind string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		h.log.Error("events: encode event", "kind", kind, "err", err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	ev := Event{Pos: h.stream.Pos() + 1, Kind: kind, Data: raw}
	h.stream.Append(ev)
	for s := range h.subs {
		select {
		case s.Queue <- ev:
		default:
			h.drop(s)
		}
	}
}

func (h *Hub) drop(s *Subscriber) {
	if s.dropped {
		return
	}
	s.dropped = true
	delete(h.subs, s)
	close(s.Dropped)
	h.log.Warn("events: slow socket dropped")
}

// Attachment is what a newly connected client needs before live events.
type Attachment struct {
	Epoch    string
	Pos      uint64
	Snapshot any     // nil when the client resumes
	Replay   []Event // events after the client's pos, when it resumes
	Sub      *Subscriber
}

// Attach decides resume or snapshot and subscribes the client, atomically with
// respect to Publish: nothing is missed or sent twice by the hub. snapshot is
// called only when the client must reset, with publishing paused.
func (h *Hub) Attach(epoch string, pos uint64, snapshot func() (any, error)) (*Attachment, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	a := &Attachment{Epoch: h.stream.Epoch(), Pos: h.stream.Pos()}
	if replay, ok := h.stream.Resume(epoch, pos); ok {
		a.Replay = replay
	} else {
		snap, err := snapshot()
		if err != nil {
			return nil, err
		}
		a.Snapshot = snap
	}
	a.Sub = &Subscriber{
		Queue: make(chan Event, QueueSize), Dropped: make(chan struct{}), hub: h,
	}
	h.subs[a.Sub] = struct{}{}
	return a, nil
}

// Detach stops delivery to s.
func (h *Hub) Detach(s *Subscriber) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
}
