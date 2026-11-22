package server

import (
	"context"
	"encoding/json"

	"github.com/ryokusnadi/mooring/internal/accounts"
	"github.com/ryokusnadi/mooring/internal/chat"
	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/files"
	"github.com/ryokusnadi/mooring/internal/git"
	"github.com/ryokusnadi/mooring/internal/lsp"
	"github.com/ryokusnadi/mooring/internal/mcp"
	"github.com/ryokusnadi/mooring/internal/password"
	"github.com/ryokusnadi/mooring/internal/projects"
	"github.com/ryokusnadi/mooring/internal/settings"
	"github.com/ryokusnadi/mooring/internal/ssh"
	"github.com/ryokusnadi/mooring/internal/terminal"
)

// commands sends terminal commands to the terminal manager, chat commands to the
// chat manager, file commands to files and the rest to projects.
type commands struct {
	projects *projects.Service
	terms    *terminal.Manager
	chats    *chat.Manager
	files    *files.Service
	git      *git.Service
	lsp      *lsp.Service
	ssh      *ssh.Manager
	tools    *mcp.Service
	accounts *accounts.Service
	settings *settings.Service
	password *password.Service
}

func (c commands) Secret(name string) bool {
	return ssh.Secret(name) || password.Secret(name) || mcp.Secret(name) || c.projects.Secret(name)
}

func (c commands) Run(ctx context.Context, cmd events.Command) (any, error) {
	if terminal.Handles(cmd.Name) {
		return c.terms.Run(cmd)
	}
	if password.Handles(cmd.Name) {
		return c.password.Run(cmd)
	}
	if settings.Handles(cmd.Name) {
		return c.settings.Run(cmd)
	}
	if accounts.Handles(cmd.Name) {
		return c.accounts.Run(cmd)
	}
	if chat.Handles(cmd.Name) {
		return c.chats.Run(cmd)
	}
	if files.Handles(cmd.Name) {
		return c.files.Run(cmd)
	}
	if git.Handles(cmd.Name) {
		return c.git.Run(ctx, cmd)
	}
	if ssh.Handles(cmd.Name) {
		return c.ssh.Run(ctx, cmd)
	}
	if mcp.Handles(cmd.Name) {
		return c.tools.Run(ctx, cmd)
	}
	if lsp.Handles(cmd.Name) {
		return c.lsp.Run(ctx, cmd)
	}
	v, err := c.projects.Run(ctx, cmd)
	if err == nil && cmd.Name == "project.remove" {
		var a struct{ ID string }
		if json.Unmarshal(cmd.Args, &a) == nil {
			c.terms.CloseProject(a.ID)
			c.chats.CloseProject(a.ID)
			c.files.CloseProject(a.ID)
			c.lsp.CloseProject(a.ID)
		}
	}
	return v, err
}

func (c commands) Snapshot() (any, error) {
	v, err := c.projects.Snapshot()
	if err != nil {
		return nil, err
	}
	if m, ok := v.(map[string]any); ok {
		m["terminals"] = c.terms.Snapshot()
		m["chats"], m["agents"] = c.chats.Snapshot()
		for k, v := range c.ssh.Snapshot() {
			m[k] = v
		}
	}
	return v, nil
}
