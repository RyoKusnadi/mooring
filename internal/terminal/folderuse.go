package terminal

import "path/filepath"

// FolderUse lists the real folders in which the running local terminals of a
// project were started. A terminal that has since run `cd` elsewhere still
// counts by its start folder: the shell's own folder cannot be read portably,
// and over-reporting only makes removing a worktree ask for a close first.
func (m *Manager) FolderUse(projectID string) []string {
	m.mu.Lock()
	terms := make([]*Terminal, 0, len(m.terms))
	for _, t := range m.terms {
		if t.projectID == projectID && t.startDir != "" {
			terms = append(terms, t)
		}
	}
	m.mu.Unlock()
	var out []string
	for _, t := range terms {
		if t.info().Exited {
			continue
		}
		dir := t.startDir
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			dir = real
		}
		out = append(out, dir)
	}
	return out
}
