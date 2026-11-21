package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"unicode/utf8"
)

// Ref is a name that points at a commit.
type Ref struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // head, branch, remote or tag
}

// LogCommit is one commit of the history.
type LogCommit struct {
	ID      string   `json:"id"`
	Parents []string `json:"parents"`
	Author  string   `json:"author"`
	Email   string   `json:"email"`
	Time    int64    `json:"time"`
	Subject string   `json:"subject"`
	Refs    []Ref    `json:"refs"`
}

const maxSubject = 1000

// refsState is where the branches, tags and HEAD are.
type refsState struct {
	key    string // names this state, safe in a cursor
	head   string // commit id of HEAD, "" before the first commit
	branch string // the branch HEAD is on, "" when detached or unborn
	refs   map[string][]Ref
	tipIDs []string // the commits (or tags) the refs name, for the order
}

var kindRank = map[string]int{"head": 0, "branch": 1, "remote": 2, "tag": 3}

// readRefs reads every ref the history shows, and HEAD.
func (s *Service) readRefs(ctx context.Context, pl *place) (*refsState, error) {
	out, err := s.run(ctx, pl.top, false, nil, "for-each-ref", "--format=%(objectname)%00%(*objectname)%00%(refname)", "refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return nil, s.failure(err)
	}
	headOut, _, err := s.runX(ctx, pl.top, false, nil, 0, []int{1, 128}, "rev-parse", "-q", "--verify", "HEAD")
	if err != nil {
		return nil, s.failure(err)
	}
	st := &refsState{head: strings.TrimSpace(string(headOut)), refs: map[string][]Ref{}}
	if st.head != "" {
		st.branch = s.branchOf(ctx, pl)
	}
	h := sha256.New()
	h.Write(out)
	h.Write([]byte("\x00" + st.head + "\x00" + st.branch))
	st.key = hex.EncodeToString(h.Sum(nil))[:16]
	seen := map[string]bool{}
	addTip := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			st.tipIDs = append(st.tipIDs, id)
		}
	}
	addTip(st.head)
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Split(line, "\x00")
		if len(f) != 3 {
			continue
		}
		addTip(f[0])
		id := f[0]
		if f[1] != "" {
			id = f[1] // an annotated tag: the commit it points at
		}
		var ref Ref
		switch {
		case strings.HasPrefix(f[2], "refs/heads/"):
			ref = Ref{strings.TrimPrefix(f[2], "refs/heads/"), "branch"}
			if ref.Name == st.branch && f[0] == st.head {
				continue // drawn as the head
			}
		case strings.HasPrefix(f[2], "refs/remotes/"):
			ref = Ref{strings.TrimPrefix(f[2], "refs/remotes/"), "remote"}
			if strings.HasSuffix(ref.Name, "/HEAD") {
				continue
			}
		case strings.HasPrefix(f[2], "refs/tags/"):
			ref = Ref{strings.TrimPrefix(f[2], "refs/tags/"), "tag"}
		default:
			continue
		}
		st.refs[id] = append(st.refs[id], ref)
	}
	if st.head != "" {
		name := st.branch
		if name == "" {
			name = "HEAD"
		}
		st.refs[st.head] = append(st.refs[st.head], Ref{name, "head"})
	}
	for _, l := range st.refs {
		sort.SliceStable(l, func(i, j int) bool {
			if a, b := kindRank[l[i].Kind], kindRank[l[j].Kind]; a != b {
				return a < b
			}
			return l[i].Name < l[j].Name
		})
	}
	return st, nil
}

// logFormat prints a commit as fields split by 0x1f; -z ends each commit with NUL.
const logFormat = "--format=%H%x1f%P%x1f%an%x1f%ae%x1f%at%x1f%s"

func cutSubject(s string) string {
	if len(s) > maxSubject && utf8.RuneCountInString(s) > maxSubject {
		return string([]rune(s)[:maxSubject])
	}
	return s
}

// parseLog reads whole commits printed with logFormat and -z, adding refs.
// A last record without its NUL is dropped.
func parseLog(out []byte, refs map[string][]Ref) []LogCommit {
	var list []LogCommit
	for len(out) > 0 {
		i := bytes.IndexByte(out, 0)
		if i < 0 {
			break
		}
		rec := string(out[:i])
		out = out[i+1:]
		f := strings.SplitN(rec, "\x1f", 6)
		if len(f) != 6 {
			continue
		}
		c := LogCommit{ID: strings.TrimLeft(f[0], "\n"), Parents: []string{}, Author: f[2], Email: f[3], Subject: cutSubject(f[5]), Refs: []Ref{}}
		if f[1] != "" {
			c.Parents = strings.Fields(f[1])
		}
		c.Time = atoi64(f[4])
		if r := refs[c.ID]; r != nil {
			c.Refs = r
		}
		list = append(list, c)
	}
	return list
}

func atoi64(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}
