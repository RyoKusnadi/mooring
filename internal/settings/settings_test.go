package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/store"
)

type pub struct {
	mu   sync.Mutex
	got  []string
	last any
}

func (p *pub) Publish(kind string, data any) {
	p.mu.Lock()
	p.got, p.last = append(p.got, kind), data
	p.mu.Unlock()
}

func newService(t *testing.T) (*Service, *pub, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	p := &pub{}
	return New(Config{DB: st.DB, Pub: p, KnownAgent: func(id string) bool { return id == "claude" || id == "fake" }}), p, st
}

func save(s *Service, body string) (any, error) {
	raw, _ := json.Marshal(map[string]json.RawMessage{"settings": json.RawMessage(body)})
	return s.Run(events.Command{Name: "settings.save", Args: raw})
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if e, ok := err.(*ledger.Error); !ok || e.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func TestDefaults(t *testing.T) {
	s, _, _ := newService(t)
	got := s.Get()
	if got.CompactDefault || got.TitleJobs || got.CommitJobs || len(got.SmallModel) != 0 || got.JobTokensPerDay != 200000 {
		t.Fatalf("defaults: %+v", got)
	}
	raw, _ := json.Marshal(got)
	if !strings.Contains(string(raw), `"smallModel":{}`) {
		t.Fatalf("smallModel must be an object: %s", raw)
	}
}

func TestSaveStoresOneJSONRowPerKeyAndPublishes(t *testing.T) {
	s, p, st := newService(t)
	if _, err := save(s, `{"compactDefault":true,"smallModel":{"fake":"fast"},"titleJobs":true}`); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := st.DB.QueryRow(`SELECT value FROM settings WHERE key = 'compactDefault'`).Scan(&raw); err != nil || raw != "true" {
		t.Fatalf("row %q: %v", raw, err)
	}
	got := s.Get()
	if !got.CompactDefault || !got.TitleJobs || got.CommitJobs || got.SmallModel["fake"] != "fast" {
		t.Fatalf("got %+v", got)
	}
	// A later save changes only the keys it names; smallModel is replaced whole.
	if _, err := save(s, `{"smallModel":{"claude":"haiku"},"commitJobs":true}`); err != nil {
		t.Fatal(err)
	}
	got = s.Get()
	if !got.TitleJobs || !got.CommitJobs || len(got.SmallModel) != 1 || got.SmallModel["claude"] != "haiku" {
		t.Fatalf("got %+v", got)
	}
	if len(p.got) != 2 || p.got[0] != "settings.changed" {
		t.Fatalf("events %v", p.got)
	}
}

func TestSaveRejectsBadValues(t *testing.T) {
	s, p, _ := newService(t)
	for _, body := range []string{
		`{"nope":1}`, `{"compactDefault":"yes"}`, `{"jobTokensPerDay":5}`, `{"jobTokensPerDay":999999999999}`,
		`{"smallModel":{"ghost":"x"}}`, `{"smallModel":{"fake":""}}`, `{"smallModel":{"fake":"a\nb"}}`, `[]`, `null`,
	} {
		_, err := save(s, body)
		wantCode(t, err, "BAD_REQUEST")
	}
	if len(p.got) != 0 || s.Get().JobTokensPerDay != 200000 {
		t.Fatalf("a refused save must change nothing: %v %+v", p.got, s.Get())
	}
}

func TestGetEndpoint(t *testing.T) {
	s, _, _ := newService(t)
	mux := http.NewServeMux()
	s.Mount(mux, func(h http.Handler) http.Handler { return h })
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/settings", nil))
	var got Settings
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.JobTokensPerDay != 200000 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestInstructionSettings(t *testing.T) {
	s, _, _ := newService(t)
	if g := s.Get(); !g.AgentInstructions || len(g.ProjectInstructions) != 0 {
		t.Fatalf("defaults: %+v", g)
	}
	if _, err := save(s, `{"agentInstructions":false,"projectInstructions":{"a":" one ","b":"two"}}`); err != nil {
		t.Fatal(err)
	}
	if _, err := save(s, `{"projectInstructions":{"b":""}}`); err != nil { // merges key by key; empty removes
		t.Fatal(err)
	}
	g := s.Get()
	if g.AgentInstructions || len(g.ProjectInstructions) != 1 || g.ProjectInstructions["a"] != "one" {
		t.Fatalf("saved: %+v", g)
	}
	if _, err := save(s, `{"projectInstructions":{"a":"`+strings.Repeat("x", MaxProjectInstructions+1)+`"}}`); err == nil {
		t.Fatal("a text over the cap was saved")
	}
	if s.Get().ProjectInstructions["a"] != "one" {
		t.Fatal("a refused save changed the text")
	}
}
