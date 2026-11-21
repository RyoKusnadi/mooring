package files

import (
	"strings"
)

func (w *watcher) otherHolders(id string) []*holder {
	var out []*holder
	for k, h := range w.holders {
		if k != id {
			out = append(out, h)
		}
	}
	return out
}

func (w *watcher) reindex() {
	w.byDir, w.byFile = map[string][]*sub{}, map[string][]*sub{}
	for _, h := range w.holders {
		for _, b := range h.subs {
			if b.file {
				w.byFile[b.real] = append(w.byFile[b.real], b)
			} else {
				w.byDir[b.real] = append(w.byDir[b.real], b)
			}
		}
	}
}

// CloseProject drops the watches of a removed project.
func (s *Service) CloseProject(projectID string) {
	w := s.w
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, h := range w.holders {
		kept := h.subs[:0]
		for _, b := range h.subs {
			if b.projectID != projectID {
				kept = append(kept, b)
			}
		}
		h.subs = kept
		if len(kept) == 0 {
			delete(w.holders, id)
		}
	}
	w.recount()
}

// recount rebuilds refs from the holders and stops watching unused folders.
func (w *watcher) recount() {
	need := map[string]int{}
	for _, h := range w.holders {
		for _, b := range h.subs {
			need[b.dir()]++
		}
	}
	if w.fs != nil {
		for d := range w.refs {
			if need[d] == 0 {
				_ = w.fs.Remove(d)
			}
		}
	}
	w.refs = need
	w.reindex()
}

func (w *watcher) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.timer != nil {
		w.timer.Stop()
	}
	if w.fs != nil {
		_ = w.fs.Close()
	}
}

func isTemp(name string) bool {
	return strings.HasPrefix(name, ".mooring-") && strings.HasSuffix(name, ".tmp")
}
