package terminal

import (
	"encoding/json"

	"github.com/ryokusnadi/mooring/internal/events"
)

// Handles reports whether name is a terminal command.
func Handles(name string) bool {
	return name == "terminal.open" || name == "terminal.close" || name == "terminal.seen"
}

// Run runs a terminal command. Errors are *ledger.Error.
func (m *Manager) Run(cmd events.Command) (any, error) {
	var a struct {
		ID        string `json:"id"`
		ProjectID string `json:"projectId"`
		Folder    string `json:"folder"`
		SSH       string `json:"ssh"`
		Cols      int    `json:"cols"`
		Rows      int    `json:"rows"`
		BellOnly  bool   `json:"bellOnly"`
	}
	if err := json.Unmarshal(cmd.Args, &a); err != nil {
		return nil, apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	switch cmd.Name {
	case "terminal.open":
		if a.SSH != "" {
			return m.OpenSSH(a.ProjectID, a.SSH, a.ID, a.Cols, a.Rows)
		}
		return m.Open(a.ProjectID, a.Folder, a.ID, a.Cols, a.Rows)
	case "terminal.close":
		m.Close(a.ID)
		return map[string]any{}, nil
	case "terminal.seen":
		m.Seen(a.ID, a.BellOnly)
		return map[string]any{}, nil
	}
	return nil, apiErr("BAD_REQUEST", "Unknown command %q.", cmd.Name)
}
