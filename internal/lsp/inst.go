package lsp

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// instKey names one running server: project, folder (a place) and server id.
type instKey struct{ project, folder, id string }

// place is the folder a server works in.
type place struct {
	dir  string // the folder as the browser sees it (project path + folder); rootUri and cwd
	real string // dir with links resolved; every URI is checked against it
	root string // real folder of the project
}

type pend struct {
	sk *sock
	id json.RawMessage
}

type held struct {
	sk     *sock
	m      map[string]json.RawMessage
	method string
	isReq  bool
}

type doc struct {
	owner *sock
	real  string
	uri   string // as the owner spelled it, for didClose
}

// inst is one running language server and the sockets that share it.
type inst struct {
	s         *Service
	key       instKey
	argv      []string
	pl        place
	initOpts  map[string]any
	proc      *proc
	out       *queue
	startedAt int64
	holder    string

	watchMu sync.Mutex

	mu              sync.Mutex
	socks           map[*sock]struct{}
	nextID          int64
	pending         map[int64]pend
	initID          int64
	initState       int // 0 not asked, 1 asked, 2 answered
	initResult      json.RawMessage
	initWaiters     []pend
	initializedSent bool
	held            []held
	docs            map[string]*doc
	watched         map[string]bool // real folders watched through files
	fsPending       map[string]int  // URI -> kind, waiting for the debounce
	fsTimer         *time.Timer
	idleTimer       *time.Timer
	stopping        bool
	mb              int
}

func (i *inst) env() []string {
	env := make([]string, 0, 32)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "PATH=") {
			env = append(env, e)
		}
	}
	return append(env, "PATH="+strings.Join(i.s.pathDirs(), string(os.PathListSeparator)))
}

// startInst starts the server process. The caller holds s.mu.
func (s *Service) startInst(key instKey, pl place, r resolved) (*inst, error) {
	i := &inst{s: s, key: key, argv: r.argv, pl: pl, out: newQueue(), startedAt: s.cfg.Now().UnixMilli(),
		socks: map[*sock]struct{}{}, pending: map[int64]pend{}, docs: map[string]*doc{},
		watched: map[string]bool{}, fsPending: map[string]int{}, nextID: 1}
	i.initOpts = map[string]any{}
	for k, v := range r.def.InitOptions {
		i.initOpts[k] = v
	}
	if filepath.Base(r.path) == "typescript-language-server" {
		if p := tsserverPath(r.path); p != "" {
			i.initOpts["tsserver"] = map[string]any{"path": p}
		}
	}
	s.holders++
	i.holder = "lsp-" + itoa(s.holders)
	p, err := startProc(r.argv, pl.dir, i.env())
	if err != nil {
		return nil, err
	}
	i.proc = p
	s.wg.Add(2)
	go func() { defer s.wg.Done(); i.writeLoop() }()
	go func() { defer s.wg.Done(); i.readLoop() }()
	s.log.Info("lsp: server started", "server", key.id, "project", key.project, "folder", key.folder, "pid", p.pid())
	s.publish("lsp.started", map[string]any{"projectId": key.project, "folder": key.folder, "id": key.id})
	return i, nil
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func (i *inst) writeLoop() {
	for {
		b, ok := i.out.pop()
		if !ok {
			return
		}
		if err := writeMessage(i.proc.in, b); err != nil {
			i.out.close()
			i.proc.killGroup()
			return
		}
	}
}

func (i *inst) readLoop() {
	for {
		raw, err := readMessage(i.proc.out)
		if err != nil {
			break
		}
		i.fromServer(raw)
	}
	<-i.proc.exited
	i.crashed()
}

// begin marks the server as stopping and takes it out of the service's table.
// cond, if given, runs under the locks and can veto it. It returns the sockets.
func (i *inst) begin(cond func() bool) ([]*sock, bool) {
	s := i.s
	s.mu.Lock()
	defer s.mu.Unlock()
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.stopping || (cond != nil && !cond()) {
		return nil, false
	}
	i.stopping = true
	for _, t := range []*time.Timer{i.idleTimer, i.fsTimer} {
		if t != nil {
			t.Stop()
		}
	}
	if s.insts[i.key] == i {
		delete(s.insts, i.key)
	}
	socks := make([]*sock, 0, len(i.socks))
	for sk := range i.socks {
		socks = append(socks, sk)
	}
	i.socks = map[*sock]struct{}{}
	return socks, true
}

func closeAll(socks []*sock, code websocket.StatusCode, reason string) {
	for _, sk := range socks {
		sk.closeWith(code, reason)
	}
}

// stop ends the server: shutdown and exit, then the group is killed after the grace.
func (i *inst) stop(reason string, cond func() bool) {
	socks, ok := i.begin(cond)
	if !ok {
		return
	}
	closeAll(socks, websocket.StatusGoingAway, "stopped")
	i.mu.Lock()
	id := i.nextID
	i.nextID++
	i.mu.Unlock()
	i.out.push(mustJSON(map[string]any{"jsonrpc": "2.0", "id": id, "method": "shutdown"}))
	i.out.push(mustJSON(map[string]any{"jsonrpc": "2.0", "method": "exit"}))
	i.out.close()
	i.proc.finish(i.s.cfg.StopGrace)
	i.releaseWatch()
	i.s.log.Info("lsp: server stopped", "server", i.key.id, "project", i.key.project, "reason", reason)
	i.s.publish("lsp.stopped", map[string]any{"projectId": i.key.project, "folder": i.key.folder, "id": i.key.id, "reason": reason})
}

// crashed handles a process that ended by itself.
func (i *inst) crashed() {
	socks, ok := i.begin(nil)
	if !ok {
		return
	}
	closeAll(socks, 4002, "crashed")
	i.out.close()
	tail := i.proc.stderrTail(2048)
	i.proc.kill()
	i.releaseWatch()
	i.s.log.Warn("lsp: server crashed", "server", i.key.id, "project", i.key.project, "folder", i.key.folder)
	i.s.publish("lsp.stopped", map[string]any{"projectId": i.key.project, "folder": i.key.folder,
		"id": i.key.id, "reason": "crashed", "stderr": tail})
}

// restartForMemory kills a server over its memory limit. The next socket starts a new one.
func (i *inst) restartForMemory(mb int) {
	socks, ok := i.begin(nil)
	if !ok {
		return
	}
	closeAll(socks, 4001, "restarted")
	i.out.close()
	i.proc.kill()
	i.releaseWatch()
	i.s.log.Warn("lsp: server over the memory limit, restarted", "server", i.key.id, "mb", mb, "limitMb", i.s.cfg.MemoryMB)
	i.s.publish("lsp.restarted", map[string]any{"projectId": i.key.project, "folder": i.key.folder,
		"id": i.key.id, "reason": "memory", "mb": mb, "limitMb": i.s.cfg.MemoryMB})
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic("lsp: encode message: " + err.Error())
	}
	return b
}

func fileURI(p string) string {
	return (&url.URL{Scheme: "file", Path: p}).String()
}
