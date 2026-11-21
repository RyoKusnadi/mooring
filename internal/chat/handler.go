package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/ryokusnadi/mooring/internal/events"
)

const (
	helloTimeout = 10 * time.Second
	writeTimeout = 10 * time.Second
	idleLimit    = 75 * time.Second
	maxReadBytes = 4 << 10
)

type clientFrame struct {
	T     string `json:"t"`
	Epoch string `json:"epoch"`
	Pos   uint64 `json:"pos"`
}

// Handler serves GET /api/chats/{id}: one chat's message stream. The caller
// checks the session first.
func (m *Manager) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := m.get(r.PathValue("id"))
		if c == nil {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			m.log.Warn("chat: socket refused", "err", err)
			return
		}
		(&conn{m: m, c: c, ws: ws, idle: m.idle()}).serve(r.Context())
	})
}

func (m *Manager) idle() time.Duration {
	if m.Idle > 0 {
		return m.Idle
	}
	return idleLimit
}

type conn struct {
	m      *Manager
	c      *Chat
	ws     *websocket.Conn
	idle   time.Duration
	ctx    context.Context
	cancel context.CancelFunc
}

func (k *conn) write(v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		k.m.log.Error("chat: encode frame", "err", err)
		return
	}
	ctx, cancel := context.WithTimeout(k.ctx, writeTimeout)
	defer cancel()
	if err := k.ws.Write(ctx, websocket.MessageText, raw); err != nil {
		k.cancel()
	}
}

func (k *conn) read(timeout time.Duration) (clientFrame, bool) {
	ctx, cancel := context.WithTimeout(k.ctx, timeout)
	defer cancel()
	_, data, err := k.ws.Read(ctx)
	var f clientFrame
	if err != nil || json.Unmarshal(data, &f) != nil {
		return f, false
	}
	return f, true
}

func (k *conn) serve(reqCtx context.Context) {
	k.ctx, k.cancel = context.WithCancel(reqCtx)
	defer k.cancel()
	defer func() { _ = k.ws.CloseNow() }()
	k.ws.SetReadLimit(maxReadBytes)
	hello, ok := k.read(helloTimeout)
	if !ok || hello.T != "hello" {
		_ = k.ws.Close(websocket.StatusPolicyViolation, "first frame must be hello")
		return
	}
	att, err := k.c.attach(hello.Epoch, hello.Pos)
	if err != nil {
		k.m.log.Error("chat: attach", "chat", k.c.id, "err", err)
		_ = k.ws.Close(websocket.StatusInternalError, "chat could not be read")
		return
	}
	defer k.c.hub.Detach(att.Sub)
	pos := att.Pos
	if att.Snapshot == nil {
		pos = hello.Pos
	}
	k.write(map[string]any{"t": "welcome", "epoch": att.Epoch, "pos": pos, "snapshot": att.Snapshot})
	for _, ev := range att.Replay {
		k.write(frame{T: "event", Event: ev})
	}
	go k.pump(att.Sub)
	for {
		f, ok := k.read(k.idle)
		if !ok {
			return
		}
		if f.T == "ping" {
			k.write(map[string]string{"t": "pong"})
		}
	}
}

type frame struct {
	T string `json:"t"`
	events.Event
}

func (k *conn) pump(s *events.Subscriber) {
	for {
		select {
		case ev := <-s.Queue:
			k.write(frame{T: "event", Event: ev})
		case <-s.Dropped:
			_ = k.ws.Close(websocket.StatusPolicyViolation, "too slow")
			k.cancel()
			return
		case <-k.ctx.Done():
			return
		}
	}
}
