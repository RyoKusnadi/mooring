package git

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

const maxStashMessage = 1000

// Stash is one saved set of changes.
type Stash struct {
	ID      string `json:"id"`
	Index   int    `json:"index"`
	Message string `json:"message"`
	Branch  string `json:"branch"`
	Time    int64  `json:"time"`
}

var stashSubject = regexp.MustCompile(`^(?:WIP on|On) ([^:]*): ?(.*)$`)

func (s *Service) stashList(ctx context.Context, pl *place) ([]Stash, error) {
	out, err := s.run(ctx, pl.top, false, nil, "stash", "list", "-z", "--format=%H%x1f%ct%x1f%gs")
	if err != nil {
		return nil, s.failure(err)
	}
	list := []Stash{}
	for _, rec := range strings.Split(string(out), "\x00") {
		f := strings.SplitN(strings.TrimLeft(rec, "\n"), "\x1f", 3)
		if len(f) != 3 {
			continue
		}
		st := Stash{ID: f[0], Index: len(list), Message: f[2], Time: atoi64(f[1])}
		if m := stashSubject.FindStringSubmatch(f[2]); m != nil {
			st.Branch, st.Message = m[1], m[2]
			if st.Branch == "(no branch)" {
				st.Branch = ""
			}
		}
		list = append(list, st)
	}
	return list, nil
}

// Stashes lists the stashes, newest first.
func (s *Service) Stashes(ctx context.Context, projectID, folder string) ([]Stash, error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	return s.stashList(ctx, pl)
}

func (s *Service) stashTip(ctx context.Context, pl *place) (string, error) {
	out, _, err := s.runX(ctx, pl.top, false, nil, 0, []int{1, 128}, "rev-parse", "-q", "--verify", "refs/stash")
	if err != nil {
		return "", s.failure(err)
	}
	return strings.TrimSpace(string(out)), nil
}

// StashSave saves the changes of the place and removes them from the files.
func (s *Service) StashSave(ctx context.Context, projectID, folder, message string, untracked bool) (map[string]string, error) {
	if len(message) > maxStashMessage || strings.ContainsFunc(message, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return nil, apiErr("BAD_REQUEST", "The stash message is at most %d bytes.", maxStashMessage)
	}
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	if err := s.busy(pl); err != nil {
		return nil, err
	}
	var id string
	err = s.change(pl, func() error {
		before, err := s.stashTip(ctx, pl)
		if err != nil {
			return err
		}
		args := []string{"stash", "push", "-q"}
		if untracked {
			args = append(args, "--include-untracked")
		}
		if message != "" {
			args = append(args, "-m", message)
		}
		if sc := pl.scope(); sc != "" {
			args = append(args, "--", ":(literal)"+sc)
		}
		ctx := context.WithValue(ctx, looseKey{}, true)
		_, stderr, err := s.runX(ctx, pl.top, true, nil, 0, nil, args...)
		if err != nil {
			if strings.Contains(stderr, "empty ident") || strings.Contains(stderr, "identity unknown") {
				return apiErr("NO_IDENTITY", "Git has no user.name and user.email. Set them with git config, then stash again.")
			}
			return s.failure(err)
		}
		if id, err = s.stashTip(ctx, pl); err != nil {
			return err
		}
		if id == "" || id == before {
			return apiErr("NOTHING_TO_STASH", "There are no changes to stash.")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]string{"id": id}, nil
}

// stashDo applies, pops or drops the stash with this commit id.
func (s *Service) stashDo(ctx context.Context, projectID, folder, id, verb string) error {
	if !hexID.MatchString(id) || len(id) < 40 {
		return apiErr("BAD_REQUEST", "A stash is named by its full commit id.")
	}
	pl, err := s.need(projectID, folder)
	if err != nil {
		return err
	}
	if err := s.busy(pl); err != nil {
		return err
	}
	return s.change(pl, func() error {
		list, err := s.stashList(ctx, pl)
		if err != nil {
			return err
		}
		ref := ""
		for _, st := range list {
			if strings.EqualFold(st.ID, id) {
				ref = "stash@{" + strconv.Itoa(st.Index) + "}"
			}
		}
		if ref == "" {
			return apiErr("NOT_FOUND", "That stash no longer exists.")
		}
		before := s.unmergedSet(ctx, pl)
		_, stderr, err := s.runX(ctx, pl.top, true, nil, 0, nil, "stash", verb, ref)
		if err == nil {
			return nil
		}
		if s.newlyUnmerged(ctx, pl, before) {
			msg := stderr
			if re, ok := err.(*runError); ok {
				msg += "\n" + re.stdout
			}
			msg = clean(msg)
			return apiErr("CONFLICTED", "The stash conflicts with the files. Resolve the conflicts; the stash is kept. %s", msg)
		}
		return s.failure(err)
	})
}

// StashApply puts the stash's changes back and keeps the stash.
func (s *Service) StashApply(ctx context.Context, projectID, folder, id string) error {
	return s.stashDo(ctx, projectID, folder, id, "apply")
}

// StashPop applies the stash and drops it when it applied cleanly.
func (s *Service) StashPop(ctx context.Context, projectID, folder, id string) error {
	return s.stashDo(ctx, projectID, folder, id, "pop")
}

// StashDrop deletes the stash.
func (s *Service) StashDrop(ctx context.Context, projectID, folder, id string) error {
	return s.stashDo(ctx, projectID, folder, id, "drop")
}

// unmergedSet is the paths with unmerged index entries.
func (s *Service) unmergedSet(ctx context.Context, pl *place) map[string]bool {
	out, err := s.run(ctx, pl.top, false, nil, "ls-files", "-u", "-z")
	set := map[string]bool{}
	if err != nil {
		return set
	}
	for _, rec := range strings.Split(string(out), "\x00") {
		if _, p, ok := strings.Cut(rec, "\t"); ok {
			set[p] = true
		}
	}
	return set
}

// newlyUnmerged tells whether a run left unmerged paths that were not there before.
func (s *Service) newlyUnmerged(ctx context.Context, pl *place, before map[string]bool) bool {
	for p := range s.unmergedSet(ctx, pl) {
		if !before[p] {
			return true
		}
	}
	return false
}
