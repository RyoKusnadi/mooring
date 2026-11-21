package lsp

import (
	"bytes"
	"encoding/json"
	"path/filepath"
)

type rpcMap = map[string]json.RawMessage

const maxHeld = 1000

func unquote(raw json.RawMessage) string {
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}

func (sk *sock) reply(id json.RawMessage, result any) {
	sk.send(mustJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": result}))
}

func (sk *sock) fail(id json.RawMessage, code int, msg string) {
	sk.send(mustJSON(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": msg}}))
}

// fromSocket handles one JSON-RPC message from a socket.
func (i *inst) fromSocket(sk *sock, raw []byte) {
	var m rpcMap
	if json.Unmarshal(raw, &m) != nil {
		return
	}
	method := unquote(m["method"])
	if method == "" {
		return // the relay answers the server's requests itself
	}
	id, hasID := m["id"]
	isReq := hasID && !bytes.Equal(bytes.TrimSpace(id), []byte("null"))
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.stopping {
		return
	}
	if _, ok := i.socks[sk]; !ok {
		return
	}
	i.route(sk, m, method, isReq)
}

// route decides what a socket's message does. The caller holds i.mu, and the
// order of i.out.push calls under it is the order the server sees.
func (i *inst) route(sk *sock, m rpcMap, method string, isReq bool) {
	id := m["id"]
	switch method {
	case "initialize":
		if isReq {
			i.routeInitialize(sk, m)
		}
		return
	case "initialized":
		if !i.initializedSent && i.initState == 2 {
			i.initializedSent = true
			i.out.push(mustJSON(m))
			queued := i.held
			i.held = nil
			for _, h := range queued {
				if _, ok := i.socks[h.sk]; ok {
					i.route(h.sk, h.m, h.method, h.isReq)
				}
			}
		}
		return
	case "shutdown":
		if isReq {
			sk.reply(id, nil)
		}
		return
	case "exit", "workspace/didChangeWatchedFiles", "workspace/didChangeWorkspaceFolders":
		return
	}
	if err := i.checkParams(m["params"]); err != nil {
		if isReq {
			sk.fail(id, -32602, "That file is outside the folder this language server works in.")
		}
		return
	}
	if !i.initializedSent {
		if len(i.held) < maxHeld {
			i.held = append(i.held, held{sk, m, method, isReq})
		} else if isReq {
			sk.fail(id, -32803, "The language server is still starting and has too many requests waiting.")
		}
		return
	}
	i.dispatch(sk, m, method, isReq)
}

func (i *inst) dispatch(sk *sock, m rpcMap, method string, isReq bool) {
	uri, _ := textDocumentURI(m)
	switch method {
	case "textDocument/didOpen":
		i.didOpen(sk, m, uri)
		return
	case "textDocument/didClose":
		if d := i.docOf(uri); d != nil && d.owner == sk {
			delete(i.docs, d.real)
			i.out.push(mustJSON(withURI(m, d.uri)))
			go i.syncWatch()
		}
		return
	case "textDocument/didChange", "textDocument/didSave", "textDocument/willSave":
		if d := i.docOf(uri); d != nil && d.owner == sk {
			i.out.push(mustJSON(withURI(m, d.uri)))
		}
		return
	case "$/cancelRequest":
		var p struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal(m["params"], &p) == nil {
			for nid, pe := range i.pending {
				if pe.sk == sk && bytes.Equal(pe.id, p.ID) {
					i.out.push(mustJSON(rpcMap{"jsonrpc": lit(`"2.0"`), "method": lit(`"$/cancelRequest"`),
						"params": mustJSON(map[string]int64{"id": nid})}))
					break
				}
			}
		}
		return
	}
	if !isReq {
		i.out.push(mustJSON(m))
		return
	}
	nid := i.nextID
	i.nextID++
	i.pending[nid] = pend{sk, m["id"]}
	m["id"] = mustJSON(nid)
	i.out.push(mustJSON(m))
}

func lit(s string) json.RawMessage { return json.RawMessage(s) }

// didOpen gives the document to sk. A document open for another socket is
// closed for it first, so the server sees one open at a time.
func (i *inst) didOpen(sk *sock, m rpcMap, uri string) {
	real, err := i.checkURI(uri)
	if err != nil {
		return
	}
	if old, open := i.docs[real]; open {
		i.out.push(mustJSON(rpcMap{"jsonrpc": lit(`"2.0"`), "method": lit(`"textDocument/didClose"`),
			"params": mustJSON(map[string]any{"textDocument": map[string]string{"uri": old.uri}})}))
	}
	i.docs[real] = &doc{owner: sk, real: real, uri: uri}
	i.out.push(mustJSON(m))
	go i.syncWatch()
}

func textDocumentURI(m rpcMap) (string, bool) {
	var p struct {
		TextDocument *struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if json.Unmarshal(m["params"], &p) != nil || p.TextDocument == nil {
		return "", false
	}
	return p.TextDocument.URI, true
}

// routeInitialize sends the first initialize to the server, rewritten for the
// place, and answers later ones from the first answer.
func (i *inst) routeInitialize(sk *sock, m rpcMap) {
	id := m["id"]
	switch i.initState {
	case 2:
		sk.reply(id, i.initResult)
		return
	case 1:
		i.initWaiters = append(i.initWaiters, pend{sk, id})
		return
	}
	params := map[string]any{}
	dec := json.NewDecoder(bytes.NewReader(m["params"]))
	dec.UseNumber()
	_ = dec.Decode(&params)
	if params == nil {
		params = map[string]any{}
	}
	uri := fileURI(i.pl.dir)
	params["rootUri"] = uri
	params["rootPath"] = i.pl.dir
	params["workspaceFolders"] = []map[string]string{{"uri": uri, "name": filepath.Base(i.pl.dir)}}
	opts, _ := params["initializationOptions"].(map[string]any)
	if merged := mergeOptions(opts, i.initOpts); len(merged) > 0 {
		params["initializationOptions"] = merged
	}
	i.initState = 1
	i.initID = i.nextID
	i.nextID++
	i.initWaiters = []pend{{sk, id}}
	i.out.push(mustJSON(map[string]any{"jsonrpc": "2.0", "id": i.initID, "method": "initialize", "params": params}))
}

// mergeOptions copies over into base; maps merge, anything else is replaced.
func mergeOptions(base, over map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		bm, bok := out[k].(map[string]any)
		om, ook := v.(map[string]any)
		if bok && ook {
			out[k] = mergeOptions(bm, om)
		} else {
			out[k] = v
		}
	}
	return out
}
