package files

import (
	"encoding/json"
	"strings"

	"github.com/ryokusnadi/mooring/internal/events"
)

// Handles reports whether name is a files command.
func Handles(name string) bool { return strings.HasPrefix(name, "file.") }

type cmdArgs struct {
	WatchArgs
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	From      string `json:"from"`
	To        string `json:"to"`
	Permanent bool   `json:"permanent"`
}

// Run runs a files command. Errors are *ledger.Error.
func (s *Service) Run(cmd events.Command) (any, error) {
	var a cmdArgs
	if err := json.Unmarshal(cmd.Args, &a); err != nil {
		return nil, apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	switch cmd.Name {
	case "file.create":
		return s.Create(a.ProjectID, a.Path, a.Kind)
	case "file.rename":
		return s.Rename(a.ProjectID, a.From, a.To)
	case "file.delete":
		return s.Delete(a.ProjectID, a.Path, a.Permanent)
	case "file.watch":
		return s.Watch(a.WatchArgs)
	case "file.copy":
		return s.Copy(a.ProjectID, a.From, a.To)
	}
	return nil, apiErr("BAD_REQUEST", "Unknown command %q.", cmd.Name)
}
