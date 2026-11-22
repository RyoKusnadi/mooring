package files

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

const maxDirty = 4096

// loop turns fsnotify events into batched subscription marks.
func (w *watcher) loop(fw *fsnotify.Watcher) {
	for {
		select {
		case ev, ok := <-fw.Events:
			if !ok {
				return
			}
			w.event(ev)
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			w.s.log.Warn("files: watcher error", "err", err)
			w.mu.Lock()
			w.overflow = true
			w.arm()
			w.mu.Unlock()
		case <-w.s.cfg.Context.Done():
			w.close()
			return
		}
	}
}

func (w *watcher) event(ev fsnotify.Event) {
	if ev.Op == fsnotify.Chmod || isTemp(filepath.Base(ev.Name)) {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	parent := filepath.Dir(ev.Name)
	watched := len(w.byDir[parent]) > 0
	defer w.notify(ev, watched)
	defer w.mu.Unlock()
	for _, b := range w.byDir[parent] {
		w.dirty[b] = struct{}{}
	}
	for _, b := range w.byFile[ev.Name] {
		w.dirty[b] = struct{}{}
	}
	if ev.Op.Has(fsnotify.Remove) || ev.Op.Has(fsnotify.Rename) {
		// A watched folder that went away: tell its watchers and try to watch it again later.
		for _, b := range w.byDir[ev.Name] {
			w.dirty[b] = struct{}{}
			w.rescan[ev.Name] = true
		}
	}
	if len(w.dirty) > maxDirty {
		w.dirty = map[*sub]struct{}{}
		w.overflow = true
	}
	w.arm()
}

// arm starts the debounce timer if none runs. The caller holds mu.
func (w *watcher) arm() {
	if w.timer == nil && (len(w.dirty) > 0 || w.overflow) {
		w.timer = time.AfterFunc(w.s.cfg.Debounce, w.flush)
	}
}

type dirKey struct{ project, path string }

func (w *watcher) flush() {
	w.mu.Lock()
	w.timer = nil
	if w.closed {
		w.mu.Unlock()
		return
	}
	dirty, overflow, rescan := w.dirty, w.overflow, w.rescan
	w.dirty, w.overflow, w.rescan = map[*sub]struct{}{}, false, map[string]bool{}
	for d := range rescan {
		if w.refs[d] > 0 {
			_ = w.fs.Add(d)
		}
	}
	w.mu.Unlock()
	pub := w.s.cfg.Pub
	if overflow {
		pub.Publish("file.overflow", map[string]any{})
		return
	}
	dirs := map[dirKey]bool{}
	for b := range dirty {
		if b.file {
			w.fileChanged(b)
		} else if k := (dirKey{b.projectID, b.rel}); !dirs[k] {
			dirs[k] = true
			pub.Publish("file.dir", map[string]string{"projectId": b.projectID, "path": b.rel})
		}
	}
}

type fileEvent struct {
	ProjectID string `json:"projectId"`
	Path      string `json:"path"`
	Version   string `json:"version"`
	Size      int64  `json:"size"`
	Deleted   bool   `json:"deleted"`
	Binary    bool   `json:"binary"`
	TooLarge  bool   `json:"tooLarge"`
}

func (w *watcher) fileChanged(b *sub) {
	ev := fileEvent{ProjectID: b.projectID, Path: b.rel}
	info, err := os.Stat(b.real)
	switch {
	case errors.Is(err, os.ErrNotExist) || (err == nil && info.IsDir()):
		ev.Deleted = true
	case err != nil:
		w.s.log.Warn("files: stat changed file", "err", err)
		return
	case info.Size() > MaxFile:
		ev.Size, ev.TooLarge = info.Size(), true
	default:
		data, err := ReadWhole(b.real, MaxFile)
		if err != nil {
			return // gone or grown again; the next event follows
		}
		ev.Size, ev.Version, ev.Binary = int64(len(data)), Version(data), !IsText(data)
	}
	key := ev.Version
	if ev.Deleted {
		key = "deleted"
	} else if ev.TooLarge {
		key = "large"
	}
	w.mu.Lock()
	same := b.last == key
	b.last = key
	w.mu.Unlock()
	if same {
		return
	}
	w.s.cfg.Pub.Publish("file.changed", ev)
}

// reap drops holders that stopped renewing.
func (w *watcher) reap() {
	t := time.NewTicker(max(w.s.cfg.Lease/4, 10*time.Millisecond))
	defer t.Stop()
	for {
		select {
		case <-t.C:
			w.mu.Lock()
			if w.closed {
				w.mu.Unlock()
				return
			}
			now, dropped := time.Now(), false
			for id, h := range w.holders {
				if now.After(h.expires) {
					delete(w.holders, id)
					dropped = true
				}
			}
			if dropped {
				w.recount()
			}
			w.mu.Unlock()
		case <-w.s.cfg.Context.Done():
			return
		}
	}
}
