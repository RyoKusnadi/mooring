package git

import (
	"context"
	"strings"
)

const maxMessage = 64 << 10

// CommitResult is the value of git.commit.
type CommitResult struct {
	Commit string `json:"commit"`
	Branch string `json:"branch"`
}

// Commit commits what is staged. Hooks run.
func (s *Service) Commit(ctx context.Context, projectID, folder, message string) (*CommitResult, error) {
	return s.commit(ctx, projectID, folder, message, false)
}

// commit is Commit; with amend it rewrites the last commit with what is staged
// and the message (the old message stays when none is sent). It never rewrites
// a commit the upstream already has.
func (s *Service) commit(ctx context.Context, projectID, folder, message string, amend bool) (*CommitResult, error) {
	if amend && message == "" {
		// keep the old message
	} else if len(message) == 0 || len(message) > maxMessage || strings.TrimSpace(message) == "" || strings.ContainsRune(message, 0) {
		return nil, apiErr("BAD_REQUEST", "The message needs 1 to %d bytes and some text.", maxMessage)
	}
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	var res *CommitResult
	err = s.change(pl, func() error {
		st, err := s.statusOf(ctx, pl)
		if err != nil {
			return err
		}
		if st.Conflicted {
			return apiErr("CONFLICTED", "Some files have merge conflicts. Resolve them first.")
		}
		if _, _, err := s.runX(ctx, pl.top, false, nil, 0, nil, "-c", "user.useConfigOnly=true", "var", "GIT_AUTHOR_IDENT"); err != nil {
			return apiErr("NO_IDENTITY", "Git has no user.name and user.email. Set them with git config, then commit again.")
		}
		args := []string{"-c", "user.useConfigOnly=true", "commit", "-q", "-F", "-"}
		if amend {
			if err := s.amendable(ctx, pl); err != nil {
				return err
			}
			args = []string{"-c", "user.useConfigOnly=true", "commit", "-q", "--amend", "-F", "-"}
			if message == "" {
				args = args[:len(args)-2]
				args = append(args, "--no-edit")
			}
		} else if s.operation(ctx, pl) == "merge" {
			// A merge commit may keep HEAD's files as they are.
		} else if _, _, err := s.runX(ctx, pl.top, false, nil, 0, nil, "diff", "--cached", "--quiet", "--no-ext-diff"); err == nil {
			return apiErr("NOTHING_TO_COMMIT", "Nothing is staged. Stage some changes first.")
		} else if re, ok := err.(*runError); !ok || re.exit != 1 {
			return s.failure(err)
		}
		_, stderr, err := s.runX(ctx, pl.top, true, []byte(message), 0, nil, args...)
		if err != nil {
			if re, ok := err.(*runError); ok && re.exit == 1 {
				out := clean(re.stdout + "\n" + stderr)
				if strings.Contains(out, "identity unknown") || strings.Contains(out, "empty ident") {
					return apiErr("NO_IDENTITY", "Git has no user.name and user.email. Set them with git config, then commit again.")
				}
				return apiErr("HOOK_FAILED", "%s", out)
			}
			return s.failure(err)
		}
		res = &CommitResult{}
		if out, err := s.run(ctx, pl.top, false, nil, "rev-parse", "--short", "HEAD"); err == nil {
			res.Commit = strings.TrimSpace(string(out))
		}
		res.Branch = s.branchOf(ctx, pl)
		return nil
	})
	return res, err
}

// amendable refuses an amend when there is no commit, or the last commit is
// already on a remote branch: changing it would need a force push. This holds
// with no upstream and with a detached HEAD too.
func (s *Service) amendable(ctx context.Context, pl *place) error {
	if s.revOf(ctx, pl, "HEAD") == "" {
		return apiErr("BAD_REQUEST", "There is no commit to amend yet.")
	}
	if n, ok := s.commitsOffRemotes(ctx, pl.top); ok && n == 0 {
		where := "a remote branch"
		if up := s.upstreamOf(ctx, pl); up != "" {
			where = up
		}
		return apiErr("ALREADY_PUSHED", "The last commit is already on %s. Changing it would need a force push, which Mooring does not do.", where)
	}
	return nil
}

// branchOf is the current branch, "" when detached.
func (s *Service) branchOf(ctx context.Context, pl *place) string {
	out, _, err := s.runX(ctx, pl.top, false, nil, 0, []int{1}, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// PushResult is the value of git.push.
type PushResult struct {
	Upstream string `json:"upstream"`
	Output   string `json:"output"`
}

// Push pushes the current branch to its upstream; with setUpstream it first
// picks the remote and sets the upstream. It never forces.
func (s *Service) Push(ctx context.Context, projectID, folder string, setUpstream bool) (*PushResult, error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	var res *PushResult
	err = s.change(pl, func() error {
		ctx := s.netCtx(ctx, pl)
		branch := s.branchOf(ctx, pl)
		if branch == "" {
			return apiErr("BAD_REQUEST", "A detached HEAD cannot be pushed. Switch to a branch first.")
		}
		upstream := s.upstreamOf(ctx, pl)
		var args []string
		if upstream != "" {
			// An explicit refspec without "+": a remote.*.push or mirror setting in the
			// user's config cannot turn this push into a forced one.
			remote, merge := s.config(ctx, pl, "branch."+branch+".remote"), s.config(ctx, pl, "branch."+branch+".merge")
			if remote == "" || merge == "" {
				return apiErr("NO_UPSTREAM", "This branch's upstream is not set up for pushing. Push with set upstream to set it again.")
			}
			args = []string{"push", "--", remote, "refs/heads/" + branch + ":" + merge}
		} else {
			if !setUpstream {
				return apiErr("NO_UPSTREAM", "This branch has no upstream yet. Push with set upstream to create one.")
			}
			remote, err := s.pickRemote(ctx, pl)
			if err != nil {
				return err
			}
			args = []string{"push", "--set-upstream", "--", remote, "refs/heads/" + branch + ":refs/heads/" + branch}
		}
		out, stderr, err := s.runX(ctx, pl.top, true, nil, 0, nil, args...)
		if err != nil {
			if re, ok := err.(*runError); ok && re.exit > 0 && pushRejected(stderr) {
				return apiErr("PUSH_REJECTED", "The remote has commits you do not have. Pull first.\n%s", clean(stderr))
			}
			return s.failure(err)
		}
		res = &PushResult{Upstream: s.upstreamOf(ctx, pl), Output: clean(string(out) + "\n" + stderr)}
		return nil
	})
	return res, err
}

func pushRejected(stderr string) bool {
	return strings.Contains(stderr, "[rejected]") || strings.Contains(stderr, "non-fast-forward") || strings.Contains(stderr, "fetch first")
}

// config is one git config value, "" when unset.
func (s *Service) config(ctx context.Context, pl *place, key string) string {
	out, _, err := s.runX(ctx, pl.top, false, nil, 0, []int{1}, "config", "--get", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (s *Service) upstreamOf(ctx context.Context, pl *place) string {
	out, _, err := s.runX(ctx, pl.top, false, nil, 0, nil, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (s *Service) pickRemote(ctx context.Context, pl *place) (string, error) {
	out, err := s.run(ctx, pl.top, false, nil, "remote")
	if err != nil {
		return "", s.failure(err)
	}
	remotes := strings.Fields(string(out))
	for _, r := range remotes {
		if r == "origin" {
			return r, nil
		}
	}
	if len(remotes) == 1 {
		return remotes[0], nil
	}
	return "", apiErr("NO_REMOTE", "This repository has no remote called origin to push to.")
}
