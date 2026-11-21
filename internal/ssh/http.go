package ssh

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/ryokusnadi/mooring/internal/files"
	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Mount registers the ssh routes. wrap adds the session check.
func (m *Manager) Mount(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	h := func(f http.HandlerFunc) http.Handler { return wrap(f) }
	mux.Handle("GET /api/ssh/hosts", h(m.httpHosts))
	mux.Handle("GET /api/ssh/config", h(m.httpConfig))
	mux.Handle("GET /api/ssh/files/list", h(m.httpList))
	mux.Handle("GET /api/ssh/files/read", h(m.httpRead))
	mux.Handle("GET /api/ssh/files/download", h(m.httpDownload))
	mux.Handle("PUT /api/ssh/files/write", h(m.httpWrite))
}

var statusOf = map[string]int{
	"BAD_REQUEST": 400, "NOT_A_FOLDER": 400, "IS_A_FOLDER": 400,
	"OUTSIDE_ROOT": 403, "PERMISSION_DENIED": 403, "NOT_FOUND": 404,
	"FILE_CHANGED": 409, "ALREADY_EXISTS": 409, "NOT_CONNECTED": 409, "COMMAND_CONFLICT": 409, "OUTCOME_UNKNOWN": 409,
	"FILE_TOO_LARGE": 413, "HOST_UNREACHABLE": 502, "SFTP_UNAVAILABLE": 502,
}

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (m *Manager) fail(w http.ResponseWriter, err error) {
	var le *ledger.Error
	if !errors.As(err, &le) {
		m.log.Error("ssh: request failed", "err", err)
		le = apiErr("INTERNAL", "Something went wrong on the server.")
	}
	st := statusOf[le.Code]
	if st == 0 {
		st = 500
	}
	reply(w, st, map[string]any{"ok": false, "error": le})
}

func (m *Manager) httpHosts(w http.ResponseWriter, _ *http.Request) {
	hosts, err := m.Hosts()
	if err != nil {
		m.fail(w, err)
		return
	}
	reply(w, 200, map[string]any{"hosts": hosts, "canRemember": m.CanRemember()})
}

func (m *Manager) httpConfig(w http.ResponseWriter, _ *http.Request) {
	reply(w, 200, map[string]any{"hosts": m.ReadConfig()})
}

func (m *Manager) httpList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := m.List(q.Get("conn"), q.Get("path"))
	if err != nil {
		m.fail(w, err)
		return
	}
	reply(w, 200, res)
}

func num[T int | int64](s string) (T, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return T(n), err
}

func (m *Manager) httpRead(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, err1 := num[int64](q.Get("offset"))
	limit, err2 := num[int](q.Get("limit"))
	if err1 != nil || err2 != nil {
		m.fail(w, apiErr("BAD_REQUEST", "The offset and limit must be numbers."))
		return
	}
	res, err := m.Read(q.Get("conn"), q.Get("path"), offset, limit, q.Get("version"))
	if err != nil {
		m.fail(w, err)
		return
	}
	reply(w, 200, res)
}

func (m *Manager) httpDownload(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	name, size, rd, err := m.Open(q.Get("conn"), q.Get("path"))
	if err != nil {
		m.fail(w, err)
		return
	}
	defer func() { _ = rd.Close() }()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, rd); err != nil {
		m.log.Info("ssh: download stopped", "err", err)
	}
}

func (m *Manager) httpWrite(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, files.MaxFile+1))
	if err != nil {
		m.fail(w, apiErr("FILE_TOO_LARGE", "The file is over 10 MB."))
		return
	}
	id := q.Get("id")
	if id == "" {
		id = r.Header.Get("Mooring-Command-Id")
	}
	sum := sha256.Sum256(body)
	args, _ := json.Marshal(map[string]string{
		"conn": q.Get("conn"), "path": q.Get("path"), "base": q.Get("base"), "sha": hex.EncodeToString(sum[:]),
	})
	call := m.Ledger.Submit(id, "ssh.fileWrite", args, false, func(context.Context) (any, error) {
		return m.Write(q.Get("conn"), q.Get("path"), q.Get("base"), body)
	})
	ctx, cancel := context.WithTimeout(r.Context(), ledger.PendingAfter)
	defer cancel()
	out, ok := call.Wait(ctx, ledger.PendingAfter, func() {})
	if !ok {
		if r.Context().Err() == nil {
			reply(w, http.StatusAccepted, map[string]any{"ok": false, "error": apiErr("PENDING", "The save is still running. Send the same id again.")})
		}
		return
	}
	if !out.OK {
		m.fail(w, out.Error)
		return
	}
	reply(w, 200, map[string]any{"ok": true, "value": out.Value})
}
