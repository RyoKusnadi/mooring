package lsp

import (
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Server describes one language server Mooring can start.
type Server struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Endings     []string `json:"endings"`
	LanguageID  string   `json:"languageId"`
	State       string   `json:"state"`
	Command     []string `json:"command"`
	Path        string   `json:"path"`
	Installable bool     `json:"installable"`
	InstallHint string   `json:"installHint"`
	Custom      bool     `json:"custom"`
}

// States of a Server.
const (
	StateReady        = "ready"
	StateNeedsConsent = "needs_consent"
	StateNotInstalled = "not_installed"
	StateInstalling   = "installing"
)

// resolved is a server definition and what it would run.
type resolved struct {
	def   Def
	path  string   // "" when not found
	argv  []string // the full path and arguments when found, else the catalog command
	state string
}

// defs lists the catalog with custom entries applied: an override replaces the
// catalog entry in place, a new id is added at the end.
func (s *Service) defs() ([]Def, error) {
	custom, err := s.loadCustom()
	if err != nil {
		return nil, err
	}
	var out []Def
	for _, d := range Catalog() {
		if c, ok := custom[d.ID]; ok {
			d = c
			delete(custom, d.ID)
		}
		out = append(out, d)
	}
	ids := make([]string, 0, len(custom))
	for id := range custom {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		out = append(out, custom[id])
	}
	return out, nil
}

func (s *Service) def(id string) (Def, bool, error) {
	ds, err := s.defs()
	if err != nil {
		return Def{}, false, err
	}
	for _, d := range ds {
		if d.ID == id {
			return d, true, nil
		}
	}
	return Def{}, false, nil
}

func (s *Service) resolve(d Def) resolved {
	r := resolved{def: d, argv: slices.Clone(d.Command)}
	p, ok := find(d.Command[0], s.searchDirs())
	if !ok {
		r.state = StateNotInstalled
		s.mu.Lock()
		if s.installing[d.ID] {
			r.state = StateInstalling
		}
		s.mu.Unlock()
		return r
	}
	r.path = p
	r.argv[0] = p
	r.state = StateNeedsConsent
	if allowed, ok := s.allowedCommand(d.ID); ok && slices.Equal(allowed, r.argv) {
		r.state = StateReady
	}
	return r
}

func (s *Service) view(r resolved) Server {
	d := r.def
	v := Server{ID: d.ID, Name: d.Name, Endings: d.Endings, LanguageID: d.LanguageID, State: r.state,
		Command: r.argv, Path: r.path, Custom: d.Custom}
	if v.Endings == nil {
		v.Endings = []string{}
	}
	switch {
	case d.Install != nil && s.cfg.DataDir != "":
		v.Installable = true
		v.InstallHint = installLine(d.Install)
	case d.Hint != "":
		v.InstallHint = d.Hint
	default:
		v.InstallHint = "Install " + d.Command[0] + " yourself or fix the command, then try again."
	}
	return v
}

func installLine(in *Install) string {
	switch in.Kind {
	case "go":
		return "go install " + strings.Join(in.Args, " ")
	case "npm":
		return "npm install " + strings.Join(in.Args, " ")
	}
	return "rustup component add " + strings.Join(in.Args, " ")
}

var customID = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)
