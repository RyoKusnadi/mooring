package git

import "context"

const stagedLimit = 64 << 10

// Staged returns `git diff --cached --stat` and the staged diff, for a commit
// message to be written from. Both are empty when nothing is staged. Long diffs
// are cut: the caller shortens them further.
func (s *Service) Staged(ctx context.Context, projectID, folder string) (stat, diff string, err error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return "", "", err
	}
	get := func(args ...string) (string, error) {
		out, _, err := s.runX(ctx, pl.top, false, nil, stagedLimit, nil, append([]string{"diff", "--cached", "--no-ext-diff", "--no-color"}, args...)...)
		if re, ok := err.(*runError); ok && re.tooLarge {
			return string(re.partial), nil
		}
		if err != nil {
			return "", s.failure(err)
		}
		return string(out), nil
	}
	if stat, err = get("--stat"); err != nil {
		return "", "", err
	}
	if stat == "" {
		return "", "", nil
	}
	diff, err = get("-U2")
	return stat, diff, err
}

// Uncommitted returns what is not committed in a folder of a project, compared with
// HEAD: the stat, the names of new files git does not track yet, and the diff,
// all limited to that folder. Both are empty when nothing changed.
func (s *Service) Uncommitted(ctx context.Context, projectID, folder string) (stat, diff string, err error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return "", "", err
	}
	if _, err := s.run(ctx, pl.dir, false, nil, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return "", "", apiErr("NO_COMMIT", "This folder has no commit yet, so there is nothing to compare with.")
	}
	get := func(args ...string) (string, error) {
		out, _, err := s.runX(ctx, pl.dir, false, nil, stagedLimit, nil, args...)
		if re, ok := err.(*runError); ok && re.tooLarge {
			return string(re.partial), nil
		}
		if err != nil {
			return "", s.failure(err)
		}
		return string(out), nil
	}
	diffArgs := []string{"diff", "HEAD", "--no-ext-diff", "--no-color"}
	if stat, err = get(append(diffArgs, "--stat", "--", ".")...); err != nil {
		return "", "", err
	}
	added, err := get("ls-files", "--others", "--exclude-standard", "--", ".")
	if err != nil {
		return "", "", err
	}
	if added != "" {
		stat += "New files, not in git yet:\n" + added
	}
	if stat == "" {
		return "", "", nil
	}
	diff, err = get(append(diffArgs, "-U2", "--", ".")...)
	return stat, diff, err
}
