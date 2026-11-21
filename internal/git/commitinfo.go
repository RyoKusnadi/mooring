package git

import (
	"bytes"
	"context"
	"path"
	"regexp"
	"strings"
)

const (
	maxCommitFiles = 5000
	maxFileList    = 16 << 20
)

// CommitFile is one file a commit changed.
type CommitFile struct {
	Path     string `json:"path"`
	OrigPath string `json:"origPath,omitempty"`
	Change   string `json:"change"`
	Binary   bool   `json:"binary"`
	// untracked marks a file of a stash's untracked commit.
	untracked bool
}

// CommitInfo is the answer of GET /api/git/commit.
type CommitInfo struct {
	Commit    LogCommit    `json:"commit"`
	Message   string       `json:"message"`
	Committer string       `json:"committer"`
	Files     []CommitFile `json:"files"`
	Truncated bool         `json:"truncated"`
}

var hexID = regexp.MustCompile(`^[0-9a-fA-F]{4,64}$`)

// resolveCommit turns the id of a request into a full commit id.
func (s *Service) resolveCommit(ctx context.Context, pl *place, id string) (string, error) {
	if !hexID.MatchString(id) {
		return "", apiErr("BAD_REQUEST", "That is not a commit id.")
	}
	out, _, err := s.runX(ctx, pl.top, false, nil, 0, []int{1, 128}, "rev-parse", "-q", "--verify", id+"^{commit}")
	if err != nil {
		return "", s.failure(err)
	}
	full := strings.TrimSpace(string(out))
	if full == "" {
		return "", apiErr("NOT_FOUND", "That commit does not exist.")
	}
	return full, nil
}

// CommitDetail answers the whole message and the changed files of a commit or stash.
func (s *Service) CommitDetail(ctx context.Context, projectID, folder, id string) (*CommitInfo, error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	full, err := s.resolveCommit(ctx, pl, id)
	if err != nil {
		return nil, err
	}
	out, err := s.run(ctx, pl.top, false, nil, "log", "-1", "--no-walk", "--no-show-signature",
		"--format=%H%x1f%P%x1f%an%x1f%ae%x1f%at%x1f%s%x1f%cn <%ce>%x1f%B", full)
	if err != nil {
		return nil, s.failure(err)
	}
	f := strings.SplitN(string(out), "\x1f", 8)
	if len(f) != 8 {
		return nil, apiErr("INTERNAL", "Something went wrong on the server.")
	}
	info := &CommitInfo{Committer: f[6], Message: strings.TrimRight(f[7], "\n"), Files: []CommitFile{}}
	st, err := s.readRefs(ctx, pl)
	if err != nil {
		return nil, err
	}
	info.Commit = parseLog([]byte(f[0]+"\x1f"+f[1]+"\x1f"+f[2]+"\x1f"+f[3]+"\x1f"+f[4]+"\x1f"+f[5]+"\x00"), st.refs)[0]
	files, cut, err := s.commitFiles(ctx, pl, info.Commit)
	if err != nil {
		return nil, err
	}
	info.Files, info.Truncated = files, cut
	return info, nil
}

// isStash tells whether a commit with three parents is a stash with untracked files.
func (s *Service) isStash(ctx context.Context, pl *place, id string) bool {
	out, err := s.run(ctx, pl.top, false, nil, "stash", "list", "--format=%H")
	return err == nil && strings.Contains(string(out), id)
}

// treeArgs are the diff-tree arguments that compare a commit with its first parent.
func treeArgs(c LogCommit) []string {
	if len(c.Parents) > 0 {
		return []string{c.Parents[0], c.ID}
	}
	return []string{"--root", c.ID}
}

func (s *Service) commitFiles(ctx context.Context, pl *place, c LogCommit) ([]CommitFile, bool, error) {
	files, cut, err := s.diffTreeFiles(ctx, pl, treeArgs(c), false)
	if err != nil || cut || len(c.Parents) != 3 || !s.isStash(ctx, pl, c.ID) {
		return files, cut, err
	}
	more, cut, err := s.diffTreeFiles(ctx, pl, []string{"--root", c.Parents[2]}, true)
	if err != nil {
		return nil, false, err
	}
	files = append(files, more...)
	if len(files) > maxCommitFiles {
		files, cut = files[:maxCommitFiles], true
	}
	return files, cut, nil
}

func (s *Service) diffTreeFiles(ctx context.Context, pl *place, rev []string, untracked bool) ([]CommitFile, bool, error) {
	run := func(mode string) ([]byte, bool, error) {
		args := append([]string{"diff-tree", "-r", "-M", "-z", "--no-ext-diff", "--no-commit-id", mode}, rev...)
		out, _, err := s.runX(ctx, pl.top, false, nil, maxFileList, nil, args...)
		if err != nil {
			var re *runError
			if asRun(err, &re) && re.tooLarge {
				return re.partial, true, nil
			}
			return nil, false, s.failure(err)
		}
		return out, false, nil
	}
	status, cut, err := run("--name-status")
	if err != nil {
		return nil, false, err
	}
	nums, _, err := run("--numstat")
	if err != nil {
		return nil, false, err
	}
	binary := binaryPaths(nums)
	files := []CommitFile{}
	tok := bytes.Split(status, []byte{0})
	for i := 0; i < len(tok); {
		st := string(tok[i])
		if st == "" {
			i++
			continue
		}
		n := 2
		if st[0] == 'R' || st[0] == 'C' {
			n = 3
		}
		if i+n > len(tok) || (cut && i+n >= len(tok)) {
			cut = true
			break
		}
		if len(files) == maxCommitFiles {
			return files, true, nil
		}
		f := CommitFile{Path: string(tok[i+n-1]), untracked: untracked}
		if n == 3 {
			f.OrigPath = string(tok[i+1])
		}
		f.Change = changeName(st[0])
		if untracked {
			f.Change = "added"
		}
		f.Binary = binary[f.Path]
		files = append(files, f)
		i += n
	}
	return files, cut, nil
}

func changeName(c byte) string {
	switch c {
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R', 'C':
		return "renamed"
	case 'T':
		return "typechange"
	}
	return "modified"
}

// binaryPaths reads `diff-tree --numstat -z`: binary files print "-" counts.
func binaryPaths(out []byte) map[string]bool {
	res := map[string]bool{}
	tok := bytes.Split(out, []byte{0})
	for i := 0; i < len(tok); i++ {
		t := string(tok[i])
		parts := strings.SplitN(t, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		p := parts[2]
		if p == "" && i+2 < len(tok) {
			p = string(tok[i+2])
			i += 2
		}
		if parts[0] == "-" && parts[1] == "-" {
			res[p] = true
		}
	}
	return res
}

// CommitDiff is the diff of one file in a commit or stash. The path is
// relative to the repository top.
func (s *Service) CommitDiff(ctx context.Context, projectID, folder, id, rel string) (*Diff, error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, err
	}
	if rel == "" || len(rel) > 4096 || strings.ContainsRune(rel, 0) || strings.HasPrefix(rel, "/") || path.Clean(rel) != rel || rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, apiErr("BAD_REQUEST", "That path is not valid. Paths are relative to the repository.")
	}
	full, err := s.resolveCommit(ctx, pl, id)
	if err != nil {
		return nil, err
	}
	pre, err := s.run(ctx, pl.top, false, nil, "rev-parse", full+"^@")
	if err != nil {
		return nil, s.failure(err)
	}
	c := LogCommit{ID: full, Parents: strings.Fields(string(pre))}
	files, _, err := s.commitFiles(ctx, pl, c)
	if err != nil {
		return nil, err
	}
	res := &Diff{Path: rel, Hunks: []Hunk{}}
	var hit *CommitFile
	for i := range files {
		if files[i].Path == rel {
			hit = &files[i]
			break
		}
	}
	if hit == nil {
		return res, nil
	}
	rev := treeArgs(c)
	specs := []string{rel}
	if hit.untracked {
		rev = []string{"--root", c.Parents[2]}
	}
	if hit.OrigPath != "" {
		specs = []string{hit.OrigPath, rel}
		res.OrigPath = hit.OrigPath
	}
	args := append([]string{"diff-tree", "-p", "-M", "--no-commit-id", "--no-ext-diff", "--no-textconv", "--no-color", "-U3", "--src-prefix=a/", "--dst-prefix=b/"}, rev...)
	args = append(append(args, "--"), specs...)
	raw, tooLarge, err := s.readDiff(ctx, pl.top, []int{0}, args)
	if err != nil {
		return nil, err
	}
	fill(res, raw, tooLarge)
	return res, nil
}
