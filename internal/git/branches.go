package git

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/ryokusnadi/mooring/internal/paths"
)

// Branch is a local branch.
type Branch struct {
	Name     string `json:"name"`
	Head     bool   `json:"head"`
	Upstream string `json:"upstream"`
	Time     int64  `json:"time"`
}

// Branches is the answer of GET /api/git/branches.
type Branches struct {
	Current  string   `json:"current"`
	Branches []Branch `json:"branches"`
}

// ListBranches reads the local branches, newest commit first.
func (s *Service) ListBranches(ctx context.Context, projectID, folder string) (*Branches, error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	out, err := s.run(ctx, pl.top, false, nil, "for-each-ref", "--sort=-committerdate",
		"--format=%(refname:lstrip=2)%1f%(HEAD)%1f%(upstream:short)%1f%(committerdate:unix)", "refs/heads")
	if err != nil {
		return nil, s.failure(err)
	}
	res := &Branches{Branches: []Branch{}, Current: s.branchOf(ctx, pl)}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Split(line, "\x1f")
		if len(f) != 4 {
			continue
		}
		res.Branches = append(res.Branches, Branch{Name: f[0], Head: f[1] == "*", Upstream: f[2], Time: atoi64(f[3])})
	}
	return res, nil
}

// busy refuses a change to the files of this checkout while a chat runs in it.
// A chat in another worktree of the project does not count.
func (s *Service) busy(pl *place) error {
	wt := filepath.Join(pl.root, filepath.FromSlash(wtDir))
	for dir, u := range s.chatUse(pl.project) {
		if !u.Live || !paths.Inside(pl.top, dir) {
			continue
		}
		if paths.Inside(wt, dir) && !paths.Inside(wt, pl.top) {
			continue
		}
		return apiErr("BUSY", "A chat is running in this folder and its files would change under it. Stop the chat first.")
	}
	return nil
}

func (s *Service) hasBranch(ctx context.Context, pl *place, name string) (bool, error) {
	// show-ref takes only a full ref name, never a revision such as main^ or dev@{1}.
	out, _, err := s.runX(ctx, pl.top, false, nil, 0, []int{1, 128}, "show-ref", "--verify", "refs/heads/"+name)
	if err != nil {
		return false, s.failure(err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// Switch moves HEAD to a local branch.
func (s *Service) Switch(ctx context.Context, projectID, folder, name string) (map[string]string, error) {
	if name == "" || len(name) > 255 || strings.HasPrefix(name, "-") || strings.ContainsRune(name, 0) {
		return nil, apiErr("BAD_REQUEST", "That is not a branch name.")
	}
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	if err := s.busy(pl); err != nil {
		return nil, err
	}
	err = s.change(pl, func() error {
		ok, err := s.hasBranch(ctx, pl, name)
		if err != nil {
			return err
		}
		if !ok {
			return apiErr("NOT_FOUND", "There is no local branch %q.", name)
		}
		if _, err := s.run(ctx, pl.top, true, nil, "checkout", "-q", name, "--"); err != nil {
			return s.failure(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]string{"branch": name}, nil
}

// BranchCreate makes a branch at HEAD and switches to it.
func (s *Service) BranchCreate(ctx context.Context, projectID, folder, name string) (map[string]string, error) {
	if name == "" || len(name) > 255 || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "@{") || strings.ContainsRune(name, 0) {
		return nil, apiErr("BAD_REQUEST", "That is not a valid branch name.")
	}
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	if err := s.busy(pl); err != nil {
		return nil, err
	}
	err = s.change(pl, func() error {
		out, _, err := s.runX(ctx, pl.top, false, nil, 0, []int{1, 128}, "check-ref-format", "--branch", name)
		if err != nil || strings.TrimSpace(string(out)) != name {
			return apiErr("BAD_REQUEST", "Git does not accept %q as a branch name.", name)
		}
		if ok, err := s.hasBranch(ctx, pl, name); err != nil {
			return err
		} else if ok {
			return apiErr("ALREADY_EXISTS", "A branch named %q exists.", name)
		}
		if _, err := s.run(ctx, pl.top, true, nil, "checkout", "-q", "-b", name); err != nil {
			return s.failure(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]string{"branch": name}, nil
}
