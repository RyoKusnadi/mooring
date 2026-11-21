package git

import (
	"context"
	"strconv"
	"strings"

	"github.com/ryokusnadi/mooring/internal/paths"
)

const terminalOpen = "A terminal is open in this worktree. Close it first."

// Terminals tells in which folders terminals were started.
type Terminals interface {
	// FolderUse lists the real start folders of the running terminals of a project.
	FolderUse(projectID string) []string
}

// SetTerminals says where to ask about terminals. Call it before serving.
func (s *Service) SetTerminals(t Terminals) {
	s.mu.Lock()
	s.terms = t
	s.mu.Unlock()
}

// terminalIn reports whether a running terminal was started in root or below it.
// A terminal that has since changed folder still counts by its start folder;
// one that moved into root from elsewhere is not seen.
func (s *Service) terminalIn(projectID, root string) bool {
	s.mu.Lock()
	t := s.terms
	s.mu.Unlock()
	if t == nil {
		return false
	}
	for _, dir := range t.FolderUse(projectID) {
		if paths.Inside(root, dir) {
			return true
		}
	}
	return false
}

// commitsOffRemotes counts the commits of HEAD in dir that no remote branch has.
// ok is false when git could not tell.
func (s *Service) commitsOffRemotes(ctx context.Context, dir string) (n int, ok bool) {
	out, _, err := s.runX(ctx, dir, false, nil, 0, nil, "rev-list", "--count", "HEAD", "--not", "--remotes")
	if err != nil {
		return 0, false
	}
	n, err = strconv.Atoi(strings.TrimSpace(string(out)))
	return n, err == nil
}

// unpushed counts the commits of the worktree's HEAD that no remote branch has.
// Removing the worktree keeps its branch, so this is a warning, not a stop.
// A repository with no remote answers 0: there is nowhere to have pushed to.
func (s *Service) unpushed(ctx context.Context, root string) int {
	if out, err := s.run(ctx, root, false, nil, "remote"); err != nil || strings.TrimSpace(string(out)) == "" {
		return 0
	}
	n, _ := s.commitsOffRemotes(ctx, root)
	return n
}
