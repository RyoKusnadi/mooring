package terminal

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/coder/websocket"

	"github.com/ryokusnadi/mooring/internal/events"
)

const (
	helloTimeout  = 10 * time.Second
	writeTimeout  = 10 * time.Second
	maxReadBytes  = 1 << 20
	maxInputBytes = 64 << 10
	// maxQueuedInput frames wait for the shell at most; the socket is closed past it.
	maxQueuedInput = 128
)

type clientFrame struct {
	T     string `json:"t"`
	Epoch string `json:"epoch"`
	Pos   uint64 `json:"pos"`
	Cols  uint16 `json:"cols"`
	Rows  uint16 `json:"rows"`
}

// Handler serves GET /api/terminals/{id}. The caller checks the session first.
func (m *Manager) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := m.get(r.PathValue("id"))
		if t == nil {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			m.log.Warn("terminal: socket refused", "err", err)
			return
		}
		(&conn{m: m, t: t, ws: ws}).serve(r.Context())
	})
}

type conn struct {
	m      *Manager
	t      *Terminal
	ws     *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

func (c *conn) idle() time.Duration {
	if c.m.Idle > 0 {
		return c.m.Idle
	}
	return events.IdleLimit
}

func (c *conn) read(timeout time.Duration) (websocket.MessageType, []byte, error) {
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	return c.ws.Read(ctx)
}

func (c *conn) write(typ websocket.MessageType, data []byte) {
	ctx, cancel := context.WithTimeout(c.ctx, writeTimeout)
	defer cancel()
	if err := c.ws.Write(ctx, typ, data); err != nil {
		c.cancel() // a socket that cannot be written to is finished
	}
}

func (c *conn) writeJSON(v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		c.m.log.Error("terminal: encode frame", "err", err)
		return
	}
	c.write(websocket.MessageText, raw)
}

func (c *conn) serve(reqCtx context.Context) {
	c.ctx, c.cancel = context.WithCancel(reqCtx)
	defer c.cancel()
	defer context.AfterFunc(c.m.ctx, c.cancel)()
	defer func() { _ = c.ws.CloseNow() }()
	c.ws.SetReadLimit(maxReadBytes)

	typ, data, err := c.read(helloTimeout)
	var hello clientFrame
	if err != nil || typ != websocket.MessageText || json.Unmarshal(data, &hello) != nil || hello.T != "hello" {
		_ = c.ws.Close(websocket.StatusPolicyViolation, "first frame must be hello")
		return
	}
	cl, state := c.t.attach(hello.Epoch, hello.Pos)
	defer c.t.detach(cl)
	c.writeJSON(state)
	go c.pump(cl)
	in := make(chan []byte, maxQueuedInput)
	go func() {
		for d := range in {
			c.t.writeInput(d)
		}
	}()
	defer close(in)

	for {
		typ, data, err := c.read(c.idle())
		if err != nil {
			return
		}
		if typ == websocket.MessageBinary {
			for len(data) > 0 {
				n := min(len(data), maxInputBytes)
				c.t.touch(cl)
				select {
				case in <- slices.Clone(data[:n]):
				default:
					_ = c.ws.Close(websocket.StatusMessageTooBig, "too much input waiting for the shell")
					return
				}
				data = data[n:]
			}
			continue
		}
		var f clientFrame
		if json.Unmarshal(data, &f) != nil {
			_ = c.ws.Close(websocket.StatusUnsupportedData, "frames must be JSON or binary")
			return
		}
		switch f.T {
		case "ack":
			c.t.ack(cl, f.Pos)
		case "resize":
			c.t.resize(cl, f.Cols, f.Rows)
		case "ping":
			c.writeJSON(map[string]string{"t": "pong"})
		}
	}
}

// pump sends what the client has not seen yet.
func (c *conn) pump(cl *client) {
	for {
		w := cl.next()
		switch {
		case w.state != nil:
			c.writeJSON(w.state)
		case w.size != nil:
			c.writeJSON(w.size)
		case w.data != nil:
			frame := make([]byte, 8+len(w.data))
			binary.BigEndian.PutUint64(frame, w.pos)
			copy(frame[8:], w.data)
			c.write(websocket.MessageBinary, frame)
		case w.exit != nil:
			c.writeJSON(w.exit)
		default:
			select {
			case <-cl.wake:
			case <-c.ctx.Done():
				return
			}
		}
		if c.ctx.Err() != nil {
			return
		}
	}
}
