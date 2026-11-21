// Package settings keeps Mooring's few global settings: one row per key in the
// settings table, the value as JSON. Nothing secret is stored here.
package settings

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Limits on the values.
const (
	MinJobTokens     = 1000
	MaxJobTokens     = 100_000_000
	DefaultJobTokens = 200_000
	maxSmallModels   = 16
	maxModelID       = 200
)

var agentID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Settings is what the browser reads and saves.
type Settings struct {
	CompactDefault  bool              `json:"compactDefault"`
	SmallModel      map[string]string `json:"smallModel"` // agent id to model id; empty means jobs are off
	TitleJobs       bool              `json:"titleJobs"`
	CommitJobs      bool              `json:"commitJobs"`
	JobTokensPerDay int64             `json:"jobTokensPerDay"`
}

// Defaults are the settings of a fresh install.
func Defaults() Settings {
	return Settings{SmallModel: map[string]string{}, JobTokensPerDay: DefaultJobTokens}
}

// Publisher receives settings.changed.
type Publisher interface {
	Publish(kind string, data any)
}

// Config is what a Service needs.
type Config struct {
	DB  *sql.DB
	Pub Publisher // nil publishes nothing
	// KnownAgent says whether an agent id is in the catalog; nil accepts any id.
	KnownAgent func(id string) bool
	Log        *slog.Logger
}

// Service reads and saves settings.
type Service struct {
	Config
	mu sync.Mutex // one save at a time
}

// New returns the service.
func New(cfg Config) *Service {
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{Config: cfg}
}

func apiErr(code, format string, a ...any) *ledger.Error {
	return &ledger.Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

// Get reads the settings; a missing or unreadable row gives its default.
func (s *Service) Get() Settings {
	out := Defaults()
	rows, err := s.DB.Query(`SELECT key, value FROM settings`)
	if err != nil {
		s.Log.Warn("settings: read", "err", err)
		return out
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) != nil {
			continue
		}
		if err := assign(&out, k, json.RawMessage(v)); err != nil {
			s.Log.Warn("settings: ignoring a saved value", "key", k)
		}
	}
	if out.SmallModel == nil {
		out.SmallModel = map[string]string{}
	}
	return out
}

func assign(s *Settings, key string, raw json.RawMessage) error {
	switch key {
	case "compactDefault":
		return json.Unmarshal(raw, &s.CompactDefault)
	case "smallModel":
		return json.Unmarshal(raw, &s.SmallModel)
	case "titleJobs":
		return json.Unmarshal(raw, &s.TitleJobs)
	case "commitJobs":
		return json.Unmarshal(raw, &s.CommitJobs)
	case "jobTokensPerDay":
		return json.Unmarshal(raw, &s.JobTokensPerDay)
	}
	return nil
}

// Save applies the keys present in patch (a JSON object), checks the result and
// stores every key. Unknown keys are BAD_REQUEST.
func (s *Service) Save(patch json.RawMessage) (Settings, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(patch, &fields); err != nil || fields == nil {
		return Settings{}, apiErr("BAD_REQUEST", "The settings are not valid.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.Get()
	for k, raw := range fields {
		switch k {
		case "compactDefault", "smallModel", "titleJobs", "commitJobs", "jobTokensPerDay":
		default:
			return Settings{}, apiErr("BAD_REQUEST", "There is no setting called %q.", k)
		}
		if k == "smallModel" {
			next.SmallModel = nil // replaced as a whole, not merged
		}
		if err := assign(&next, k, raw); err != nil {
			return Settings{}, apiErr("BAD_REQUEST", "The value of %q is not valid.", k)
		}
	}
	if err := s.check(&next); err != nil {
		return Settings{}, err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return Settings{}, s.fail(err)
	}
	defer func() { _ = tx.Rollback() }()
	for k, v := range map[string]any{"compactDefault": next.CompactDefault, "smallModel": next.SmallModel,
		"titleJobs": next.TitleJobs, "commitJobs": next.CommitJobs, "jobTokensPerDay": next.JobTokensPerDay} {
		raw, _ := json.Marshal(v)
		if _, err := tx.Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, k, string(raw)); err != nil {
			return Settings{}, s.fail(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Settings{}, s.fail(err)
	}
	if s.Pub != nil {
		s.Pub.Publish("settings.changed", next)
	}
	return next, nil
}

func (s *Service) fail(err error) error {
	s.Log.Error("settings: save", "err", err)
	return apiErr("INTERNAL", "The settings could not be saved.")
}

func (s *Service) check(n *Settings) error {
	if n.SmallModel == nil {
		n.SmallModel = map[string]string{}
	}
	if len(n.SmallModel) > maxSmallModels {
		return apiErr("BAD_REQUEST", "At most %d agents can have a small model.", maxSmallModels)
	}
	for agent, model := range n.SmallModel {
		if !agentID.MatchString(agent) || (s.KnownAgent != nil && !s.KnownAgent(agent)) {
			return apiErr("BAD_REQUEST", "There is no agent called %q.", agent)
		}
		if strings.TrimSpace(model) == "" || utf8.RuneCountInString(model) > maxModelID || strings.IndexFunc(model, unicode.IsControl) >= 0 {
			return apiErr("BAD_REQUEST", "The small model of %s needs a model id of 1 to %d characters.", agent, maxModelID)
		}
	}
	if n.JobTokensPerDay < MinJobTokens || n.JobTokensPerDay > MaxJobTokens {
		return apiErr("BAD_REQUEST", "The daily job limit needs %d to %d tokens.", MinJobTokens, MaxJobTokens)
	}
	return nil
}

// Handles reports whether name is a settings command.
func Handles(name string) bool { return strings.HasPrefix(name, "settings.") }

// Run runs a settings command. Errors are *ledger.Error.
func (s *Service) Run(cmd events.Command) (any, error) {
	if cmd.Name != "settings.save" {
		return nil, apiErr("BAD_REQUEST", "Unknown command %q.", cmd.Name)
	}
	var a struct {
		Settings json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(cmd.Args, &a); err != nil || len(a.Settings) == 0 {
		return nil, apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	return s.Save(a.Settings)
}

// Mount serves GET /api/settings.
func (s *Service) Mount(mux *http.ServeMux, guard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/settings", guard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(s.Get())
	})))
}
