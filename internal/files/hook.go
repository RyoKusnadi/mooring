package files

import (
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Kinds of an FSChange, the same numbers as LSP's FileChangeType.
const (
	Created = 1
	Changed = 2
	Deleted = 3
)

// FSChange is one raw change inside a watched folder: the real path and what
// happened. It is not debounced and not tied to a holder.
type FSChange struct {
	Path string
	Kind int
}

type hooks struct {
	mu   sync.Mutex
	next int
	subs map[int]func(FSChange)
}

// Subscribe calls fn for every change inside any folder a holder watches, until
// the returned function is called. fn runs on the watcher's goroutine, so it
// must return quickly.
func (s *Service) Subscribe(fn func(FSChange)) (cancel func()) {
	h := &s.hooks
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs == nil {
		h.subs = map[int]func(FSChange){}
	}
	h.next++
	id := h.next
	h.subs[id] = fn
	return func() {
		h.mu.Lock()
		delete(h.subs, id)
		h.mu.Unlock()
	}
}

// notify hands an event in a watched folder to the subscribers. The caller no
// longer holds the watcher's lock.
func (w *watcher) notify(ev fsnotify.Event, watched bool) {
	if !watched {
		return
	}
	kind := Changed
	switch {
	case ev.Op.Has(fsnotify.Remove) || ev.Op.Has(fsnotify.Rename):
		kind = Deleted
	case ev.Op.Has(fsnotify.Create):
		kind = Created
	}
	h := &w.s.hooks
	h.mu.Lock()
	fns := make([]func(FSChange), 0, len(h.subs))
	for _, fn := range h.subs {
		fns = append(fns, fn)
	}
	h.mu.Unlock()
	for _, fn := range fns {
		fn(FSChange{Path: ev.Name, Kind: kind})
	}
}
