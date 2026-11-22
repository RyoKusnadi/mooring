package mcp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ryokusnadi/mooring/internal/events"
)

// Handles reports whether name is a tool server command.
func Handles(name string) bool { return strings.HasPrefix(name, "toolServer.") }

// Secret is true for the commands whose args hold secret values or a sign-in code.
func Secret(name string) bool { return name == "toolServer.save" || name == "toolServer.signInFinish" }

// Run runs a tool server command. Errors are *ledger.Error.
func (s *Service) Run(ctx context.Context, cmd events.Command) (any, error) {
	var a struct {
		SaveArgs
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
		Fresh   bool   `json:"fresh"`
		URL     string `json:"url"`
	}
	if err := json.Unmarshal(cmd.Args, &a); err != nil {
		return nil, apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	switch cmd.Name {
	case "toolServer.save":
		return s.Save(ctx, a.SaveArgs)
	case "toolServer.remove":
		return map[string]any{}, s.Remove(a.ID)
	case "toolServer.enable":
		return s.Enable(a.ID, a.Enabled)
	case "toolServer.check":
		if a.Fresh {
			return s.CheckFresh(ctx, a.ID)
		}
		return s.Check(ctx, a.ID)
	case "toolServer.signIn":
		u, redirect, err := s.SignIn(ctx, a.ID)
		if err != nil {
			return nil, err
		}
		return map[string]string{"url": u, "redirect": redirect}, nil
	case "toolServer.signInFinish":
		return s.SignInFinish(ctx, a.ID, a.URL)
	case "toolServer.signOut":
		return s.SignOut(a.ID)
	case "toolServer.catalogUpdate":
		return s.CatalogUpdate(ctx)
	}
	return nil, apiErr("BAD_REQUEST", "Unknown command %s.", cmd.Name)
}
