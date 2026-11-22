package git

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// maxBlame caps git blame's porcelain output; a file past it is refused, not cut.
const maxBlame = 16 << 20

// BlameCommit is a commit that last changed some lines of a file.
type BlameCommit struct {
	Author  string `json:"author"`
	Time    int64  `json:"time"` // Unix seconds
	Summary string `json:"summary"`
}

// BlameRun is lines start..start+count-1 (1-based, as the file is on disk) last
// changed by one commit. Commit is "" for lines that are not committed yet.
type BlameRun struct {
	Start  int    `json:"start"`
	Count  int    `json:"count"`
	Commit string `json:"commit"`
}

// Blame is who last changed each line of a file.
type Blame struct {
	Runs    []BlameRun             `json:"runs"`
	Commits map[string]BlameCommit `json:"commits"`
}

const notCommitted = "0000000000000000000000000000000000000000"

// Blame reads `git blame` of a file in the working tree, so the line numbers
// match the editor; lines changed since the last commit have no commit.
func (s *Service) Blame(ctx context.Context, projectID, folder, file string) (*Blame, error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	if file == "" {
		return nil, apiErr("BAD_REQUEST", "Name the file to read the history of.")
	}
	rel, err := pl.relPath(file)
	if err != nil {
		return nil, err
	}
	if fi, err := os.Stat(filepath.Join(pl.top, filepath.FromSlash(rel))); err == nil && fi.IsDir() {
		return nil, apiErr("BAD_REQUEST", "%s is a folder. Pick a file to see who changed each line.", file)
	}
	out, _, err := s.runX(ctx, pl.top, false, nil, maxBlame, nil, "blame", "--porcelain", "--", rel)
	if err != nil {
		var re *runError
		if asRun(err, &re) {
			switch {
			case re.tooLarge:
				return nil, apiErr("TOO_LARGE", "%s is too large to show who changed each line.", file)
			case strings.Contains(re.stderr, "no such path"), strings.Contains(re.stderr, "no such ref"):
				return nil, apiErr("NOT_TRACKED", "%s is not committed yet, so no line has a history.", file)
			}
		}
		return nil, s.failure(err)
	}
	return parseBlame(out), nil
}

// parseBlame reads `git blame --porcelain`. A header line "<sha> <orig> <final> <count>"
// starts a group of lines; the first time a commit shows, key lines describe it.
func parseBlame(out []byte) *Blame {
	b := &Blame{Runs: []BlameRun{}, Commits: map[string]BlameCommit{}}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 64<<10), maxBlame)
	cur := ""
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "\t") {
			continue // the line's text
		}
		f := strings.Fields(line)
		if len(f) >= 3 && len(f[0]) == 40 && isHex(f[0]) {
			cur = f[0]
			if len(f) < 4 {
				continue // a later line of the same group
			}
			start, err1 := strconv.Atoi(f[2])
			count, err2 := strconv.Atoi(f[3])
			if err1 != nil || err2 != nil || count < 1 {
				continue
			}
			id := cur
			if id == notCommitted {
				id = ""
			}
			if n := len(b.Runs); n > 0 && b.Runs[n-1].Commit == id && b.Runs[n-1].Start+b.Runs[n-1].Count == start {
				b.Runs[n-1].Count += count
			} else {
				b.Runs = append(b.Runs, BlameRun{Start: start, Count: count, Commit: id})
			}
			continue
		}
		if cur == "" || cur == notCommitted {
			continue
		}
		key, val, _ := strings.Cut(line, " ")
		c := b.Commits[cur]
		switch key {
		case "author":
			c.Author = val
		case "author-time":
			c.Time, _ = strconv.ParseInt(val, 10, 64)
		case "summary":
			c.Summary = val
		default:
			continue
		}
		b.Commits[cur] = c
	}
	return b
}

func isHex(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
