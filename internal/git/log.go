package git

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPage = 200
	maxPage     = 500
	maxQuery    = 200
	cacheKeep   = 4
	cacheBytes  = 48 << 20
)

// searchTimeout is how long a search runs before it answers what it found.
var searchTimeout = 55 * time.Second

// LogQuery is what GET /api/git/log asks for.
type LogQuery struct {
	Cursor string
	Limit  int // 0 for the default
	Text   string
	In     string // message, author or code
}

// LogPage is the answer of GET /api/git/log.
type LogPage struct {
	Commits []LogCommit `json:"commits"`
	Next    string      `json:"next"`
	Total   int         `json:"total"`
	Stopped bool        `json:"stopped,omitempty"`
}

// logOrder is the topological order of all commits for one state of the refs.
type logOrder struct {
	key  string
	w    int    // bytes per id
	raw  []byte // the ids, binary, one after another
	refs map[string][]Ref
}

func (o *logOrder) len() int {
	if o.w == 0 {
		return 0
	}
	return len(o.raw) / o.w
}

// ids returns the hex ids from a to b.
func (o *logOrder) ids(a, b int) []string {
	res := make([]string, 0, b-a)
	for i := a; i < b; i++ {
		res = append(res, hex.EncodeToString(o.raw[i*o.w:(i+1)*o.w]))
	}
	return res
}

// size is a rough count of the bytes the order holds.
func (o *logOrder) size() int { return len(o.raw) + len(o.refs)*120 }

// logCache keeps the last few orders, most recent first.
type logCache struct {
	mu    sync.Mutex
	build sync.Mutex
	list  []*logOrder
}

func (c *logCache) get(key string) *logOrder {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, o := range c.list {
		if o.key == key {
			copy(c.list[1:i+1], c.list[:i])
			c.list[0] = o
			return o
		}
	}
	return nil
}

func (c *logCache) put(o *logOrder) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.list = append([]*logOrder{o}, c.list...)
	total := 0
	for i, e := range c.list {
		total += e.size()
		if i > 0 && (i >= cacheKeep || total > cacheBytes) {
			c.list = c.list[:i]
			break
		}
	}
}

// order returns the commit order for the refs state, computing it once.
func (s *Service) order(ctx context.Context, pl *place, st *refsState) (*logOrder, error) {
	key := pl.top + "\x00" + st.key
	if o := s.lc.get(key); o != nil {
		return o, nil
	}
	s.lc.build.Lock()
	defer s.lc.build.Unlock()
	if o := s.lc.get(key); o != nil {
		return o, nil
	}
	o := &logOrder{key: key, refs: st.refs}
	if len(st.tipIDs) > 0 {
		// The ids read with the refs, not their names, so a ref that moves
		// meanwhile cannot put a newer order under this key.
		in := []byte(strings.Join(st.tipIDs, "\n") + "\n")
		out, err := s.run(ctx, pl.top, false, in, "rev-list", "--topo-order", "--stdin")
		if err != nil {
			return nil, s.failure(err)
		}
		if o.w, o.raw, err = packIDs(out); err != nil {
			return nil, apiErr("INTERNAL", "Something went wrong on the server.")
		}
	}
	s.lc.put(o)
	return o, nil
}

// tips are the revision arguments that select every commit of the history.
func tips(st *refsState) []string {
	a := []string{"--branches", "--remotes", "--tags"}
	if st.head != "" {
		a = append(a, "HEAD")
	}
	return a
}

// Log answers one page of the history, or of a search.
func (s *Service) Log(ctx context.Context, projectID, folder string, q LogQuery) (*LogPage, error) {
	if q.Limit == 0 {
		q.Limit = defaultPage
	}
	if q.Limit < 1 || q.Limit > maxPage {
		return nil, apiErr("BAD_REQUEST", "The page size is 1 to %d.", maxPage)
	}
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	st, err := s.readRefs(ctx, pl)
	if err != nil {
		return nil, err
	}
	if q.Text != "" || q.In != "" {
		return s.search(ctx, pl, st, q)
	}
	var off int
	if q.Cursor != "" {
		key, n, ok := strings.Cut(q.Cursor, ".")
		off, err = strconv.Atoi(n)
		if !ok || err != nil || off < 0 {
			return nil, apiErr("BAD_REQUEST", "That page cursor is not valid.")
		}
		if key != st.key {
			return nil, apiErr("LOG_CHANGED", "The branches moved. Reloading the history from the top.")
		}
	}
	o, err := s.order(ctx, pl, st)
	if err != nil {
		return nil, err
	}
	if off > o.len() {
		return nil, apiErr("BAD_REQUEST", "That page cursor is not valid.")
	}
	end := min(off+q.Limit, o.len())
	page := &LogPage{Commits: []LogCommit{}, Total: o.len()}
	if end > off {
		if page.Commits, err = s.details(ctx, pl, o.ids(off, end), o.refs); err != nil {
			return nil, err
		}
	}
	if end < o.len() {
		page.Next = st.key + "." + strconv.Itoa(end)
	}
	return page, nil
}

// details reads the commits with one git run, in the order of ids.
func (s *Service) details(ctx context.Context, pl *place, ids []string, refs map[string][]Ref) ([]LogCommit, error) {
	in := []byte(strings.Join(ids, "\n") + "\n")
	out, err := s.run(ctx, pl.top, false, in, "log", "--no-walk=unsorted", "--stdin", "-z", "--no-show-signature", logFormat)
	if err != nil {
		return nil, s.failure(err)
	}
	list := parseLog(out, refs)
	if len(list) != len(ids) {
		byID := map[string]LogCommit{}
		for _, c := range list {
			byID[c.ID] = c
		}
		list = list[:0]
		for _, id := range ids {
			if c, ok := byID[id]; ok {
				list = append(list, c)
			}
		}
	}
	return list, nil
}

func (s *Service) search(ctx context.Context, pl *place, st *refsState, q LogQuery) (*LogPage, error) {
	if q.Text == "" || len(q.Text) > maxQuery || strings.ContainsRune(q.Text, 0) {
		return nil, apiErr("BAD_REQUEST", "Search text is 1 to %d bytes.", maxQuery)
	}
	var filter []string
	switch q.In {
	case "message", "":
		filter = []string{"--grep=" + q.Text, "--fixed-strings", "-i"}
	case "author":
		filter = []string{"--author=" + q.Text, "--fixed-strings", "-i"}
	case "code":
		filter = []string{"-S" + q.Text}
	default:
		return nil, apiErr("BAD_REQUEST", "Search in message, author or code.")
	}
	off := 0
	if q.Cursor != "" {
		n, err := strconv.Atoi(q.Cursor)
		if err != nil || n < 0 {
			return nil, apiErr("BAD_REQUEST", "That page cursor is not valid.")
		}
		off = n
	}
	page := &LogPage{Commits: []LogCommit{}, Total: -1}
	if st.head == "" && len(st.refs) == 0 {
		return page, nil
	}
	args := append([]string{"log", "-z", "--no-show-signature", logFormat, "--skip=" + strconv.Itoa(off), "-n", strconv.Itoa(q.Limit + 1)}, filter...)
	args = append(append(args, tips(st)...), "--")
	sctx, stop := context.WithTimeout(ctx, searchTimeout)
	defer stop()
	out, _, err := s.runX(sctx, pl.top, false, nil, 0, nil, args...)
	if err != nil {
		var re *runError
		if !asRun(err, &re) || !re.timedOut {
			return nil, s.failure(err)
		}
		page.Stopped = true
		out = []byte(re.stdout)
	}
	page.Commits, page.Next = trimSearch(parseLog(out, st.refs), q.Limit, off, page.Stopped)
	return page, nil
}

// trimSearch cuts the commits git printed (one more than the limit, to tell
// whether a next page exists) to the limit, and names the next page.
func trimSearch(list []LogCommit, limit, off int, stopped bool) ([]LogCommit, string) {
	if len(list) <= limit {
		return list, ""
	}
	if stopped {
		return list[:limit], ""
	}
	return list[:limit], strconv.Itoa(off + limit)
}

// packIDs turns lines of hex ids into binary ids of one width.
func packIDs(out []byte) (w int, raw []byte, err error) {
	for len(out) > 0 {
		line := out
		if i := bytes.IndexByte(out, '\n'); i >= 0 {
			line, out = out[:i], out[i+1:]
		} else {
			out = nil
		}
		if len(line) == 0 {
			continue
		}
		if w == 0 {
			w = len(line) / 2
			raw = make([]byte, 0, len(out)/(len(line)+1)*w+w)
		}
		if len(line) != w*2 {
			return 0, nil, errors.New("ids of different widths")
		}
		n := len(raw)
		raw = append(raw, make([]byte, w)...)
		if _, err := hex.Decode(raw[n:], line); err != nil {
			return 0, nil, err
		}
	}
	return w, raw, nil
}
