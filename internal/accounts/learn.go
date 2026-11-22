package accounts

import (
	"encoding/json"
	"sort"
)

// What chats tell Mooring about an account. The id may be "" for the agent's own login.

func canon(agent, id string) string {
	if id == "" {
		return BuiltinID(agent)
	}
	return id
}

// update reads the account, lets fn change it, and saves and announces it when
// something changed.
func (s *Service) update(agent, id string, fn func(r *row, now int64)) {
	if !s.Has(agent) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.get(canon(agent, id))
	if err != nil {
		return
	}
	before, _ := json.Marshal([]any{r.login, r.usage})
	fn(&r, s.now().UnixMilli())
	after, _ := json.Marshal([]any{r.login, r.usage})
	if string(before) == string(after) {
		return
	}
	if err := s.put(r); err != nil {
		s.log.Error("accounts: save learned state", "id", r.id, "err", err)
		return
	}
	s.announce(r)
}

// Ran records that a turn ran: the account is signed in and its limit, if any, is over.
func (s *Service) Ran(agent, id string) {
	s.update(agent, id, func(r *row, _ int64) {
		r.login = LoginSignedIn
		r.usage.LimitHit, r.usage.LimitResetsAt = 0, 0
	})
}

// NeedsLogin records that the agent said it is not signed in.
func (s *Service) NeedsLogin(agent, id string) {
	s.update(agent, id, func(r *row, _ int64) { r.login = LoginNeeded })
}

// RateLimit reads the agent's rate-limit object from a usage update. It reports
// whether the window says the agent refuses turns now, and when that ends (ms).
func (s *Service) RateLimit(agent, id string, raw json.RawMessage) (rejected bool, resetsAt int64) {
	a, ok := s.Adapters[agent]
	if !ok || a.Usage == "" {
		return false, 0
	}
	w, ok := readWindow(a.Usage, raw, s.now().UnixMilli())
	if !ok {
		return false, 0
	}
	s.update(agent, id, func(r *row, now int64) { r.usage.merge(w, now) })
	return w.Status == "rejected", w.ResetsAt
}

// LimitHit records that the agent refused a turn for a limit. An agent that
// names a limit knows the account, so it is signed in.
func (s *Service) LimitHit(agent, id string, resetsAt int64) {
	s.update(agent, id, func(r *row, now int64) {
		r.login = LoginSignedIn
		r.usage.LimitHit, r.usage.LimitResetsAt = now, boundReset(float64(resetsAt), now)
		if r.usage.Source == "" {
			r.usage.Source = "unknown"
		}
	})
}

// IsLimitText reports whether an error text from the agent says a limit was hit.
func (s *Service) IsLimitText(agent, text string) bool {
	a, ok := s.Adapters[agent]
	return ok && a.Usage != "" && LimitText(text)
}

// SwitchFor is the account a chat could move to after a limit: signed in, same
// agent, not at a limit, with the most room (the lowest use in its tightest
// window; unknown use after known). "" when there is none.
func (s *Service) SwitchFor(agent, current string) string {
	rows, err := s.list()
	if err != nil {
		return ""
	}
	now := s.now().UnixMilli()
	var best []row
	for _, r := range rows {
		if r.agent == agent && r.id != canon(agent, current) && r.login == LoginSignedIn && !r.usage.limited(now) {
			best = append(best, r)
		}
	}
	if len(best) == 0 {
		return ""
	}
	sort.SliceStable(best, func(i, j int) bool {
		a, b := best[i].usage.tightest(), best[j].usage.tightest()
		if (a < 0) != (b < 0) {
			return a >= 0
		}
		return a < b
	})
	return best[0].id
}
