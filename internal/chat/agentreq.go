package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/ryokusnadi/mooring/internal/files"
	"github.com/ryokusnadi/mooring/internal/paths"
)

const maxFileBytes = 5 << 20

// pendingPerm is a permission request waiting for an answer from any device.
type pendingPerm struct {
	msg    int
	answer chan string // option id, or "" for cancelled; buffered so answering never blocks
}

func (r *run) request(ctx context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case "session/request_permission":
		return r.c.askPermission(r, params)
	case "fs/read_text_file":
		return r.c.readFile(params)
	case "fs/write_text_file":
		return r.c.writeFile(params)
	}
	if strings.HasPrefix(method, "terminal/") {
		return r.terminal(method, params)
	}
	return nil, &rpcError{Code: -32601, Message: "Method not found: " + method}
}

type permParams struct {
	ToolCall struct {
		ID        string          `json:"toolCallId"`
		Title     string          `json:"title"`
		Kind      string          `json:"kind"`
		RawInput  json.RawMessage `json:"rawInput"`
		Locations json.RawMessage `json:"locations"`
		Content   json.RawMessage `json:"content"`
	} `json:"toolCall"`
	Options []struct {
		ID   string `json:"optionId"`
		Name string `json:"name"`
		Kind string `json:"kind"`
	} `json:"options"`
}

func cancelledOutcome() map[string]any {
	return map[string]any{"outcome": map[string]string{"outcome": "cancelled"}}
}

// askPermission records the request in the stream and waits, with no timeout,
// until a device answers or the turn is cancelled.
func (c *Chat) askPermission(r *run, raw json.RawMessage) (any, error) {
	var p permParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "Invalid params"}
	}
	id := make([]byte, 8)
	_, _ = rand.Read(id)
	reqID := hex.EncodeToString(id)
	msg := Message{
		Kind: KindPermission, RequestID: reqID, State: PermPending, ToolID: p.ToolCall.ID,
		Title: shorten(p.ToolCall.Title, 500), ToolKind: p.ToolCall.Kind, Locations: p.ToolCall.Locations,
	}
	msg.RawInput, msg.Truncated = capBlob(p.ToolCall.RawInput)
	var t bool
	msg.Content, t = capBlob(p.ToolCall.Content)
	msg.Truncated = msg.Truncated || t
	for _, o := range p.Options {
		msg.Options = append(msg.Options, Option{ID: o.ID, Name: o.Name, Kind: o.Kind})
	}
	pp := &pendingPerm{answer: make(chan string, 1)}
	c.mu.Lock()
	if c.cur != r || c.cancelling {
		c.mu.Unlock()
		return cancelledOutcome(), nil
	}
	c.openIdx = -1
	pp.msg = c.addLocked(msg)
	c.perms[reqID] = pp
	c.mu.Unlock()
	c.publishInfo("chat.changed") // attention marker

	var chosen string
	select {
	case chosen = <-pp.answer:
	case <-r.rpc.Done():
	}
	if chosen == "" {
		return cancelledOutcome(), nil
	}
	return map[string]any{"outcome": map[string]string{"outcome": "selected", "optionId": chosen}}, nil
}

// Answer records an answer to a permission request. The first answer wins; a
// later one changes nothing and reports what was chosen.
func (c *Chat) Answer(requestID, optionID string) (accepted bool, chosen string, err error) {
	c.mu.Lock()
	pp := c.perms[requestID]
	if pp == nil {
		defer c.mu.Unlock()
		for i := range c.msgs {
			if m := c.msgs[i]; m.Kind == KindPermission && m.RequestID == requestID {
				return false, m.Chosen, nil
			}
		}
		return false, "", apiErr("NOT_FOUND", "That permission request does not exist.")
	}
	m := &c.msgs[pp.msg]
	known := false
	for _, o := range m.Options {
		known = known || o.ID == optionID
	}
	if !known {
		c.mu.Unlock()
		return false, "", apiErr("BAD_REQUEST", "That is not one of the choices.")
	}
	delete(c.perms, requestID)
	m.State, m.Chosen = PermAnswered, optionID
	c.updateLocked(pp.msg)
	pp.answer <- optionID
	c.mu.Unlock()
	c.publishInfo("chat.changed")
	return true, optionID, nil
}

// cancelPermsLocked answers every open request as cancelled.
func (c *Chat) cancelPermsLocked() {
	for id, pp := range c.perms {
		delete(c.perms, id)
		c.msgs[pp.msg].State = PermCancelled
		c.updateLocked(pp.msg)
		pp.answer <- ""
	}
}

func (c *Chat) safePath(p string) (string, error) {
	if !strings.HasPrefix(p, "/") {
		return "", &rpcError{Code: -32602, Message: "Path must be absolute"}
	}
	real, err := paths.Resolve(c.projectDir, p)
	if errors.Is(err, paths.ErrOutsideRoot) {
		return "", &rpcError{Code: -32602, Message: "Path is outside the project"}
	}
	if err != nil {
		return "", &rpcError{Code: -32602, Message: "Path cannot be used"}
	}
	return real, nil
}

func (c *Chat) readFile(raw json.RawMessage) (any, error) {
	var p struct {
		Path  string `json:"path"`
		Line  int    `json:"line"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "Invalid params"}
	}
	real, err := c.safePath(p.Path)
	if err != nil {
		return nil, err
	}
	data, err := files.ReadWhole(real, maxFileBytes)
	if err != nil {
		return nil, &rpcError{Code: -32002, Message: "Cannot read that file (missing, a folder, or over 5 MB)"}
	}
	text := string(data)
	if p.Line < 0 || p.Limit < 0 {
		return nil, &rpcError{Code: -32602, Message: "Invalid params"}
	}
	if p.Line > 0 || p.Limit > 0 {
		lines := strings.SplitAfter(text, "\n")
		start := max(p.Line-1, 0)
		if start > len(lines) {
			start = len(lines)
		}
		end := len(lines)
		if p.Limit > 0 && p.Limit < end-start {
			end = start + p.Limit
		}
		text = strings.Join(lines[start:end], "")
	}
	return map[string]string{"content": text}, nil
}

func (c *Chat) writeFile(raw json.RawMessage) (any, error) {
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(raw, &p); err != nil || len(p.Content) > maxFileBytes {
		return nil, &rpcError{Code: -32602, Message: "Invalid params"}
	}
	real, err := c.safePath(p.Path)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(real); err == nil && info.IsDir() {
		return nil, &rpcError{Code: -32002, Message: "That path is a folder"}
	}
	write := c.m.WriteFile
	if write == nil {
		write = files.WriteAtomic
	}
	if err := write(real, []byte(p.Content)); err != nil {
		return nil, &rpcError{Code: -32002, Message: "Cannot write that file"}
	}
	return nil, nil
}
