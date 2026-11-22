package chat

import (
	"path/filepath"

	"github.com/ryokusnadi/mooring/internal/paths"
)

// extraDirs is what a chat may read besides its own folder: a worktree chat also
// reads the project folder, when the agent takes extra folders. Sent on every start.
func (c *Chat) extraDirs(cp caps) []string {
	c.mu.Lock()
	wt, dir, project := c.spec.Folder.Worktree, c.dir, c.projectDir
	c.mu.Unlock()
	if !cp.AdditionalDirectories || !wt || !filepath.IsAbs(project) || sameDir(project)(dir) {
		return nil
	}
	return []string{project}
}

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
