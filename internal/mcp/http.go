package mcp

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Mount registers the routes. wrap adds the session check; the sign-in callback
// is left without it on purpose (see Callback).
func (s *Service) Mount(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /api/toolservers", wrap(http.HandlerFunc(s.httpList)))
	mux.Handle("GET /api/toolservers/catalog", wrap(http.HandlerFunc(s.httpCatalog)))
	mux.HandleFunc("GET "+callbackPath, s.Callback)
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
		s.log.Error("tool server: request failed", "err", err)
		le = apiErr("INTERNAL", "Something went wrong on the server.")
	}
	reply(w, 500, map[string]any{"ok": false, "error": le})
}

func (s *Service) httpList(w http.ResponseWriter, _ *http.Request) {
	list, err := s.List()
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, map[string]any{"servers": list, "canSaveSecrets": s.CanSaveSecrets()})
}

func (s *Service) httpCatalog(w http.ResponseWriter, r *http.Request) {
	cat, err := s.reg.get()
	if err != nil {
		s.fail(w, apiErr("CATALOG_FAILED", "The catalog could not be loaded."))
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("cursor"))
	page, next := cat.search(r.URL.Query().Get("q"), offset, catalogPage)
	out := make([]Entry, len(page))
	for i, e := range page {
		e.Kinds = slices.Clone(e.Kinds)
		for j := range e.Kinds {
			k := &e.Kinds[j]
			k.Available = true
			if k.Command != "" {
				if _, err := s.LookPath(k.Command); err != nil {
					k.Available, k.Missing = false, k.Command
				}
			}
		}
		out[i] = e
	}
	var nextv any
	if next >= 0 {
		nextv = next
	}
	reply(w, 200, map[string]any{"entries": out, "next": nextv, "date": cat.date})
}
