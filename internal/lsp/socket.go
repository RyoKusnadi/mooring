package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/ryokusnadi/mooring/internal/paths"
)

const (
	maxFrame     = maxMessage
	writeTimeout = 10 * time.Second
	pingEvery    = 20 * time.Second
	pingWait     = 60 * time.Second
	sockQueue    = 4096
)

// sock is one browser socket attached to a server.
type sock struct {
	ws   *websocket.Conn
	out  chan []byte
	done chan struct{}
	once sync.Once
}

func newSock(ws *websocket.Conn) *sock {
	return &sock{ws: ws, out: make(chan []byte, sockQueue), done: make(chan struct{})}
}

// send queues a message. A socket that cannot keep up is closed, not waited for.
func (sk *sock) send(b []byte) {
	select {
	case sk.out <- b:
	default:
		sk.closeWith(websocket.StatusTryAgainLater, "too slow")
	}
}

// closeWith closes the socket with a code the browser can act on.
func (sk *sock) closeWith(code websocket.StatusCode, reason string) {
	sk.once.Do(func() {
		close(sk.done)
		go func() { _ = sk.ws.Close(code, reason) }()
	})
}

func (sk *sock) writeLoop() {
	for {
		select {
		case b := <-sk.out:
			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := sk.ws.Write(ctx, websocket.MessageText, b)
			cancel()
			if err != nil {
				_ = sk.ws.CloseNow()
				return
			}
		case <-sk.done:
			return
		}
	}
}

func (sk *sock) pingLoop(stop <-chan struct{}) {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), pingWait)
			err := sk.ws.Ping(ctx)
			cancel()
			if err != nil {
				_ = sk.ws.CloseNow()
				return
			}
		case <-stop:
			return
		case <-sk.done:
			return
		}
	}
}

func jsonError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "code": code})
}

// Handler serves GET /api/lsp/{id}?project=&folder=. The caller checks the session first.
func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		if closed {
			jsonError(w, http.StatusServiceUnavailable, "SHUTTING_DOWN")
			return
		}
		d, ok, err := s.def(r.PathValue("id"))
		if err != nil {
			s.log.Error("lsp: read servers", "err", err)
			jsonError(w, http.StatusInternalServerError, "INTERNAL")
			return
		}
		if !ok {
			jsonError(w, http.StatusNotFound, "NOT_FOUND")
			return
		}
		q := r.URL.Query()
		folder := q.Get("folder")
		pl, status, code := s.place(q.Get("project"), &folder)
		if status != 0 {
			jsonError(w, status, code)
			return
		}
		res := s.resolve(d)
		switch res.state {
		case StateNeedsConsent:
			jsonError(w, http.StatusConflict, "NEEDS_CONSENT")
			return
		case StateNotInstalled, StateInstalling:
			jsonError(w, http.StatusConflict, "NOT_INSTALLED")
			return
		}
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			s.log.Warn("lsp: socket refused", "err", err)
			return
		}
		s.serve(ws, instKey{q.Get("project"), folder, d.ID}, pl, res)
	})
}

// place resolves the project and folder of a request. status is 0 when it works.
func (s *Service) place(projectID string, folder *string) (place, int, string) {
	dir, err := s.cfg.Dir(projectID)
	if err != nil {
		return place{}, http.StatusNotFound, "NOT_FOUND"
	}
	root, err := paths.Root(dir)
	if err != nil {
		return place{}, http.StatusNotFound, "NOT_FOUND"
	}
	f := *folder
	if len(f) > 4096 || strings.ContainsRune(f, 0) || filepath.IsAbs(f) {
		return place{}, http.StatusBadRequest, "BAD_REQUEST"
	}
	if f = path.Clean(f); f == "." {
		f = ""
	}
	real, err := paths.Resolve(root, filepath.FromSlash(f))
	if err != nil {
		if errors.Is(err, paths.ErrOutsideRoot) {
			return place{}, http.StatusBadRequest, "OUTSIDE_PROJECT"
		}
		return place{}, http.StatusNotFound, "NOT_FOUND"
	}
	if fi, err := os.Stat(real); err != nil || !fi.IsDir() {
		return place{}, http.StatusNotFound, "NOT_FOUND"
	}
	*folder = f
	return place{dir: filepath.Join(dir, filepath.FromSlash(f)), real: real, root: root}, 0, ""
}

func (s *Service) serve(ws *websocket.Conn, key instKey, pl place, res resolved) {
	ws.SetReadLimit(maxFrame)
	sk := newSock(ws)
	i, err := s.attach(key, pl, res, sk)
	if err != nil {
		s.log.Error("lsp: start server", "server", key.id, "err", err)
		_ = ws.Close(4002, "could not start")
		return
	}
	stopPing := make(chan struct{})
	go sk.writeLoop()
	go sk.pingLoop(stopPing)
	for {
		typ, data, err := ws.Read(context.Background())
		if err != nil {
			break
		}
		if typ != websocket.MessageText {
			sk.closeWith(websocket.StatusUnsupportedData, "frames must be text")
			continue
		}
		i.fromSocket(sk, data)
	}
	close(stopPing)
	i.removeSock(sk)
	sk.once.Do(func() { close(sk.done) })
	_ = ws.CloseNow()
}

// attach finds or starts the server of key and adds the socket.
func (s *Service) attach(key instKey, pl place, res resolved, sk *sock) (*inst, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("the server is shutting down")
	}
	i := s.insts[key]
	if i == nil {
		var err error
		if i, err = s.startInst(key, pl, res); err != nil {
			return nil, err
		}
		s.insts[key] = i
	}
	i.mu.Lock()
	i.socks[sk] = struct{}{}
	if i.idleTimer != nil {
		i.idleTimer.Stop()
		i.idleTimer = nil
	}
	i.mu.Unlock()
	return i, nil
}

// removeSock detaches a closed socket: its documents are closed for the
// server and, with no sockets left, the idle timer starts.
func (i *inst) removeSock(sk *sock) {
	i.mu.Lock()
	if i.stopping {
		i.mu.Unlock()
		return
	}
	delete(i.socks, sk)
	for key, d := range i.docs {
		if d.owner == sk {
			delete(i.docs, key)
			uri := d.uri
			i.out.push(mustJSON(rpcMap{"jsonrpc": lit(`"2.0"`), "method": lit(`"textDocument/didClose"`),
				"params": mustJSON(map[string]any{"textDocument": map[string]string{"uri": uri}})}))
		}
	}
	for n, pe := range i.pending {
		if pe.sk == sk {
			delete(i.pending, n)
		}
	}
	if len(i.socks) == 0 {
		i.idleTimer = time.AfterFunc(i.s.cfg.Idle, func() {
			i.stop("idle", func() bool { return len(i.socks) == 0 })
		})
	}
	i.mu.Unlock()
	i.syncWatch()
}
