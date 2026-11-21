package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Hunk is one block of a diff.
type Hunk struct {
	OldStart int        `json:"oldStart"`
	OldLines int        `json:"oldLines"`
	NewStart int        `json:"newStart"`
	NewLines int        `json:"newLines"`
	Header   string     `json:"header"`
	Lines    [][]string `json:"lines"`
}

// Diff is the answer of GET /api/git/diff.
type Diff struct {
	Path     string `json:"path"`
	OrigPath string `json:"origPath,omitempty"`
	Binary   bool   `json:"binary"`
	TooLarge bool   `json:"tooLarge"`
	Hunks    []Hunk `json:"hunks"`
	Version  string `json:"version"`
}

var diffBase = []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "-U3", "--src-prefix=a/", "--dst-prefix=b/"}

// Diff compares one file: the staged version with HEAD, or the working file
// with the staged version.
func (s *Service) Diff(ctx context.Context, projectID, folder, rel string, staged bool) (*Diff, error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	p, err := pl.relPath(rel)
	if err != nil {
		return nil, err
	}
	if p == pl.prefix {
		return nil, apiErr("BAD_REQUEST", "A diff needs the path of one file.")
	}
	res := &Diff{Hunks: []Hunk{}}
	res.Path, _ = pl.inPlace(p)
	raw, orig, tooLarge, err := s.workDiff(ctx, pl, p, staged)
	if err != nil {
		return nil, err
	}
	if orig != "" {
		res.OrigPath, _ = pl.inPlace(orig)
	}
	fill(res, raw, tooLarge)
	return res, nil
}

// fill puts the parsed output of git's diff into res.
func fill(res *Diff, raw []byte, tooLarge bool) {
	if tooLarge {
		res.TooLarge = true
		return
	}
	res.Hunks, res.Binary = parseDiff(raw)
	res.Version = versionOf(raw)
}

// versionOf names one diff output: the first 16 bytes of its SHA-256, hex.
func versionOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:16])
}

// workDiff is git's raw diff of one file of the repository (p is relative to
// the top). orig is the old path of a staged rename.
func (s *Service) workDiff(ctx context.Context, pl *place, p string, staged bool) (raw []byte, orig string, tooLarge bool, err error) {
	var args []string
	okExit := []int{0}
	if staged {
		orig, err = s.renamedFrom(ctx, pl, p)
		if err != nil {
			return nil, "", false, err
		}
		specs := []string{p}
		if orig != "" {
			specs = []string{orig, p}
		}
		args = append(append(append([]string{}, diffBase...), "--cached", "-M", "--"), specs...)
	} else {
		untracked, err := s.untracked(ctx, pl, p)
		if err != nil {
			return nil, "", false, err
		}
		if untracked {
			args = append(append([]string{}, diffBase...), "--no-index", "--", os.DevNull, filepath.FromSlash(p))
			okExit = []int{0, 1}
		} else {
			args = append(append([]string{}, diffBase...), "--", p)
		}
	}
	raw, tooLarge, err = s.readDiff(ctx, pl.top, okExit, args)
	return raw, orig, tooLarge, err
}

// readDiff runs a diff with the size limit of the contract.
func (s *Service) readDiff(ctx context.Context, dir string, okExit []int, args []string) (raw []byte, tooLarge bool, err error) {
	out, _, err := s.runX(ctx, dir, false, nil, maxDiff, okExit, args...)
	if err != nil {
		var re *runError
		if asRun(err, &re) && re.tooLarge {
			return nil, true, nil
		}
		return nil, false, s.failure(err)
	}
	return out, false, nil
}

// untracked reports whether p is an existing file git does not know.
func (s *Service) untracked(ctx context.Context, pl *place, p string) (bool, error) {
	out, err := s.run(ctx, pl.top, false, nil, "ls-files", "-z", "--cached", "--", p)
	if err != nil {
		return false, s.failure(err)
	}
	if len(out) > 0 {
		return false, nil
	}
	_, err = os.Lstat(filepath.Join(pl.top, filepath.FromSlash(p)))
	return err == nil, nil
}

// renamedFrom finds the old path of a staged rename that ends at p.
func (s *Service) renamedFrom(ctx context.Context, pl *place, p string) (string, error) {
	out, err := s.run(ctx, pl.top, false, nil, "diff", "--cached", "--name-status", "-M", "-z", "--no-ext-diff")
	if err != nil {
		return "", s.failure(err)
	}
	f := bytes.Split(out, []byte{0})
	for i := 0; i < len(f); i++ {
		st := string(f[i])
		switch {
		case strings.HasPrefix(st, "R") || strings.HasPrefix(st, "C"):
			if i+2 < len(f) && string(f[i+2]) == p && st[0] == 'R' {
				return string(f[i+1]), nil
			}
			i += 2
		case st != "":
			i++
		}
	}
	return "", nil
}

// rawHunk is one hunk with its lines as git printed them, kind byte first and
// no line ending ("\r" stays).
type rawHunk struct {
	Hunk
	raw []string
}

// rawFile is the diff of one file: the lines before the first hunk, then hunks.
type rawFile struct {
	header []string
	hunks  []rawHunk
}

// splitRaw reads a unified diff of one file. Hunk and line indexes here are
// the ones the browser gets from parseDiff.
func splitRaw(out []byte) rawFile {
	var f rawFile
	cur := -1
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "@@ "):
			h, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			f.hunks = append(f.hunks, rawHunk{Hunk: h})
			cur = len(f.hunks) - 1
		case cur < 0:
			f.header = append(f.header, line)
		case line == "":
		case line[0] == ' ' || line[0] == '+' || line[0] == '-' || line[0] == '\\':
			f.hunks[cur].raw = append(f.hunks[cur].raw, line)
		}
	}
	return f
}

// parseDiff reads a unified diff of one file.
func parseDiff(out []byte) (hunks []Hunk, binary bool) {
	f := splitRaw(out)
	for _, l := range f.header {
		if strings.HasPrefix(l, "Binary files ") || strings.HasPrefix(l, "GIT binary patch") {
			binary = true
		}
	}
	hunks = make([]Hunk, 0, len(f.hunks))
	for _, rh := range f.hunks {
		h := rh.Hunk
		for _, line := range rh.raw {
			if line[0] == '\\' {
				h.Lines = append(h.Lines, []string{"\\", strings.TrimSpace(line[1:])})
			} else {
				h.Lines = append(h.Lines, []string{line[:1], line[1:]})
			}
		}
		hunks = append(hunks, h)
	}
	return hunks, binary
}

func parseHunkHeader(line string) (Hunk, bool) {
	h := Hunk{Lines: [][]string{}}
	rest := line[3:]
	end := strings.Index(rest, " @@")
	if end < 0 {
		return h, false
	}
	h.Header = strings.TrimPrefix(rest[end+3:], " ")
	old, nw, ok := strings.Cut(rest[:end], " ")
	if !ok || !strings.HasPrefix(old, "-") || !strings.HasPrefix(nw, "+") {
		return h, false
	}
	var ok1, ok2 bool
	h.OldStart, h.OldLines, ok1 = rangeOf(old[1:])
	h.NewStart, h.NewLines, ok2 = rangeOf(nw[1:])
	return h, ok1 && ok2
}

func rangeOf(s string) (start, n int, ok bool) {
	a, b, hasN := strings.Cut(s, ",")
	start, err := strconv.Atoi(a)
	if err != nil {
		return 0, 0, false
	}
	n = 1
	if hasN {
		if n, err = strconv.Atoi(b); err != nil {
			return 0, 0, false
		}
	}
	return start, n, true
}
