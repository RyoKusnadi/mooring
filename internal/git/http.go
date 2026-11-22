package git

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Mount registers the git read routes. wrap adds the session check.
func (s *Service) Mount(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /api/git/status", wrap(http.HandlerFunc(s.httpStatus)))
	mux.Handle("GET /api/git/diff", wrap(http.HandlerFunc(s.httpDiff)))
	mux.Handle("GET /api/git/worktrees", wrap(http.HandlerFunc(s.httpWorktrees)))
	mux.Handle("GET /api/git/log", wrap(http.HandlerFunc(s.httpLog)))
	mux.Handle("GET /api/git/commit", wrap(http.HandlerFunc(s.httpCommit)))
	mux.Handle("GET /api/git/branches", wrap(http.HandlerFunc(s.httpBranches)))
	mux.Handle("GET /api/git/stashes", wrap(http.HandlerFunc(s.httpStashes)))
	mux.Handle("GET /api/git/blame", wrap(http.HandlerFunc(s.httpBlame)))
	mux.Handle("GET /api/git/image", wrap(http.HandlerFunc(s.httpImage))) // GET patterns also match HEAD
}

var statusOf = map[string]int{
	"BAD_REQUEST": 400, "OUTSIDE_PROJECT": 403, "NOT_FOUND": 404, "NOT_A_REPO": 409, "TIMEOUT": 504,
	"GIT_NOT_INSTALLED": 500, "BUSY": 409, "DIRTY": 409, "ALREADY_EXISTS": 409,
	"DIFF_CHANGED": 409, "LOG_CHANGED": 409, "NOTHING_TO_STASH": 409, "CONFLICTED": 409,
	"NOT_TRACKED": 409, "TOO_LARGE": 413, "NOT_AN_IMAGE": 415,
	"NOT_CONFLICTED": 409, "HAS_MARKERS": 409, "NO_OPERATION": 409, "NOTHING_TO_STAGE": 409,
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
		if errors.Is(err, context.Canceled) {
			return // the browser left; nobody reads the answer
		}
		s.log.Error("git: request failed", "err", err)
		le = apiErr("INTERNAL", "Something went wrong on the server.")
	}
	st := statusOf[le.Code]
	if st == 0 {
		st = 500
	}
	reply(w, st, map[string]any{"ok": false, "error": le})
}

func (s *Service) httpStatus(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := s.Status(r.Context(), q.Get("project"), q.Get("folder"))
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, res)
}

func (s *Service) httpDiff(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var res *Diff
	var err error
	if id := q.Get("commit"); id != "" {
		res, err = s.CommitDiff(r.Context(), q.Get("project"), q.Get("folder"), id, q.Get("path"))
	} else {
		res, err = s.Diff(r.Context(), q.Get("project"), q.Get("folder"), q.Get("path"), q.Get("staged") == "1")
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, res)
}

func (s *Service) httpWorktrees(w http.ResponseWriter, r *http.Request) {
	list, err := s.Worktrees(r.Context(), r.URL.Query().Get("project"))
	if le := (*ledger.Error)(nil); errors.As(err, &le) && le.Code == "NOT_A_REPO" {
		list, err = []Worktree{}, nil // a folder outside git has no worktrees; it is not a failure
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, map[string]any{"worktrees": list})
}
