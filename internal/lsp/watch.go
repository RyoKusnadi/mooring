package lsp

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/ryokusnadi/mooring/internal/files"
)

// syncWatch makes the watched folders those of the open documents. A server
// has one files holder; it is renewed every WatchRenew so it does not lapse.
func (i *inst) syncWatch() {
	w := i.s.cfg.Files
	if w == nil {
		return
	}
	i.watchMu.Lock()
	defer i.watchMu.Unlock()
	i.mu.Lock()
	if i.stopping {
		i.mu.Unlock()
		return
	}
	set := map[string]bool{}
	for _, d := range i.docs {
		set[filepath.Dir(d.real)] = true
	}
	i.mu.Unlock()
	dirs := make([]string, 0, len(set))
	for d := range set {
		rel, err := filepath.Rel(i.pl.root, d)
		if err != nil {
			continue
		}
		if rel == "." {
			rel = ""
		}
		dirs = append(dirs, filepath.ToSlash(rel))
	}
	sort.Strings(dirs)
	if _, err := w.Watch(files.WatchArgs{ProjectID: i.key.project, Holder: i.holder, Dirs: dirs}); err != nil {
		i.s.log.Warn("lsp: watch folders", "server", i.key.id, "err", err)
		return
	}
	i.mu.Lock()
	i.watched = set
	i.mu.Unlock()
}

func (i *inst) releaseWatch() {
	w := i.s.cfg.Files
	if w == nil {
		return
	}
	i.watchMu.Lock()
	defer i.watchMu.Unlock()
	if _, err := w.Watch(files.WatchArgs{ProjectID: i.key.project, Holder: i.holder}); err != nil {
		i.s.log.Warn("lsp: release watch", "server", i.key.id, "err", err)
	}
}

// fsChange is called for every raw change in a watched folder.
func (s *Service) fsChange(ch files.FSChange) {
	s.mu.Lock()
	list := make([]*inst, 0, len(s.insts))
	for _, i := range s.insts {
		list = append(list, i)
	}
	s.mu.Unlock()
	for _, i := range list {
		i.fsChange(ch)
	}
}

func (i *inst) fsChange(ch files.FSChange) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.stopping || !i.watched[filepath.Dir(ch.Path)] {
		return
	}
	rel, err := filepath.Rel(i.pl.real, ch.Path)
	if err != nil || rel == ".." || len(rel) > 2 && rel[:3] == "../" {
		return
	}
	uri := fileURI(filepath.Join(i.pl.dir, rel))
	if prev, ok := i.fsPending[uri]; !ok || ch.Kind == files.Deleted || (prev == files.Deleted && ch.Kind == files.Created) {
		i.fsPending[uri] = ch.Kind
	}
	if i.fsTimer == nil {
		i.fsTimer = time.AfterFunc(i.s.cfg.WatchDebounce, i.flushFS)
	}
}

// flushFS sends one workspace/didChangeWatchedFiles for what changed.
func (i *inst) flushFS() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.fsTimer = nil
	if i.stopping || !i.initializedSent || len(i.fsPending) == 0 {
		i.fsPending = map[string]int{}
		return
	}
	changes := make([]map[string]any, 0, len(i.fsPending))
	for uri, kind := range i.fsPending {
		changes = append(changes, map[string]any{"uri": uri, "type": kind})
	}
	i.fsPending = map[string]int{}
	sort.Slice(changes, func(a, b int) bool { return changes[a]["uri"].(string) < changes[b]["uri"].(string) })
	i.out.push(mustJSON(map[string]any{"jsonrpc": "2.0", "method": "workspace/didChangeWatchedFiles",
		"params": map[string]any{"changes": changes}}))
}
