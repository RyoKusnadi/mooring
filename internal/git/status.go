package git

import (
	"bytes"
	"context"
	"sort"
	"strconv"
	"strings"
)

const maxChanges = 5000

// Change is one file in the status.
type Change struct {
	Path     string `json:"path"`
	OrigPath string `json:"origPath,omitempty"`
	Staged   string `json:"staged"`
	Unstaged string `json:"unstaged"`
	Kind     string `json:"kind"`
}

// Status is the answer of GET /api/git/status.
type Status struct {
	Repo       bool     `json:"repo"`
	Branch     string   `json:"branch"`
	Detached   bool     `json:"detached"`
	Head       string   `json:"head"`
	Upstream   string   `json:"upstream"`
	Ahead      int      `json:"ahead"`
	Behind     int      `json:"behind"`
	Files      []Change `json:"files"`
	StagedOut  int      `json:"stagedOutside"`
	Truncated  bool     `json:"truncated"`
	Conflicted bool     `json:"conflicted"`
	// Pushed is true when the last commit is on some remote branch.
	Pushed bool `json:"pushed"`
}

// Status reads the changes of a place.
func (s *Service) Status(ctx context.Context, projectID, folder string) (*Status, error) {
	pl, repo, err := s.resolve(projectID, folder)
	if err != nil {
		return nil, err
	}
	if !repo {
		return &Status{Files: []Change{}}, nil
	}
	return s.statusOf(ctx, pl)
}

func (s *Service) statusOf(ctx context.Context, pl *place) (*Status, error) {
	args := []string{"status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all"}
	if sc := pl.scope(); sc != "" {
		args = append(args, "--", sc)
	}
	out, cut, err := s.runCapped(ctx, pl.top, args...)
	if err != nil {
		return nil, s.failure(err)
	}
	st := parseStatus(out, pl)
	st.Truncated = st.Truncated || cut
	if st.Head != "" {
		n, ok := s.commitsOffRemotes(ctx, pl.top)
		st.Pushed = ok && n == 0
	}
	if pl.prefix != "" {
		if st.StagedOut, err = s.stagedOutside(ctx, pl); err != nil {
			return nil, err
		}
	}
	return st, nil
}

// stagedOutside counts staged files in the repository that lie outside the place.
func (s *Service) stagedOutside(ctx context.Context, pl *place) (int, error) {
	out, _, err := s.runCapped(ctx, pl.top, "diff", "--cached", "--name-only", "-z", "--no-ext-diff")
	if err != nil {
		return 0, s.failure(err)
	}
	n := 0
	for _, f := range bytes.Split(out, []byte{0}) {
		if len(f) == 0 {
			continue
		}
		if _, in := pl.inPlace(string(f)); !in {
			n++
		}
	}
	return n, nil
}

// maxListBytes caps what a status read takes in; a repository with more is listed in part.
var maxListBytes = 8 << 20

// runCapped runs a read whose output is a NUL-separated list. Past maxListBytes
// it returns the whole records read so far and cut=true.
func (s *Service) runCapped(ctx context.Context, dir string, args ...string) (out []byte, cut bool, err error) {
	out, _, err = s.runX(ctx, dir, false, nil, maxListBytes, nil, args...)
	var re *runError
	if err != nil && asRun(err, &re) && re.tooLarge {
		out = re.partial
		if i := bytes.LastIndexByte(out, 0); i >= 0 {
			return out[:i], true, nil
		}
		return nil, true, nil
	}
	return out, false, err
}

var sideName = map[byte]string{'M': "modified", 'T': "typechange", 'A': "added", 'D': "deleted", 'R': "renamed", 'C': "added"}

func parseStatus(out []byte, pl *place) *Status {
	st := &Status{Repo: true, Files: []Change{}}
	recs := strings.Split(string(out), "\x00")
	rel := func(p string) string {
		r, _ := pl.inPlace(p)
		return r
	}
	for i := 0; i < len(recs); i++ {
		r := recs[i]
		if r == "" {
			continue
		}
		switch r[0] {
		case '#':
			st.header(r)
			continue
		case '?':
			if len(r) > 2 {
				st.add(Change{Path: rel(r[2:]), Kind: "untracked"})
			}
			continue
		case '!':
			continue
		}
		var c Change
		switch r[0] {
		case '1':
			f := strings.SplitN(r, " ", 9)
			if len(f) < 9 {
				continue
			}
			c = Change{Path: rel(f[8])}
		case '2':
			f := strings.SplitN(r, " ", 10)
			if len(f) < 10 || i+1 >= len(recs) {
				continue
			}
			i++
			c = Change{Path: rel(f[9]), OrigPath: rel(recs[i])}
		case 'u':
			f := strings.SplitN(r, " ", 11)
			if len(f) < 11 {
				continue
			}
			c = Change{Path: rel(f[10]), Kind: "conflicted"}
			st.Conflicted = true
			st.add(c)
			continue
		default:
			continue
		}
		xy := strings.SplitN(r, " ", 3)[1]
		c.Staged, c.Unstaged = sideName[xy[0]], sideName[xy[1]]
		st.add(c)
	}
	sort.Slice(st.Files, func(i, j int) bool { return st.Files[i].Path < st.Files[j].Path })
	return st
}

func (st *Status) add(c Change) {
	if len(st.Files) >= maxChanges {
		st.Truncated = true
		return
	}
	st.Files = append(st.Files, c)
}

func (st *Status) header(r string) {
	k, v, _ := strings.Cut(r[2:], " ")
	switch k {
	case "branch.oid":
		if v != "(initial)" && len(v) >= 7 {
			st.Head = v[:7]
		}
	case "branch.head":
		if v == "(detached)" {
			st.Detached = true
		} else {
			st.Branch = v
		}
	case "branch.upstream":
		st.Upstream = v
	case "branch.ab":
		a, b, _ := strings.Cut(v, " ")
		st.Ahead, _ = strconv.Atoi(strings.TrimPrefix(a, "+"))
		st.Behind, _ = strconv.Atoi(strings.TrimPrefix(b, "-"))
	}
}
