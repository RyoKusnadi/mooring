package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Mount registers the files routes. wrap adds the session check.
func (s *Service) Mount(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /api/files/list", wrap(http.HandlerFunc(s.httpList)))
	mux.Handle("GET /api/files/read", wrap(http.HandlerFunc(s.httpRead)))
	mux.Handle("GET /api/files/raw", wrap(http.HandlerFunc(s.httpRaw))) // GET patterns also match HEAD
	mux.Handle("PUT /api/files/content", wrap(http.HandlerFunc(s.httpWrite)))
	mux.Handle("GET /api/files/search", wrap(http.HandlerFunc(s.httpSearch)))
	mux.Handle("GET /api/files/names", wrap(http.HandlerFunc(s.httpNames)))
	mux.Handle("GET /api/files/download", wrap(http.HandlerFunc(s.httpDownload)))
	mux.Handle("PUT /api/files/upload", wrap(http.HandlerFunc(s.httpUpload)))
}

var statusOf = map[string]int{
	"BAD_REQUEST": 400, "BAD_QUERY": 400, "NOT_A_FOLDER": 400, "IS_A_FOLDER": 400, "TRASH_UNAVAILABLE": 409, "NOT_A_FILE": 400, "PROTECTED": 400,
	"OUTSIDE_PROJECT": 403, "NOT_FOUND": 404, "FILE_CHANGED": 409, "ALREADY_EXISTS": 409,
	"COMMAND_CONFLICT": 409, "OUTCOME_UNKNOWN": 409, "FILE_TOO_LARGE": 413, "NOT_AN_IMAGE": 415, "SEARCH_BUSY": 429,
}

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Service) fail(w http.ResponseWriter, err error) {
	var le *ledger.Error
	if !errors.As(err, &le) {
		s.log.Error("files: request failed", "err", err)
		le = apiErr("INTERNAL", "Something went wrong on the server.")
	}
	st := statusOf[le.Code]
	if st == 0 {
		st = 500
	}
	reply(w, st, map[string]any{"ok": false, "error": le})
}

func (s *Service) httpList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := s.List(q.Get("project"), q.Get("path"))
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, res)
}

func (s *Service) httpNames(w http.ResponseWriter, r *http.Request) {
	res, err := s.Names(r.Context(), r.URL.Query().Get("project"))
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, res)
}

func (s *Service) httpRead(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, err1 := strconv.ParseInt(orZero(q.Get("offset")), 10, 64)
	limit, err2 := strconv.Atoi(orZero(q.Get("limit")))
	if err1 != nil || err2 != nil {
		s.fail(w, apiErr("BAD_REQUEST", "The offset and limit must be numbers."))
		return
	}
	res, err := s.Read(q.Get("project"), q.Get("path"), offset, limit, q.Get("version"))
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, res)
}

func orZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

func (s *Service) httpWrite(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxFile+1))
	if err != nil {
		s.fail(w, apiErr("FILE_TOO_LARGE", "The file is over 10 MB."))
		return
	}
	sum := sha256.Sum256(body)
	args, _ := json.Marshal(map[string]string{
		"projectId": q.Get("project"), "path": q.Get("path"), "base": q.Get("base"), "sha": hex.EncodeToString(sum[:]),
	})
	call := s.cfg.Ledger.Submit(r.Header.Get("Mooring-Command-Id"), "file.write", args, false,
		func(context.Context) (any, error) {
			return s.Write(q.Get("project"), q.Get("path"), q.Get("base"), body)
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
		s.fail(w, out.Error)
		return
	}
	reply(w, 200, map[string]any{"ok": true, "value": out.Value})
}

func (s *Service) httpSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	req := SearchRequest{
		ProjectID: q.Get("project"), Query: q.Get("q"), Regex: q.Get("regex") == "1", Case: q.Get("case") == "1",
		Glob: q.Get("glob"), Path: q.Get("path"), Limit: limit,
	}
	fl, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	started := false
	var last time.Time
	err := s.Search(r.Context(), req, func(l SearchLine) error {
		if !started {
			started = true
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.Header().Set("Cache-Control", "no-store")
			if l.T == "error" {
				if st := statusOf[l.Code]; st != 0 {
					w.WriteHeader(st)
				}
			}
		}
		if err := enc.Encode(l); err != nil {
			return err
		}
		if fl != nil && (l.T != "match" || time.Since(last) > 50*time.Millisecond) {
			fl.Flush()
			last = time.Now()
		}
		return nil
	})
	if err != nil && r.Context().Err() == nil {
		s.log.Warn("files: search stream", "err", err)
	}
}
