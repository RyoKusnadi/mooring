package mcp

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Source says where a saved server came from: a catalog entry, or {} when typed.
type Source struct {
	Registry string `json:"registry,omitempty"`
	Version  string `json:"version,omitempty"`
}

// Tool is one tool of a checked server.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToolServer is a saved tool server as the browser sees it. It holds no secret value.
type ToolServer struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Command      string   `json:"command"`
	Args         []string `json:"args"`
	Env          []Var    `json:"env"`
	Headers      []Var    `json:"headers"`
	URL          string   `json:"url"`
	AllowPrivate bool     `json:"allowPrivate"`
	Enabled      bool     `json:"enabled"`
	Allowed      bool     `json:"allowed"`
	Source       Source   `json:"source"`
	SignIn       string   `json:"signIn"`
	Expires      int64    `json:"expires,omitempty"`
	CheckedAt    int64    `json:"checkedAt"`
	Tools        []Tool   `json:"tools"`
	// Note says why a sign-in is needed when the default reason is not the true one. It
	// lives in memory only. Cached is set on the reply of a check that served a saved list.
	Note   string `json:"note,omitempty"`
	Cached bool   `json:"cached,omitempty"`
}

// rec is a row: the browser's view plus what stays on the server.
type rec struct {
	ToolServer
	allowedSig string
	checkOK    bool
}

func (r rec) remote() bool { return r.Kind == "http" || r.Kind == "sse" }

// sig identifies what the user consents to: the exact command line or address.
func (r rec) sig() string {
	h := sha256.New()
	for _, p := range append([]string{r.Kind, r.Command, r.URL}, r.Args...) {
		_, _ = fmt.Fprintf(h, "%d:%s;", len(p), p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// material identifies everything a check depends on.
func (r rec) material(secretsGiven bool) string {
	h := sha256.New()
	parts := []string{r.sig(), fmt.Sprint(r.AllowPrivate), fmt.Sprint(secretsGiven)}
	for _, v := range r.Env {
		parts = append(parts, "e"+v.Name)
	}
	for _, v := range r.Headers {
		parts = append(parts, "h"+v.Name)
	}
	for _, p := range parts {
		_, _ = fmt.Fprintf(h, "%d:%s;", len(p), p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

const serverColumns = `id, name, kind, command, args, env, headers, url, allow_private, enabled, allowed_sig, source, sign_in, expires, checked_at, check_ok, tools`

type scanner interface{ Scan(...any) error }

func scanRec(row scanner) (rec, error) {
	var r rec
	var args, env, headers, source, tools string
	err := row.Scan(&r.ID, &r.Name, &r.Kind, &r.Command, &args, &env, &headers, &r.URL, &r.AllowPrivate, &r.Enabled,
		&r.allowedSig, &source, &r.SignIn, &r.Expires, &r.CheckedAt, &r.checkOK, &tools)
	if err != nil {
		return rec{}, err
	}
	for _, f := range []struct {
		raw string
		to  any
	}{{args, &r.Args}, {env, &r.Env}, {headers, &r.Headers}, {source, &r.Source}, {tools, &r.Tools}} {
		if err := json.Unmarshal([]byte(f.raw), f.to); err != nil {
			return rec{}, fmt.Errorf("read saved tool server %s: %w", r.ID, err)
		}
	}
	r.Allowed = r.allowedSig != "" && r.allowedSig == r.sig()
	if r.Args == nil {
		r.Args = []string{}
	}
	if r.Env == nil {
		r.Env = []Var{}
	}
	if r.Headers == nil {
		r.Headers = []Var{}
	}
	if r.Tools == nil {
		r.Tools = []Tool{}
	}
	return r, nil
}

func (s *Service) get(id string) (rec, error) {
	r, err := scanRec(s.db.QueryRow(`SELECT `+serverColumns+` FROM tool_servers WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return rec{}, apiErr("NOT_FOUND", "That tool server is not saved.")
	}
	if err != nil {
		return rec{}, fmt.Errorf("read tool server: %w", err)
	}
	r.Note = s.note(id)
	return r, nil
}

func (s *Service) list() ([]rec, error) {
	rows, err := s.db.Query(`SELECT ` + serverColumns + ` FROM tool_servers ORDER BY position`)
	if err != nil {
		return nil, fmt.Errorf("list tool servers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []rec
	for rows.Next() {
		r, err := scanRec(rows)
		if err != nil {
			return nil, err
		}
		r.Note = s.note(r.ID)
		out = append(out, r)
	}
	return out, rows.Err()
}

// put inserts or replaces a row (the id decides).
func (s *Service) put(r rec, position int) error {
	enc := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	_, err := s.db.Exec(`INSERT INTO tool_servers (`+serverColumns+`, position)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, kind=excluded.kind, command=excluded.command, args=excluded.args,
		env=excluded.env, headers=excluded.headers, url=excluded.url, allow_private=excluded.allow_private,
		enabled=excluded.enabled, allowed_sig=excluded.allowed_sig, source=excluded.source, sign_in=excluded.sign_in,
		expires=excluded.expires, checked_at=excluded.checked_at, check_ok=excluded.check_ok, tools=excluded.tools`,
		r.ID, r.Name, r.Kind, r.Command, enc(r.Args), enc(r.Env), enc(r.Headers), r.URL, r.AllowPrivate, r.Enabled,
		r.allowedSig, enc(r.Source), r.SignIn, r.Expires, r.CheckedAt, r.checkOK, enc(r.Tools), position)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return apiErr("ALREADY_EXISTS", "A tool server called %q is already saved.", r.Name)
		}
		return fmt.Errorf("save tool server: %w", err)
	}
	return nil
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
