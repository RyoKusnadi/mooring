package git

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ryokusnadi/mooring/internal/events"
)

// Handles reports whether name is a git command.
func Handles(name string) bool { return strings.HasPrefix(name, "git.") }

type cmdArgs struct {
	ProjectID   string   `json:"projectId"`
	Folder      string   `json:"folder"`
	Paths       []string `json:"paths"`
	Permanent   bool     `json:"permanent"`
	Message     string   `json:"message"`
	SetUpstream bool     `json:"setUpstream"`
	Branch      string   `json:"branch"`
	Name        string   `json:"name"`
	ID          string   `json:"id"`
	Untracked   bool     `json:"untracked"`
	Path        string   `json:"path"`
	Staged      bool     `json:"staged"`
	Version     string   `json:"version"`
	Lines       [][2]int `json:"lines"`
	Amend       bool     `json:"amend"`
	Side        string   `json:"side"`     // git.resolve: ours, theirs or mark
	How         string   `json:"how"`      // git.finish: abort or continue
	ChatID      string   `json:"chatId"`   // git.lastTurn, git.turnDiff
	OrigPath    string   `json:"origPath"` // git.turnDiff: the old path of a renamed file
}

// Run runs a git command. Errors are *ledger.Error.
func (s *Service) Run(ctx context.Context, cmd events.Command) (any, error) {
	var a cmdArgs
	if err := json.Unmarshal(cmd.Args, &a); err != nil {
		return nil, apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	empty := map[string]any{}
	switch cmd.Name {
	case "git.stage":
		return empty, s.Stage(ctx, a.ProjectID, a.Folder, a.Paths)
	case "git.unstage":
		return empty, s.Unstage(ctx, a.ProjectID, a.Folder, a.Paths)
	case "git.discard":
		return empty, s.Discard(ctx, a.ProjectID, a.Folder, a.Paths, a.Permanent)
	case "git.commit":
		return s.commit(ctx, a.ProjectID, a.Folder, a.Message, a.Amend)
	case "git.fetch":
		return s.Fetch(ctx, a.ProjectID, a.Folder)
	case "git.pull":
		return s.Pull(ctx, a.ProjectID, a.Folder)
	case "git.push":
		return s.Push(ctx, a.ProjectID, a.Folder, a.SetUpstream)
	case "git.worktreeAdd":
		return s.WorktreeAdd(ctx, a.ProjectID, a.Branch)
	case "git.worktreeRemove":
		return empty, s.WorktreeRemove(ctx, a.ProjectID, a.Folder)
	case "git.switch":
		return s.Switch(ctx, a.ProjectID, a.Folder, a.Branch)
	case "git.branchCreate":
		return s.BranchCreate(ctx, a.ProjectID, a.Folder, a.Name)
	case "git.stashSave":
		return s.StashSave(ctx, a.ProjectID, a.Folder, a.Message, a.Untracked)
	case "git.stashApply":
		return empty, s.StashApply(ctx, a.ProjectID, a.Folder, a.ID)
	case "git.stashPop":
		return empty, s.StashPop(ctx, a.ProjectID, a.Folder, a.ID)
	case "git.stashDrop":
		return empty, s.StashDrop(ctx, a.ProjectID, a.Folder, a.ID)
	case "git.stageLines":
		return empty, s.StageLines(ctx, a.ProjectID, a.Folder, a.Path, a.Staged, a.Version, a.Lines)
	case "git.resolve":
		return empty, s.Resolve(ctx, a.ProjectID, a.Folder, a.Paths, a.Side)
	case "git.lastTurn":
		return s.LastTurn(ctx, a.ProjectID, a.ChatID)
	case "git.turnDiff":
		return s.TurnDiff(ctx, a.ProjectID, a.ChatID, a.Path, a.OrigPath)
	case "git.finish":
		return s.Finish(ctx, a.ProjectID, a.Folder, a.How)
	}
	return nil, apiErr("BAD_REQUEST", "Unknown command %q.", cmd.Name)
}
