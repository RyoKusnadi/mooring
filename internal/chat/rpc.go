package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

const maxLine = 32 << 20

// rpcError is a JSON-RPC error object.
type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("agent error %d: %s", e.Code, e.Message) }

type rpcMsg struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

// handler receives what the agent sends. Notifications run in read order on the
// read loop, so they must not block. Requests run on their own goroutine.
type handler interface {
	notification(method string, params json.RawMessage)
	request(ctx context.Context, method string, params json.RawMessage) (any, error)
}

var errClosed = errors.New("agent connection closed")

// rpcConn is newline-delimited JSON-RPC 2.0 over a pipe pair. It is written by
// hand because the typed SDK drops fields Mooring needs.
type rpcConn struct {
	w  io.Writer
	h  handler
	mu sync.Mutex // guards w, next, pending, closed
	// next is the id of the next request.
	next    int64
	pending map[string]chan rpcMsg
	closed  bool
	done    chan struct{}
}

func newRPC(r io.Reader, w io.Writer, h handler) *rpcConn {
	c := &rpcConn{w: w, h: h, pending: map[string]chan rpcMsg{}, done: make(chan struct{})}
	go c.readLoop(r)
	return c
}

// Done is closed when the agent's output ends.
func (c *rpcConn) Done() <-chan struct{} { return c.done }

func (c *rpcConn) readLoop(r io.Reader) {
	defer c.finish()
	br := bufio.NewReaderSize(r, 64<<10)
	for {
		line, err := readLine(br)
		if len(line) > 0 {
			var m rpcMsg
			if json.Unmarshal(line, &m) == nil {
				c.dispatch(m)
			}
		}
		if err != nil {
			return
		}
	}
}

// readLine returns one line of any length up to maxLine.
func readLine(br *bufio.Reader) ([]byte, error) {
	var out []byte
	for {
		part, err := br.ReadSlice('\n')
		out = append(out, part...)
		if len(out) > maxLine {
			return nil, errors.New("agent line too long")
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return out, err
	}
}

func (c *rpcConn) finish() {
	c.mu.Lock()
	c.closed = true
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.mu.Unlock()
	close(c.done)
}

func (c *rpcConn) dispatch(m rpcMsg) {
	switch {
	case m.Method == "":
		c.mu.Lock()
		ch := c.pending[string(m.ID)]
		delete(c.pending, string(m.ID))
		c.mu.Unlock()
		if ch != nil {
			ch <- m
		}
	case len(m.ID) == 0:
		c.h.notification(m.Method, m.Params)
	default:
		go c.serve(m)
	}
}

func (c *rpcConn) serve(m rpcMsg) {
	defer func() {
		if v := recover(); v != nil {
			slog.Error("chat: agent request panicked", "method", m.Method, "panic", fmt.Sprint(v))
			_ = c.send(map[string]any{"jsonrpc": "2.0", "id": m.ID, "error": &rpcError{Code: -32603, Message: "Internal error"}})
		}
	}()
	res, err := c.h.request(context.Background(), m.Method, m.Params)
	reply := map[string]any{"jsonrpc": "2.0", "id": m.ID}
	if err != nil {
		var re *rpcError
		if !errors.As(err, &re) {
			re = &rpcError{Code: -32603, Message: err.Error()}
		}
		reply["error"] = re
	} else {
		reply["result"] = res
	}
	_ = c.send(reply)
}

func (c *rpcConn) send(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errClosed
	}
	_, err = c.w.Write(append(raw, '\n'))
	return err
}

// Call sends a request and waits for its result, which it decodes into out (if non-nil).
func (c *rpcConn) Call(ctx context.Context, method string, params, out any) error {
	return c.CallThen(ctx, method, params, out, nil)
}

// CallThen is Call that runs sent (if non-nil) once the request is written.
func (c *rpcConn) CallThen(ctx context.Context, method string, params, out any, sent func()) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errClosed
	}
	c.next++
	id := fmt.Sprint(c.next)
	ch := make(chan rpcMsg, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.send(map[string]any{"jsonrpc": "2.0", "id": json.Number(id), "method": method, "params": params}); err != nil {
		c.forget(id)
		return err
	}
	if sent != nil {
		sent()
	}
	select {
	case m, ok := <-ch:
		if !ok {
			return errClosed
		}
		if m.Error != nil {
			return m.Error
		}
		if out != nil && len(m.Result) > 0 {
			return json.Unmarshal(m.Result, out)
		}
		return nil
	case <-ctx.Done():
		c.forget(id)
		return ctx.Err()
	}
}

func (c *rpcConn) forget(id string) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// Notify sends a notification.
func (c *rpcConn) Notify(method string, params any) error {
	return c.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}
