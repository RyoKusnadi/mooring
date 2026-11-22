package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryokusnadi/mooring/internal/files"
)

const maxPaths = 1000

// change runs fn with the repository locked, then announces git.changed.
func (s *Service) change(pl *place, fn func() error) error {
	unlock := s.repoLock(pl.top)
	err := fn()
	unlock()
	if s.cfg.Pub != nil {
		s.cfg.Pub.Publish("git.changed", map[string]string{"projectId": pl.project, "folder": pl.folderOf(pl.dir)})
	}
	return err
}

// folderOf is the folder of a request for the real folder dir, relative to the project.
func (p *place) folderOf(dir string) string {
	r, err := filepath.Rel(p.root, dir)
	if err != nil || r == "." {
		return ""
	}
	return filepath.ToSlash(r)
}

// targets checks the paths of a request. all is true for [""]; otherwise the
// paths come back relative to the repository top.
func (pl *place) targets(rels []string) (list []string, all bool, err error) {
	if len(rels) == 0 || len(rels) > maxPaths {
		return nil, false, apiErr("BAD_REQUEST", "Send 1 to %d paths.", maxPaths)
	}
	for _, r := range rels {
		if r == "" {
			all = true
			continue
		}
		p, err := pl.relPath(r)
		if err != nil {
			return nil, false, err
		}
		list = append(list, p)
	}
	if all {
		return nil, true, nil
	}
	return list, false, nil
}

func (pl *place) spec(list []string, all bool) []string {
	if all {
		if pl.prefix == "" {
			return nil
		}
		return []string{"--", pl.prefix}
	}
	return append([]string{"--"}, list...)
}

// Stage adds changes (including deletes) to the index.
func (s *Service) Stage(ctx context.Context, projectID, folder string, rels []string) error {
	return s.stageOp(ctx, projectID, folder, rels, []string{"add", "-A"})
}

// Unstage takes changes out of the index and keeps the working files.
func (s *Service) Unstage(ctx context.Context, projectID, folder string, rels []string) error {
	return s.stageOp(ctx, projectID, folder, rels, []string{"reset", "-q"})
}

func (s *Service) stageOp(ctx context.Context, projectID, folder string, rels []string, base []string) error {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return err
	}
	list, all, err := pl.targets(rels)
	if err != nil {
		return err
	}
	return s.change(pl, func() error {
		spec, stdin := pl.spec(list, all), []byte(nil)
		if base[0] == "add" {
			if spec, stdin, err = s.stageSpec(ctx, pl, list, all, spec); err != nil {
				return err
			}
		}
		if _, err := s.run(ctx, pl.top, true, stdin, append(base, spec...)...); err != nil {
			return s.failure(err)
		}
		return nil
	})
}

// Discard throws away unstaged changes and untracked files. Each file goes to
// the trash first unless permanent is set.
func (s *Service) Discard(ctx context.Context, projectID, folder string, rels []string, permanent bool) error {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return err
	}
	list, all, err := pl.targets(rels)
	if err != nil {
		return err
	}
	if !permanent && s.trash == "" {
		return apiErr("TRASH_UNAVAILABLE", "This server has no trash. Discard for good only if you are sure.")
	}
	return s.change(pl, func() error {
		st, err := s.statusOf(ctx, pl)
		if err != nil {
			return err
		}
		var restore []string
		for _, c := range st.Files {
			top := joinPrefix(pl.prefix, c.Path)
			if !all && !underAny(top, list) || c.Kind == "conflicted" {
				continue
			}
			untracked := c.Kind == "untracked"
			if !untracked && c.Unstaged == "" {
				continue
			}
			full := filepath.Join(pl.top, filepath.FromSlash(top))
			// A folder here is a nested repository or a submodule: git lists it as one
			// entry, and throwing it away would take its own history with it.
			if fi, err := os.Lstat(full); err == nil && fi.IsDir() {
				continue
			} else if err == nil {
				if err := s.dispose(full, permanent); err != nil {
					return err
				}
			}
			if !untracked {
				restore = append(restore, top)
			}
		}
		if len(restore) > 0 {
			if _, err := s.run(ctx, pl.top, true, nil, append([]string{"checkout-index", "-f", "-q", "--"}, restore...)...); err != nil {
				return s.failure(err)
			}
		}
		return nil
	})
}

func (s *Service) dispose(full string, permanent bool) error {
	if !permanent {
		return files.MoveToTrash(s.trash, s.log, full)
	}
	if err := os.Remove(full); err != nil {
		return apiErr("INTERNAL", "A file could not be removed.")
	}
	return nil
}

// underAny reports whether p is one of list or inside one of its folders.
func underAny(p string, list []string) bool {
	for _, l := range list {
		if p == l || strings.HasPrefix(p, l+"/") {
			return true
		}
	}
	return false
}

func joinPrefix(prefix, rel string) string {
	if prefix == "" {
		return rel
	}
	if rel == "" {
		return prefix
	}
	return prefix + "/" + rel
}
