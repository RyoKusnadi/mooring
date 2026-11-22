package lsp

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// fromServer handles one message the server wrote.
func (i *inst) fromServer(raw []byte) {
	var m rpcMap
	if json.Unmarshal(raw, &m) != nil {
		return
	}
	id, hasID := m["id"]
	method := unquote(m["method"])
	switch {
	case method != "" && hasID:
		i.answerServerRequest(id, method, m["params"])
	case method != "":
		i.broadcast(raw)
	case hasID:
		i.answer(id, m)
	}
}

func (i *inst) broadcast(raw []byte) {
	i.mu.Lock()
	socks := make([]*sock, 0, len(i.socks))
	for sk := range i.socks {
		socks = append(socks, sk)
	}
	i.mu.Unlock()
	for _, sk := range socks {
		sk.send(raw)
	}
}

// answerServerRequest answers a request from the server. The relay is the
// server's only client, so no socket sees it.
func (i *inst) answerServerRequest(id json.RawMessage, method string, params json.RawMessage) {
	var result any
	switch method {
	case "workspace/configuration":
		var p struct {
			Items []json.RawMessage `json:"items"`
		}
		_ = json.Unmarshal(params, &p)
		result = make([]any, len(p.Items))
	case "client/registerCapability", "client/unregisterCapability", "window/workDoneProgress/create":
		result = nil
	case "workspace/applyEdit":
		result = map[string]bool{"applied": false}
	default:
		i.out.push(mustJSON(map[string]any{"jsonrpc": "2.0", "id": id,
			"error": map[string]any{"code": -32601, "message": "Method not found"}}))
		return
	}
	i.out.push(mustJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": result}))
}

// answer sends a response to the socket that asked, with its own request id.
func (i *inst) answer(id json.RawMessage, m rpcMap) {
	n, err := strconv.ParseInt(string(bytes.TrimSpace(id)), 10, 64)
	if err != nil {
		return
	}
	i.mu.Lock()
	if n == i.initID && i.initState == 1 {
		waiters := i.initWaiters
		i.initWaiters = nil
		if errRaw, failed := m["error"]; failed {
			i.initState = 0
			i.mu.Unlock()
			for _, w := range waiters {
				w.sk.send(mustJSON(rpcMap{"jsonrpc": lit(`"2.0"`), "id": w.id, "error": errRaw}))
			}
			return
		}
		result := m["result"]
		i.initResult, i.initState = result, 2
		i.mu.Unlock()
		for _, w := range waiters {
			w.sk.reply(w.id, result)
		}
		return
	}
	pe, ok := i.pending[n]
	delete(i.pending, n)
	i.mu.Unlock()
	if !ok {
		return
	}
	m["id"] = pe.id
	pe.sk.send(mustJSON(m))
}
