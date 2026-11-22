package git

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
)

// ops are the operations that can stop partway, in the order they are checked, with the file
// git keeps while one runs. A rebase is checked first: its steps leave other heads behind.
var ops = []struct{ name, file string }{
	{"am", "rebase-apply/applying"},
	{"rebase", "rebase-merge"},
	{"rebase", "rebase-apply"},
	{"merge", "MERGE_HEAD"},
	{"cherry-pick", "CHERRY_PICK_HEAD"},
	{"revert", "REVERT_HEAD"},
}

// operation is the operation the repository is in the middle of, or "".
func (s *Service) operation(ctx context.Context, pl *place) string {
	args := []string{"rev-parse"}
	for _, o := range ops {
		args = append(args, "--git-path", o.file)
	}
	out, err := s.run(ctx, pl.top, false, nil, args...)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	for i, o := range ops {
		if i >= len(lines) {
			break
		}
		p := lines[i]
		if !filepath.IsAbs(p) {
			p = filepath.Join(pl.top, p)
		}
		if _, err := os.Lstat(p); err == nil {
			return o.name
		}
	}
	return ""
}

// Resolve settles conflicted files: side "ours" keeps the current branch's version, "theirs"
// the incoming one, and "mark" stages the file as the user edited it. A side that deleted the
// file deletes it (to the trash).
func (s *Service) Resolve(ctx context.Context, projectID, folder string, rels []string, side string) error {
	if side != "ours" && side != "theirs" && side != "mark" {
		return apiErr("BAD_REQUEST", "Choose ours, theirs or mark.")
	}
	pl, err := s.need(projectID, folder)
	if err != nil {
		return err
	}
	list, all, err := pl.targets(rels)
	if err != nil {
		return err
	}
	if all {
		return apiErr("BAD_REQUEST", "Name the conflicted files one by one.")
	}
	return s.change(pl, func() error {
		for _, p := range list {
			if err := s.resolveOne(ctx, pl, p, side); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) resolveOne(ctx context.Context, pl *place, p, side string) error {
	out, err := s.run(ctx, pl.top, false, nil, "ls-files", "-u", "-z", "--", p)
	if err != nil {
		return s.failure(err)
	}
	stages := map[string]bool{}
	for _, rec := range strings.Split(string(out), "\x00") {
		// "<mode> <object> <stage>\t<path>"
		if f := strings.Fields(strings.SplitN(rec, "\t", 2)[0]); len(f) == 3 {
			stages[f[2]] = true
		}
	}
	if len(stages) == 0 {
		return apiErr("NOT_CONFLICTED", "%s has no merge conflict.", p)
	}
	full := filepath.Join(pl.top, filepath.FromSlash(p))
	switch side {
	case "mark":
		if hasMarkers(full) {
			return apiErr("HAS_MARKERS", "%s still has conflict markers (<<<<<<<). Edit them out, then mark it resolved.", p)
		}
		_, err = s.run(ctx, pl.top, true, nil, "add", "-A", "--", p)
	default:
		stage := map[string]string{"ours": "2", "theirs": "3"}[side]
		if stages[stage] {
			if _, err = s.run(ctx, pl.top, true, nil, "checkout", "--"+side, "--", p); err == nil {
				_, err = s.run(ctx, pl.top, true, nil, "add", "--", p)
			}
			break
		}
		// That side deleted the file.
		if _, serr := os.Lstat(full); serr == nil {
			if err := s.dispose(full, false); err != nil {
				return err
			}
		}
		_, err = s.run(ctx, pl.top, true, nil, "rm", "-q", "--cached", "--ignore-unmatch", "--", p)
	}
	if err != nil {
		return s.failure(err)
	}
	return nil
}

// stageSpec keeps staging from settling a conflict by accident: staging everything names
// every other changed file (git add -A settles conflicts even when told to leave them out),
// and a conflicted file named on its own is refused while it has markers.
func (s *Service) stageSpec(ctx context.Context, pl *place, list []string, all bool, spec []string) ([]string, []byte, error) {
	unmerged := s.unmergedSet(ctx, pl)
	if len(unmerged) == 0 {
		return spec, nil, nil
	}
	if !all {
		for u := range unmerged {
			if underAny(u, list) && hasMarkers(filepath.Join(pl.top, filepath.FromSlash(u))) {
				return nil, nil, apiErr("HAS_MARKERS", "%s still has conflict markers (<<<<<<<). Edit them out, then mark it resolved.", u)
			}
		}
		return spec, nil, nil
	}
	st, err := s.statusOf(ctx, pl)
	if err != nil {
		return nil, nil, err
	}
	if st.Truncated {
		return nil, nil, apiErr("BAD_REQUEST", "Too many files changed to stage them all while files conflict. Stage a folder at a time.")
	}
	var names bytes.Buffer
	for _, c := range st.Files {
		if c.Kind == "conflicted" {
			continue
		}
		for _, p := range []string{c.Path, c.OrigPath} {
			if p != "" {
				names.WriteString(joinPrefix(pl.prefix, p) + "\x00")
			}
		}
	}
	if names.Len() == 0 {
		return nil, nil, apiErr("NOTHING_TO_STAGE", "Only conflicted files are left. Resolve each one with its buttons.")
	}
	return []string{"--pathspec-from-file=-", "--pathspec-file-nul"}, names.Bytes(), nil
}

// hasMarkers reports whether the file has a line that starts a conflict (<<<<<<<) or ends one.
func hasMarkers(full string) bool {
	f, err := os.Open(full)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		l := sc.Bytes()
		if marker(l, "<<<<<<<") || marker(l, ">>>>>>>") {
			return true
		}
	}
	return false
}

// marker reports whether line is m alone or m and a space (git adds the side's name).
func marker(line []byte, m string) bool {
	rest, ok := bytes.CutPrefix(bytes.TrimRight(line, "\r"), []byte(m))
	return ok && (len(rest) == 0 || rest[0] == ' ')
}

// Finish ends the stopped operation ("abort" goes back to before it, "continue" goes on after
// every conflict is resolved). It returns the operation still running afterwards, for
// example a rebase that stopped again at its next commit.
func (s *Service) Finish(ctx context.Context, projectID, folder, how string) (map[string]any, error) {
	if how != "abort" && how != "continue" {
		return nil, apiErr("BAD_REQUEST", "Choose abort or continue.")
	}
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	var after string
	err = s.change(pl, func() error {
		op := s.operation(ctx, pl)
		if op == "" {
			return apiErr("NO_OPERATION", "No merge, rebase or cherry-pick is in progress.")
		}
		if how == "continue" {
			if left, err := s.run(ctx, pl.top, false, nil, "ls-files", "-u", "-z"); err != nil {
				return s.failure(err)
			} else if len(left) > 0 {
				return apiErr("CONFLICTED", "Some files still have conflicts. Resolve them first.")
			}
		}
		_, stderr, err := s.runX(ctx, pl.top, true, nil, 0, nil, "-c", "core.editor=true", "-c", "user.useConfigOnly=true", op, "--"+how)
		after = s.operation(ctx, pl)
		if err != nil {
			var re *runError
			if !asRun(err, &re) || re.exit < 0 {
				return s.failure(err)
			}
			if after != "" && how == "continue" {
				if left, _ := s.run(ctx, pl.top, false, nil, "ls-files", "-u", "-z"); len(left) > 0 {
					return nil // the next step conflicts; the status shows it
				}
			}
			return apiErr("GIT_FAILED", "%s", clean(re.stdout+"\n"+stderr))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"operation": after}, nil
}
