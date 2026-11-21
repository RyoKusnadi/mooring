package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

const maxLinePairs = 100000

// StageLines stages (or, with staged, unstages) chosen lines of one file.
// version is the one the browser got with the diff.
func (s *Service) StageLines(ctx context.Context, projectID, folder, rel string, staged bool, version string, lines [][2]int) error {
	if len(lines) == 0 || len(lines) > maxLinePairs {
		return apiErr("BAD_REQUEST", "Choose 1 to %d lines.", maxLinePairs)
	}
	pl, err := s.need(projectID, folder)
	if err != nil {
		return err
	}
	p, err := pl.relPath(rel)
	if err != nil {
		return err
	}
	if p == pl.prefix || strings.ContainsFunc(p, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return apiErr("BAD_REQUEST", "Line staging needs the path of one file with an ordinary name.")
	}
	chosen := make(map[[2]int]bool, len(lines))
	for _, l := range lines {
		chosen[l] = true
	}
	return s.change(pl, func() error { return s.stageLines(ctx, pl, p, staged, version, chosen) })
}

func (s *Service) stageLines(ctx context.Context, pl *place, p string, staged bool, version string, chosen map[[2]int]bool) error {
	unmerged, err := s.run(ctx, pl.top, false, nil, "ls-files", "-u", "-z", "--", p)
	if err != nil {
		return s.failure(err)
	}
	if len(unmerged) > 0 {
		return apiErr("BAD_REQUEST", "This file has a merge conflict. Resolve it, then stage the file.")
	}
	if err := refuseSpecial(pl, p); err != nil {
		return err
	}
	untracked := false
	if !staged {
		if untracked, err = s.untracked(ctx, pl, p); err != nil {
			return err
		}
	}
	raw, orig, tooLarge, err := s.workDiff(ctx, pl, p, staged)
	if err != nil {
		return err
	}
	switch {
	case tooLarge:
		return apiErr("BAD_REQUEST", "This diff is too large to stage by lines. Stage the whole file.")
	case orig != "":
		return apiErr("BAD_REQUEST", "This file is a staged rename. Unstage the whole file first.")
	case versionOf(raw) != version:
		return apiErr("DIFF_CHANGED", "The file changed after you opened this diff. Reload the diff and choose again.")
	}
	file := splitRaw(raw)
	if _, binary := parseDiff(raw); binary {
		return apiErr("BAD_REQUEST", "Binary files cannot be staged by lines. Stage the whole file.")
	}
	kind, mode, modeChange := fileKind(file.header)
	if len(file.hunks) == 0 {
		if modeChange {
			return apiErr("BAD_REQUEST", "Only the file mode changed. Stage the whole file.")
		}
		return apiErr("BAD_REQUEST", "There are no changed lines in this file.")
	}
	if oddHeader(file.header) {
		return apiErr("BAD_REQUEST", "This change is a type change, link or submodule. Stage the whole file.")
	}
	for c := range chosen {
		if c[0] < 0 || c[0] >= len(file.hunks) || c[1] < 0 || c[1] >= len(file.hunks[c[0]].raw) {
			return apiErr("BAD_REQUEST", "Those lines are not in this diff. Reload the diff and choose again.")
		}
	}
	if untracked {
		return s.stageNewFile(ctx, pl, p, file, chosen)
	}
	ps := &patchSpec{file: file, path: p, kind: kind, mode: mode, chosen: chosen, reverse: staged}
	patch, _, err := ps.build()
	if err != nil {
		return err
	}
	args := []string{"-c", "core.autocrlf=false", "-c", "core.safecrlf=false", "apply", "--cached", "--whitespace=nowarn"}
	if staged {
		args = append(args, "--reverse")
	}
	if _, err := s.run(ctx, pl.top, true, []byte(patch), append(args, "-")...); err != nil {
		return s.failure(err)
	}
	return nil
}

// refuseSpecial refuses a nested repository or a link, which have no lines.
func refuseSpecial(pl *place, p string) error {
	if fi, err := os.Lstat(filepath.Join(pl.top, filepath.FromSlash(p))); err == nil && (fi.IsDir() || fi.Mode()&os.ModeSymlink != 0) {
		return apiErr("BAD_REQUEST", "This is a folder, link or submodule. Stage it whole.")
	}
	return nil
}

// stageNewFile adds an untracked file to the index with only the chosen lines.
func (s *Service) stageNewFile(ctx context.Context, pl *place, p string, file rawFile, chosen map[[2]int]bool) error {
	if len(file.hunks) != 1 {
		return apiErr("BAD_REQUEST", "Reload the diff and choose again.")
	}
	content := newFileContent(file, chosen)
	if len(content) == 0 {
		return apiErr("BAD_REQUEST", "Choose at least one added line.")
	}
	mode := "100644"
	if fi, err := os.Lstat(filepath.Join(pl.top, filepath.FromSlash(p))); err == nil && fi.Mode()&0o111 != 0 {
		mode = "100755"
	}
	out, err := s.run(ctx, pl.top, true, content, "hash-object", "-w", "--no-filters", "--stdin")
	if err != nil {
		return s.failure(err)
	}
	if _, err := s.run(ctx, pl.top, true, nil, "update-index", "--add", "--cacheinfo", mode+","+strings.TrimSpace(string(out))+","+p); err != nil {
		return s.failure(err)
	}
	return nil
}

// oddHeader spots a diff that is more than one file or touches a link or submodule.
func oddHeader(header []string) bool {
	for i, l := range header {
		f := strings.Fields(l)
		if len(f) == 0 {
			continue
		}
		last := f[len(f)-1]
		switch {
		case i > 0 && strings.HasPrefix(l, "diff --git "):
			return true
		case strings.Contains(l, " mode ") && isLinkMode(last):
			return true
		case strings.HasPrefix(l, "index ") && len(f) == 3 && isLinkMode(last):
			return true
		}
	}
	return false
}

func isLinkMode(m string) bool { return m == "160000" || m == "120000" }
