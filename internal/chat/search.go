package chat

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Search limits.
const (
	searchMaxQuery = 200 // characters of query text, as the search box counts them
	searchDefault  = 30  // chats per answer
	searchMax      = 50
	searchRows     = 400 // index rows read before grouping them by chat
)

// Hit marks in a snippet. Control characters never come from a chat message's
// text, so the browser can split on them and show the words without HTML.
const (
	markOpen  = "\x02"
	markClose = "\x03"
)

// SearchHit is one chat that matches a search.
type SearchHit struct {
	ChatID    string `json:"chatId"`
	ProjectID string `json:"projectId"`
	Title     string `json:"title"`
	Agent     string `json:"agent"`
	State     string `json:"state"`
	UpdatedAt int64  `json:"updatedAt"`
	Snippet   string `json:"snippet"` // the best matching passage, hits between \x02 and \x03
}

// ftsQuery turns what the user typed into an FTS5 query: every word must be
// present, the last one may be the start of a word. Words are quoted, so FTS5
// syntax (OR, NEAR, *, column filters) in the input is matched as plain text.
func ftsQuery(q string) string {
	words := strings.Fields(q)
	parts := make([]string, 0, len(words))
	for i, w := range words {
		p := `"` + strings.ReplaceAll(w, `"`, `""`) + `"`
		if i == len(words)-1 {
			p += "*"
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, " ")
}

// Search finds chats in every project whose title or messages hold all the words
// of q. The best match comes first. An empty q lists the newest chats.
func (m *Manager) Search(q string, limit int) ([]SearchHit, error) {
	if limit <= 0 || limit > searchMax {
		limit = searchDefault
	}
	if utf8.RuneCountInString(q) > searchMaxQuery {
		return nil, apiErr("BAD_REQUEST", "The search is too long. Use at most %d characters.", searchMaxQuery)
	}
	infos := m.infos()
	if strings.TrimSpace(q) == "" {
		sort.Slice(infos, func(i, j int) bool { return infos[i].UpdatedAt > infos[j].UpdatedAt })
		out := make([]SearchHit, 0, min(limit, len(infos)))
		for _, in := range infos[:min(limit, len(infos))] {
			out = append(out, hitOf(in, ""))
		}
		return out, nil
	}
	byID := make(map[string]Info, len(infos))
	for _, in := range infos {
		byID[in.ID] = in
	}
	out := []SearchHit{}
	seen := map[string]bool{}
	add := func(id, snippet string) {
		in, ok := byID[id]
		if !ok || seen[id] || len(out) >= limit {
			return
		}
		seen[id] = true
		out = append(out, hitOf(in, snippet))
	}
	for _, in := range infos {
		if titleHas(in.Title, q) {
			add(in.ID, "")
		}
	}
	rows, err := m.db.Query(`SELECT r.chat_id, snippet(chat_search, 0, ?, ?, '…', 14)
		FROM chat_search JOIN chat_search_rows r ON r.id = chat_search.rowid
		WHERE chat_search MATCH ? ORDER BY rank LIMIT ?`, markOpen, markClose, ftsQuery(q), searchRows)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() && len(out) < limit {
		var id, snip string
		if err := rows.Scan(&id, &snip); err != nil {
			return nil, err
		}
		if seen[id] && out[indexOf(out, id)].Snippet == "" {
			out[indexOf(out, id)].Snippet = snip // a title match gets the passage too
			continue
		}
		add(id, snip)
	}
	return out, rows.Err()
}

// infos is every chat's Info, in no order.
func (m *Manager) infos() []Info {
	m.mu.Lock()
	all := make([]*Chat, 0, len(m.chats))
	for _, c := range m.chats {
		all = append(all, c)
	}
	m.mu.Unlock()
	out := make([]Info, 0, len(all))
	for _, c := range all {
		out = append(out, c.Info())
	}
	return out
}

func indexOf(hits []SearchHit, id string) int {
	for i, h := range hits {
		if h.ChatID == id {
			return i
		}
	}
	return -1
}

// titleHas reports whether every word of q is in title, ignoring case.
func titleHas(title, q string) bool {
	t := strings.ToLower(title)
	for _, w := range strings.Fields(strings.ToLower(q)) {
		if !strings.Contains(t, w) {
			return false
		}
	}
	return true
}

func hitOf(in Info, snippet string) SearchHit {
	return SearchHit{ChatID: in.ID, ProjectID: in.ProjectID, Title: in.Title, Agent: in.Agent,
		State: in.State, UpdatedAt: in.UpdatedAt, Snippet: snippet}
}

// SearchHandler serves GET /api/chats/search?q=&limit=. The caller checks the session first.
func (m *Manager) SearchHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		hits, err := m.Search(r.URL.Query().Get("q"), limit)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err != nil {
			status, le := http.StatusBadRequest, &ledger.Error{}
			if !errors.As(err, &le) {
				m.log.Error("chat: search", "err", err)
				status, le = http.StatusInternalServerError, &ledger.Error{Code: "INTERNAL", Message: "The search failed on the server. Try again."}
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": le.Code, "message": le.Message}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"hits": hits})
	})
}
