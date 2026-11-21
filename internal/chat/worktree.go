package chat

import "github.com/ryokusnadi/mooring/internal/paths"

// makeWorktree creates the worktree of a chat and stores its folder in the spec.
// The returned drop removes it again.
func (m *Manager) makeWorktree(projectID, projectDir string, spec *Spec) (string, func(), error) {
	if m.Worktrees == nil {
		return "", nil, apiErr("INTERNAL", "This server cannot make worktrees.")
	}
	folder, branch, err := m.Worktrees.WorktreeFolder(projectID, spec.Folder.Branch)
	if err != nil {
		return "", nil, err
	}
	drop := func() { m.Worktrees.DropWorktree(projectID, folder, branch) }
	dir, err := paths.Resolve(projectDir, folder)
	if err != nil {
		drop()
		return "", nil, apiErr("INTERNAL", "The new worktree could not be opened.")
	}
	spec.Folder.Path = dir
	return dir, drop, nil
}
