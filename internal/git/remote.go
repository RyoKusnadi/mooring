package git

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
)

// sshKey carries the GIT_SSH_COMMAND a network command runs with.
type sshKey struct{}

// netCtx marks ctx so that git's ssh never waits for a passphrase or a host
// key question: BatchMode=yes is added to the ssh command git will use
// (GIT_SSH_COMMAND, then core.sshCommand), or to plain ssh. It is left alone
// when that command is not ssh, when it already sets BatchMode, and when
// only a GIT_SSH program is set, which cannot take an option.
func (s *Service) netCtx(ctx context.Context, pl *place) context.Context {
	base, gitSSH := "", false
	for _, e := range append(osEnviron(), s.cfg.Env...) {
		if v, ok := strings.CutPrefix(e, "GIT_SSH_COMMAND="); ok {
			base = v
		} else if _, ok := strings.CutPrefix(e, "GIT_SSH="); ok {
			gitSSH = true
		}
	}
	if base == "" {
		base = s.config(ctx, pl, "core.sshCommand")
	}
	if base == "" {
		if gitSSH {
			return ctx
		}
		base = "ssh"
	}
	if f := strings.Fields(base); len(f) == 0 || filepath.Base(f[0]) != "ssh" || strings.Contains(strings.ToLower(base), "batchmode") {
		return ctx
	}
	return context.WithValue(ctx, sshKey{}, base+" -o BatchMode=yes")
}

// FetchResult is the value of git.fetch.
type FetchResult struct {
	Upstream string `json:"upstream"`
	// New is how many commits the fetch added to the upstream branch.
	New int `json:"new"`
}

// remoteOf is the remote the current branch tracks, else the one pickRemote chooses.
func (s *Service) remoteOf(ctx context.Context, pl *place, branch string) (string, error) {
	if branch != "" {
		if r := s.config(ctx, pl, "branch."+branch+".remote"); r != "" && r != "." {
			return r, nil
		}
	}
	return s.pickRemote(ctx, pl)
}

func (s *Service) revOf(ctx context.Context, pl *place, rev string) string {
	out, _, err := s.runX(ctx, pl.top, false, nil, 0, nil, "rev-parse", "--verify", "-q", rev+"^{commit}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (s *Service) countBetween(ctx context.Context, pl *place, from, to string) int {
	if to == "" {
		return 0
	}
	spec := to
	if from != "" {
		spec = from + ".." + to
	}
	out, _, err := s.runX(ctx, pl.top, false, nil, 0, nil, "rev-list", "--count", spec)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

// isAncestor reports whether a is reachable from b.
func (s *Service) isAncestor(ctx context.Context, pl *place, a, b string) bool {
	_, _, err := s.runX(ctx, pl.top, false, nil, 0, nil, "merge-base", "--is-ancestor", a, b)
	return err == nil
}

// Fetch updates the remote-tracking branches. It changes no branch and no file.
func (s *Service) Fetch(ctx context.Context, projectID, folder string) (*FetchResult, error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	var res *FetchResult
	err = s.change(pl, func() error {
		ctx := s.netCtx(ctx, pl)
		remote, err := s.remoteOf(ctx, pl, s.branchOf(ctx, pl))
		if err != nil {
			return err
		}
		before := s.revOf(ctx, pl, "@{upstream}")
		if _, err := s.run(ctx, pl.top, true, nil, "fetch", "-q", "--no-tags", "--", remote); err != nil {
			return s.failure(err)
		}
		res = &FetchResult{Upstream: s.upstreamOf(ctx, pl)}
		res.New = s.countBetween(ctx, pl, before, s.revOf(ctx, pl, "@{upstream}"))
		return nil
	})
	return res, err
}

// PullResult is the value of git.pull.
type PullResult struct {
	Upstream string `json:"upstream"`
	// Pulled is how many commits the branch moved forward; 0 means it was up to date.
	Pulled int `json:"pulled"`
}

// Pull fetches, then moves the branch forward to its upstream. It never makes
// a merge commit: when both sides have new commits it stops before it touches
// any file, and the user merges or rebases in a terminal.
func (s *Service) Pull(ctx context.Context, projectID, folder string) (*PullResult, error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	var res *PullResult
	err = s.change(pl, func() error {
		ctx := s.netCtx(ctx, pl)
		branch := s.branchOf(ctx, pl)
		if branch == "" {
			return apiErr("BAD_REQUEST", "A detached HEAD cannot be pulled. Switch to a branch first.")
		}
		upstream := s.upstreamOf(ctx, pl)
		if upstream == "" {
			return apiErr("NO_UPSTREAM", "This branch has no upstream to pull from. Push it first to create one.")
		}
		remote, err := s.remoteOf(ctx, pl, branch)
		if err != nil {
			return err
		}
		if _, err := s.run(ctx, pl.top, true, nil, "fetch", "-q", "--no-tags", "--", remote); err != nil {
			return s.failure(err)
		}
		head, up := s.revOf(ctx, pl, "HEAD"), s.revOf(ctx, pl, "@{upstream}")
		if up == "" {
			return apiErr("NO_UPSTREAM", "%s does not exist on the remote any more.", upstream)
		}
		res = &PullResult{Upstream: upstream}
		if head == up || s.isAncestor(ctx, pl, up, head) {
			return nil
		}
		if head != "" && !s.isAncestor(ctx, pl, head, up) {
			return apiErr("PULL_DIVERGED", "Pull stopped: your branch and %s both have new commits. Merge or rebase in a terminal.", upstream)
		}
		if _, _, err := s.runX(ctx, pl.top, true, nil, 0, nil, "merge", "--ff-only", "-q", "--no-edit", up); err != nil {
			return s.failure(err)
		}
		res.Pulled = s.countBetween(ctx, pl, head, up)
		return nil
	})
	return res, err
}
