package lsp

import (
	"context"
	"encoding/json"
	"path"
	"slices"
	"strings"

	"github.com/ryokusnadi/mooring/internal/events"
)

// Handles reports whether name is a language server command.
func Handles(name string) bool { return strings.HasPrefix(name, "lsp.") }

type cmdArgs struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Endings    []string `json:"endings"`
	LanguageID string   `json:"languageId"`
	Command    []string `json:"command"`
	ProjectID  string   `json:"projectId"`
	Folder     string   `json:"folder"`
}

// Run runs a language server command. Errors are *ledger.Error.
func (s *Service) Run(ctx context.Context, cmd events.Command) (any, error) {
	var a cmdArgs
	if err := json.Unmarshal(cmd.Args, &a); err != nil {
		return nil, apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	switch cmd.Name {
	case "lsp.list":
		return s.List()
	case "lsp.allow":
		return s.Allow(a.ID, a.Command)
	case "lsp.install":
		return s.Install(ctx, a.ID)
	case "lsp.setCustom":
		return s.SetCustom(Def{ID: a.ID, Name: a.Name, Endings: a.Endings, LanguageID: a.LanguageID, Command: a.Command})
	case "lsp.removeCustom":
		return map[string]any{}, s.RemoveCustom(a.ID)
	case "lsp.stop":
		folder := path.Clean(a.Folder)
		if folder == "." {
			folder = ""
		}
		s.stopKey(instKey{a.ProjectID, folder, a.ID}, "closed")
		return map[string]any{}, nil
	}
	return nil, apiErr("BAD_REQUEST", "Unknown command %q.", cmd.Name)
}

// List is the value of lsp.list.
func (s *Service) List() (any, error) {
	ds, err := s.defs()
	if err != nil {
		return nil, err
	}
	servers := make([]Server, 0, len(ds))
	for _, d := range ds {
		servers = append(servers, s.view(s.resolve(d)))
	}
	return map[string]any{"servers": servers, "running": s.running()}, nil
}

// Allow records that the user accepts running command for server id.
func (s *Service) Allow(id string, command []string) (Server, error) {
	d, ok, err := s.def(id)
	if err != nil {
		return Server{}, err
	}
	if !ok {
		return Server{}, apiErr("NOT_FOUND", "There is no language server %q.", id)
	}
	r := s.resolve(d)
	if r.path == "" {
		return Server{}, apiErr("NOT_INSTALLED", "%s is not installed.", d.Command[0])
	}
	if !slices.Equal(command, r.argv) {
		return Server{}, apiErr("COMMAND_CHANGED", "The command is no longer the one you were shown. Look at it again.")
	}
	if err := s.saveConsent(id, r.argv); err != nil {
		s.log.Error("lsp: save consent", "server", id, "err", err)
		return Server{}, apiErr("INTERNAL", "Something went wrong on the server.")
	}
	r.state = StateReady
	v := s.view(r)
	s.publish("lsp.changed", map[string]any{"server": v})
	return v, nil
}

// SetCustom adds a server or overrides a catalog entry.
func (s *Service) SetCustom(d Def) (Server, error) {
	if !customID.MatchString(d.ID) {
		return Server{}, apiErr("BAD_REQUEST", "A server id is 1 to 32 letters (a-z), digits or -.")
	}
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" || len(d.Name) > 100 {
		return Server{}, apiErr("BAD_REQUEST", "Give the server a name of up to 100 characters.")
	}
	if d.LanguageID == "" || len(d.LanguageID) > 64 {
		return Server{}, apiErr("BAD_REQUEST", "Give the language id, for example \"python\".")
	}
	if len(d.Endings) == 0 || len(d.Endings) > 32 {
		return Server{}, apiErr("BAD_REQUEST", "Give 1 to 32 file endings, for example \".py\".")
	}
	for _, e := range d.Endings {
		if len(e) < 2 || len(e) > 16 || e[0] != '.' || strings.ContainsAny(e, "/\\ \x00") {
			return Server{}, apiErr("BAD_REQUEST", "A file ending starts with a dot, for example \".py\".")
		}
	}
	if len(d.Command) == 0 || len(d.Command) > 64 || strings.TrimSpace(d.Command[0]) == "" {
		return Server{}, apiErr("BAD_REQUEST", "Give the command as a program followed by its arguments.")
	}
	for _, a := range d.Command {
		if len(a) > 4096 || strings.ContainsRune(a, 0) {
			return Server{}, apiErr("BAD_REQUEST", "The command has an argument that is too long or not valid.")
		}
	}
	d.Custom = true
	if err := s.saveCustom(d); err != nil {
		s.log.Error("lsp: save custom server", "server", d.ID, "err", err)
		return Server{}, apiErr("INTERNAL", "Something went wrong on the server.")
	}
	s.stopServer(d.ID, "closed")
	v := s.view(s.resolve(d))
	s.publish("lsp.changed", map[string]any{"server": v})
	return v, nil
}

// RemoveCustom deletes a custom entry; a catalog entry it overrode comes back.
func (s *Service) RemoveCustom(id string) error {
	had, err := s.deleteCustom(id)
	if err != nil {
		s.log.Error("lsp: remove custom server", "server", id, "err", err)
		return apiErr("INTERNAL", "Something went wrong on the server.")
	}
	if !had {
		return apiErr("NOT_FOUND", "There is no custom server %q.", id)
	}
	s.stopServer(id, "closed")
	if d, ok, _ := s.def(id); ok {
		s.publish("lsp.changed", map[string]any{"server": s.view(s.resolve(d))})
	} else {
		s.publish("lsp.removed", map[string]any{"id": id})
	}
	return nil
}
