package files

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/ryokusnadi/mooring/internal/paths"
)

var holderPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// WatchArgs is the args of file.watch.
type WatchArgs struct {
	ProjectID string   `json:"projectId"`
	Holder    string   `json:"holder"`
	Dirs      []string `json:"dirs"`
	Files     []string `json:"files"`
}

// WatchResult is the value of file.watch.
type WatchResult struct {
	Dirs    int       `json:"dirs"`
	Files   int       `json:"files"`
	Skipped []Skipped `json:"skipped"`
}

// Skipped is a path that could not be watched.
type Skipped struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

// sub is one thing a holder watches: a folder, or a file (through its folder).
type sub struct {
	projectID string
	rel       string
	file      bool
	real      string // the folder, or the file
	handles   int    // entries in the watched folder where each costs a file handle
	last      string // last version published for a file
}

func (b *sub) dir() string {
	if b.file {
		return filepath.Dir(b.real)
	}
	return b.real
}

type holder struct {
	subs    []*sub
	expires time.Time
}

// watcher owns the fsnotify watcher and every holder's subscriptions.
type watcher struct {
	s *Service

	mu       sync.Mutex
	fs       *fsnotify.Watcher
	holders  map[string]*holder
	refs     map[string]int // folder -> subscriptions needing it
	byDir    map[string][]*sub
	byFile   map[string][]*sub
	dirty    map[*sub]struct{}
	rescan   map[string]bool
	timer    *time.Timer
	closed   bool
	overflow bool
}

func newWatcher(s *Service) *watcher {
	return &watcher{s: s, holders: map[string]*holder{}, refs: map[string]int{},
		dirty: map[*sub]struct{}{}, rescan: map[string]bool{}}
}

// Watch replaces everything one holder watches.
func (s *Service) Watch(a WatchArgs) (*WatchResult, error) {
	if !holderPattern.MatchString(a.Holder) {
		return nil, apiErr("BAD_REQUEST", "A holder id needs 1 to 64 letters, digits, - or _.")
	}
	if len(a.Dirs)+len(a.Files) > maxHolderSet {
		return nil, apiErr("BAD_REQUEST", "At most %d paths can be watched at once.", maxHolderSet)
	}
	res := &WatchResult{Skipped: []Skipped{}}
	var subs []*sub
	if len(a.Dirs)+len(a.Files) > 0 {
		root, err := s.root(a.ProjectID)
		if err != nil {
			return nil, err
		}
		for _, p := range a.Dirs {
			b, err := s.dirSub(root, a.ProjectID, p)
			if err != nil {
				if sk, ok := skip(p, err); ok {
					res.Skipped = append(res.Skipped, sk)
					continue
				}
				return nil, err
			}
			subs = append(subs, b)
			res.Dirs++
		}
		for _, p := range a.Files {
			b, err := s.fileSub(root, a.ProjectID, p)
			if err != nil {
				if sk, ok := skip(p, err); ok {
					res.Skipped = append(res.Skipped, sk)
					continue
				}
				return nil, err
			}
			subs = append(subs, b)
			res.Files++
		}
	}
	if err := s.w.set(a.Holder, subs, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Service) dirSub(root, projectID, p string) (*sub, error) {
	real, err := s.follow(root, p)
	if err != nil {
		return nil, err
	}
	n, err := handleCost(real)
	if err != nil {
		if info, serr := os.Stat(real); serr == nil && !info.IsDir() {
			return nil, apiErr("NOT_A_FOLDER", "That is a file, not a folder.")
		}
		return nil, err
	}
	c, _ := clean(p)
	return &sub{projectID: projectID, rel: c, real: real, handles: n}, nil
}

// fileSub watches a file through its folder. The file may be missing (it can
// come back), but it must not be a folder: the project folder would put the
// watch on the folder above the project.
func (s *Service) fileSub(root, projectID, p string) (*sub, error) {
	real, err := s.follow(root, p)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(real); err == nil && info.IsDir() {
		return nil, apiErr("IS_A_FOLDER", "That is a folder, not a file.")
	}
	b := &sub{projectID: projectID, file: true, real: real}
	if !paths.Inside(root, b.dir()) {
		return nil, apiErr("OUTSIDE_PROJECT", "That path leaves the project folder.")
	}
	if b.handles, err = handleCost(b.dir()); err != nil {
		return nil, err
	}
	b.rel, _ = clean(p)
	return b, nil
}

// handleCost is how many file handles watching dir takes. On Linux it is one
// watch, counted as 0; elsewhere (kqueue) every entry costs a handle.
func handleCost(dir string) (int, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return 0, pathErr(err)
	}
	if runtime.GOOS == "linux" {
		return 0, nil
	}
	if len(des) > maxFolderHandles {
		return 0, apiErr("LIMIT_REACHED", "Too many files to watch in that folder.")
	}
	return len(des), nil
}

// skip turns a failure that leaves the rest of the command intact into a Skipped.
func skip(p string, err error) (Skipped, bool) {
	code, _ := errCode(err)
	switch code {
	case "NOT_FOUND", "NOT_A_FOLDER", "IS_A_FOLDER", "LIMIT_REACHED":
		return Skipped{Path: p, Code: code}, true
	}
	return Skipped{}, false
}

// set swaps in a holder's new subscriptions.
func (w *watcher) set(id string, subs []*sub, res *WatchResult) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return apiErr("INTERNAL", "The server is shutting down.")
	}
	old := w.holders[id]
	if old != nil {
		for _, n := range subs {
			for _, o := range old.subs {
				if o.file == n.file && o.projectID == n.projectID && o.real == n.real {
					n.last = o.last
				}
			}
		}
	}
	need, cost := map[string]int{}, map[string]int{}
	for _, h := range w.otherHolders(id) {
		for _, b := range h.subs {
			need[b.dir()]++
			cost[b.dir()] = b.handles
		}
	}
	fresh := map[string]bool{}
	for _, b := range subs {
		need[b.dir()]++
		cost[b.dir()] = b.handles
		fresh[b.dir()] = true
	}
	total := 0
	for _, n := range cost {
		total += n
	}
	if total > maxWatchHandles {
		return apiErr("LIMIT_REACHED", "Too many files are watched. Close some folders or files.")
	}
	added := 0
	for d := range fresh {
		if w.refs[d] == 0 {
			added++
		}
	}
	if len(w.refs)+added > maxWatchDirs {
		return apiErr("LIMIT_REACHED", "Too many folders are watched. Close some folders or files.")
	}
	if len(subs) > 0 && w.fs == nil {
		fw, err := fsnotify.NewWatcher()
		if err != nil {
			w.s.log.Error("files: start watcher", "err", err)
			return apiErr("INTERNAL", "Watching files is not available on this server.")
		}
		w.fs = fw
		go w.loop(fw)
		go w.reap()
	}
	for d := range fresh {
		if w.refs[d] == 0 {
			if err := w.fs.Add(d); err != nil {
				w.s.log.Warn("files: watch folder", "err", err)
				for d2 := range fresh { // undo the adds of this call
					if d2 != d && w.refs[d2] == 0 {
						_ = w.fs.Remove(d2)
					}
				}
				return apiErr("LIMIT_REACHED", "The system refused to watch that folder.")
			}
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
	if len(subs) == 0 {
		delete(w.holders, id)
	} else {
		w.holders[id] = &holder{subs: subs, expires: time.Now().Add(w.s.cfg.Lease)}
	}
	w.reindex()
	return nil
}
