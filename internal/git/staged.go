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
