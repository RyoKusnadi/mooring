package fakemcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// envInt reads a whole number from the environment; 0 when unset or not a number.
func envInt(name string) int {
	n, _ := strconv.Atoi(os.Getenv(name))
	return n
}

// addTTL sets "ttlMs" of a tools/list result, or removes it when ms is 0 (the SDK always
// writes one; an older server writes none). Any other result comes back as it was.
func addTTL(result []byte, ms int) []byte {
	var res map[string]json.RawMessage
	if json.Unmarshal(result, &res) != nil || res["tools"] == nil {
		return result
	}
	if ms > 0 {
		res["ttlMs"], _ = json.Marshal(ms)
	} else {
		delete(res, "ttlMs")
	}
	out, err := json.Marshal(res)
	if err != nil {
		return result
	}
	return out
}

// injectTTL does that to the result of a whole JSON-RPC reply.
func injectTTL(body []byte, ms int) []byte {
	var msg map[string]json.RawMessage
	if json.Unmarshal(body, &msg) != nil || msg["result"] == nil {
		return body
	}
	msg["result"] = addTTL(msg["result"], ms)
	out, err := json.Marshal(msg)
	if err != nil {
		return body
	}
	return out
}

// withTTL answers POSTs in plain JSON with ttlMs set on tools/list results.
func withTTL(next http.Handler, ms func() int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.Header().Del("Content-Length")
		w.WriteHeader(rec.Code)
		_, _ = w.Write(injectTTL(rec.Body.Bytes(), ms()))
	})
}

// ttlTransport sets ttlMs on the tools/list replies of a server transport.
type ttlTransport struct {
	sdk.Transport
	ms int
}

func (t ttlTransport) Connect(ctx context.Context) (sdk.Connection, error) {
	c, err := t.Transport.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return ttlConn{c, t.ms}, nil
}

type ttlConn struct {
	sdk.Connection
	ms int
}

func (c ttlConn) Write(ctx context.Context, m jsonrpc.Message) error {
	if r, ok := m.(*jsonrpc.Response); ok && r.Error == nil {
		r.Result = addTTL(r.Result, c.ms)
	}
	return c.Connection.Write(ctx, m)
}
