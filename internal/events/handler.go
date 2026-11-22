package events

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Timings.
const (
	IdleLimit       = 75 * time.Second
	HiddenIdleLimit = 300 * time.Second
	helloTimeout    = 10 * time.Second
	writeTimeout    = 10 * time.Second
	maxFrameBytes   = 160 << 10
)

// Command is one browser command, already checked for the run-once rules.
type Command struct {
	ID   string
	Name string
	Args json.RawMessage
}

// Commands runs commands by name. Errors should be *ledger.Error.
type Commands interface {
	Run(ctx context.Context, cmd Command) (any, error)
	// Secret reports whether the command's args and value must not be stored.
	Secret(name string) bool
}

// Config is what the socket handler needs.
type Config struct {
	Hub      *Hub
	Ledger   *ledger.Ledger
	Commands Commands
	// Snapshot builds the state a client resets from.
	Snapshot func() (any, error)
	Log      *slog.Logger
	// Context ends all sockets when cancelled (server shutdown).
	Context context.Context
	// PendingAfter overrides ledger.PendingAfter; tests only.
	PendingAfter time.Duration
	// Idle overrides IdleLimit and HiddenIdleLimit; tests only.
	Idle time.Duration
}

type clientFrame struct {
	T      string          `json:"t"`
	Epoch  string          `json:"epoch"`
	Pos    uint64          `json:"pos"`
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Args   json.RawMessage `json:"args"`
	Hidden bool            `json:"hidden"`
}

type welcomeFrame struct {
	T        string `json:"t"`
	Epoch    string `json:"epoch"`
	Pos      uint64 `json:"pos"`
	Snapshot any    `json:"snapshot,omitempty"`
}

type eventFrame struct {
	T string `json:"t"`
	Event
}

type resultFrame struct {
	T     string          `json:"t"`
	ID    string          `json:"id"`
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value,omitempty"`
	Error *ledger.Error   `json:"error,omitempty"`
}

// Handler serves GET /api/events. The caller checks the session first; the
// WebSocket library's own Origin check stays on in addition to the guard.
func Handler(cfg Config) http.Handler {
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	if cfg.PendingAfter == 0 {
		cfg.PendingAfter = ledger.PendingAfter
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			cfg.Log.Warn("events: socket refused", "err", err)
			return
		}
		(&conn{cfg: cfg, ws: ws}).serve(r.Context())
	})
}

type conn struct {
	cfg    Config
	ws     *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

func (c *conn) write(v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		c.cfg.Log.Error("events: encode frame", "err", err)
		return
	}
	ctx, cancel := context.WithTimeout(c.ctx, writeTimeout)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, raw); err != nil {
		c.cancel() // a socket that cannot be written to is finished
	}
}

func (c *conn) read(timeout time.Duration) (clientFrame, error) {
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	_, data, err := c.ws.Read(ctx)
	if err != nil {
		return clientFrame{}, err
	}
	var f clientFrame
	if err := json.Unmarshal(data, &f); err != nil {
		return clientFrame{}, errBadFrame
	}
	return f, nil
}

var errBadFrame = errors.New("bad frame")

func (c *conn) serve(reqCtx context.Context) {
	c.ctx, c.cancel = context.WithCancel(reqCtx)
	defer c.cancel()
	stop := context.AfterFunc(c.cfg.Context, c.cancel)
	defer stop()
	defer func() { _ = c.ws.CloseNow() }()
	c.ws.SetReadLimit(maxFrameBytes)

	hello, err := c.read(helloTimeout)
	if err != nil || hello.T != "hello" {
		_ = c.ws.Close(websocket.StatusPolicyViolation, "first frame must be hello")
		return
	}
	att, err := c.cfg.Hub.Attach(hello.Epoch, hello.Pos, c.cfg.Snapshot)
	if err != nil {
		c.cfg.Log.Error("events: build snapshot", "err", err)
		_ = c.ws.Close(websocket.StatusInternalError, "snapshot failed")
		return
	}
	defer c.cfg.Hub.Detach(att.Sub)

	w := welcomeFrame{T: "welcome", Epoch: att.Epoch, Pos: att.Pos, Snapshot: att.Snapshot}
	if att.Snapshot == nil {
		w.Pos = hello.Pos // resuming: replayed events follow this position
	}
	c.write(w)
	for _, ev := range att.Replay {
		c.write(eventFrame{T: "event", Event: ev})
	}
	go c.pump(att.Sub)

	hidden := false
	for {
		idle := c.idleLimit(hidden)
		f, err := c.read(idle)
		if errors.Is(err, errBadFrame) {
			_ = c.ws.Close(websocket.StatusUnsupportedData, "frames must be JSON")
			return
		}
		if err != nil {
			return
		}
		switch f.T {
		case "ping":
			c.write(map[string]string{"t": "pong"})
		case "visibility":
			hidden = f.Hidden
		case "cmd":
			c.command(f)
		}
	}
}

func (c *conn) idleLimit(hidden bool) time.Duration {
	if c.cfg.Idle > 0 {
		return c.cfg.Idle
	}
	if hidden {
		return HiddenIdleLimit
	}
	return IdleLimit
}

func (c *conn) pump(s *Subscriber) {
	for {
		select {
		case ev := <-s.Queue:
			c.write(eventFrame{T: "event", Event: ev})
		case <-s.Dropped:
			_ = c.ws.Close(websocket.StatusPolicyViolation, "too slow")
			c.cancel()
			return
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *conn) command(f clientFrame) {
	args := f.Args
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	cmd := Command{ID: f.ID, Name: f.Name, Args: args}
	secret := c.cfg.Commands.Secret(f.Name)
	call := c.cfg.Ledger.Submit(f.ID, f.Name, args, secret, func(ctx context.Context) (any, error) {
		c.cfg.Log.Info("command", "name", cmd.Name, "id", cmd.ID)
		return c.cfg.Commands.Run(ctx, cmd)
	})
	go func() {
		out, ok := call.Wait(c.ctx, c.cfg.PendingAfter, func() {
			c.write(map[string]string{"t": "pending", "id": f.ID})
		})
		if !ok {
			return
		}
		if !out.OK {
			c.cfg.Log.Warn("command failed", "name", f.Name, "id", f.ID, "code", out.Error.Code)
		}
		c.write(resultFrame{T: "result", ID: f.ID, OK: out.OK, Value: out.Value, Error: out.Error})
	}()
}
