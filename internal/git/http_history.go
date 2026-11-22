package git

import (
	"net/http"
	"strconv"
)

func (s *Service) httpLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lq := LogQuery{Cursor: q.Get("cursor"), Text: q.Get("q"), In: q.Get("in")}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			s.fail(w, apiErr("BAD_REQUEST", "The page size is 1 to %d.", maxPage))
			return
		}
		lq.Limit = n
	}
	res, err := s.Log(r.Context(), q.Get("project"), q.Get("folder"), lq)
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, res)
}

func (s *Service) httpCommit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := s.CommitDetail(r.Context(), q.Get("project"), q.Get("folder"), q.Get("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, res)
}

func (s *Service) httpBranches(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := s.ListBranches(r.Context(), q.Get("project"), q.Get("folder"))
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, res)
}

func (s *Service) httpStashes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := s.Stashes(r.Context(), q.Get("project"), q.Get("folder"))
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, map[string]any{"stashes": list})
}

func (s *Service) httpBlame(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := s.Blame(r.Context(), q.Get("project"), q.Get("folder"), q.Get("path"))
	if err != nil {
		s.fail(w, err)
		return
	}
	reply(w, 200, res)
}
