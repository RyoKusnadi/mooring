package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A checkpoint is a commit of the whole working tree (tracked and untracked
// files, not ignored ones), made in a temporary index so the user's index,
// HEAD, branches and stash stay as they were. Each agent turn gets two, one
// before and one after, kept under refs/mooring/cp/<chat>/<n>.
const (
	cpNS        = "refs/mooring/cp/"
	cpKeepTurns = 20
	cpTimeout   = 90 * time.Second
	cpStart     = "start"
	cpEnd       = "end"
)

var cpChatID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// cpIdent is the author of a checkpoint commit; it works without user.name set.
var cpIdent = []string{"-c", "user.name=Mooring", "-c", "user.email=mooring@localhost", "-c", "commit.gpgsign=false"}

// SetCheckpointGate says whether a project saves checkpoints. Without it every
// project does.
func (s *Service) SetCheckpointGate(on func(projectID string) bool) {
	s.mu.Lock()
	s.cpOn = on
	s.mu.Unlock()
}

// SetChatLookup says where to find a chat's working folder and title.
func (s *Service) SetChatLookup(f func(projectID, chatID string) (dir, title string, ok bool)) {
	s.mu.Lock()
	s.chatInfo = f
	s.mu.Unlock()
}

func (s *Service) checkpointsOn(projectID string) bool {
	s.mu.Lock()
	f := s.cpOn
	s.mu.Unlock()
	return f == nil || f(projectID)
}

// cpRef is one saved checkpoint.
type cpRef struct {
	n    int
	sha  string
	name string
	at   int64
	end  bool
}

// topOf finds the work tree and git directory of dir. repo is false outside a repository.
func (s *Service) topOf(ctx context.Context, dir string) (top, gitDir string, repo bool, err error) {
	out, err := s.run(ctx, dir, false, nil, "rev-parse", "--show-toplevel", "--absolute-git-dir")
	if err != nil {
		var re *runError
		if asRun(err, &re) && re.notRepo() {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	lines := strings.Split(strings.TrimRight(string(out), "\r\n"), "\n")
	if len(lines) < 2 {
		return "", "", false, fmt.Errorf("git rev-parse gave %d lines", len(lines))
	}
	top, err = filepath.EvalSymlinks(lines[0])
	if err != nil {
		return "", "", false, err
	}
	return top, lines[1], true, nil
}

// Checkpoint saves the working tree of dir (the folder of a chat) as the start
// or the end of a turn. It does nothing outside a git repository or when the
// project has checkpoints off. Turns older than the newest 20 are pruned.
func (s *Service) Checkpoint(ctx context.Context, projectID, dir, chatID string, end bool) error {
	if !cpChatID.MatchString(chatID) {
		return fmt.Errorf("checkpoint: bad chat id")
	}
	if !s.checkpointsOn(projectID) {
		return nil
	}
	ctx, stop := context.WithTimeout(ctx, cpTimeout)
	defer stop()
	top, gitDir, repo, err := s.topOf(ctx, dir)
	if err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	if !repo {
		s.log.Debug("git: no checkpoint, the folder is not a git repository", "chat", chatID)
		return nil
	}
	defer s.repoLock(top)()
	sha, err := s.writeTree(ctx, top, dir, gitDir, end)
	if err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	refs, err := s.cpRefs(ctx, top, chatID)
	if err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	next := 1
	if len(refs) > 0 {
		next = refs[len(refs)-1].n + 1
	}
	name := cpNS + chatID + "/" + strconv.Itoa(next)
	if _, err := s.run(ctx, top, true, nil, "update-ref", name, sha); err != nil {
		return fmt.Errorf("checkpoint: %w", s.failure(err))
	}
	refs = append(refs, cpRef{n: next, sha: sha, name: name, end: end})
	if err := s.pruneRefs(ctx, top, refs); err != nil {
		s.log.Warn("git: checkpoint prune failed", "chat", chatID, "err", err)
	}
	return nil
}

// writeTree commits the working tree through a private copy of the index. Only dir, the
// chat's folder, is read from disk: a project inside a much larger repository (a home folder
// kept in git) must not have the whole repository copied on every turn.
func (s *Service) writeTree(ctx context.Context, top, dir, gitDir string, end bool) (string, error) {
	tmp, err := os.MkdirTemp(gitDir, "mooring-cp-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	idx := filepath.Join(tmp, "index")
	head, err := s.head(ctx, top)
	if err != nil {
		return "", err
	}
	cx := context.WithValue(ctx, envKey{}, []string{"GIT_INDEX_FILE=" + idx})
	// Copying the index keeps git's stat cache, so a big tree is not read again.
	switch err := copyFile(filepath.Join(gitDir, "index"), idx); {
	case err == nil:
	case !os.IsNotExist(err):
		return "", err
	case head != "":
		if _, err := s.run(cx, top, true, nil, "read-tree", head); err != nil {
			return "", s.failure(err)
		}
	}
	if _, err := s.run(cx, dir, true, nil, "add", "-A", "--ignore-errors", "--", "."); err != nil {
		return "", s.failure(err)
	}
	out, err := s.run(cx, top, true, nil, "write-tree")
	if err != nil {
		return "", s.failure(err)
	}
	args := append([]string{}, cpIdent...)
	args = append(args, "commit-tree", strings.TrimSpace(string(out)))
	if head != "" {
		args = append(args, "-p", head)
	}
	kind := cpStart
	if end {
		kind = cpEnd
	}
	out, err = s.run(cx, top, true, nil, append(args, "-m", kind)...)
	if err != nil {
		return "", s.failure(err)
	}
	return strings.TrimSpace(string(out)), nil
}

// head is the commit HEAD names, or "" in a repository with no commit.
func (s *Service) head(ctx context.Context, top string) (string, error) {
	out, _, err := s.runX(ctx, top, false, nil, 0, []int{1}, "rev-parse", "-q", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", s.failure(err)
	}
	return strings.TrimSpace(string(out)), nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// cpRefs lists the checkpoints of a chat, oldest first.
func (s *Service) cpRefs(ctx context.Context, top, chatID string) ([]cpRef, error) {
	prefix := cpNS + chatID + "/"
	out, err := s.run(ctx, top, false, nil, "for-each-ref", "--format=%(objectname) %(refname) %(committerdate:unix) %(contents:subject)", prefix)
	if err != nil {
		return nil, s.failure(err)
	}
	var refs []cpRef
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(f[1], prefix))
		if err != nil {
			continue
		}
		at, _ := strconv.ParseInt(f[2], 10, 64)
		refs = append(refs, cpRef{n: n, sha: f[0], name: f[1], at: at, end: f[3] == cpEnd})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].n < refs[j].n })
	return refs, nil
}

// pruneRefs deletes the checkpoints of turns older than the newest 20. A turn
// begins at a start checkpoint.
func (s *Service) pruneRefs(ctx context.Context, top string, refs []cpRef) error {
	var starts []int
	for i, r := range refs {
		if !r.end {
			starts = append(starts, i)
		}
	}
	if len(starts) <= cpKeepTurns {
		return nil
	}
	return s.deleteRefs(ctx, top, refs[:starts[len(starts)-cpKeepTurns]])
}

func (s *Service) deleteRefs(ctx context.Context, top string, refs []cpRef) error {
	if len(refs) == 0 {
		return nil
	}
	var b strings.Builder
	for _, r := range refs {
		b.WriteString("delete " + r.name + "\n")
	}
	if _, err := s.run(ctx, top, true, []byte(b.String()), "update-ref", "--stdin"); err != nil {
		return s.failure(err)
	}
	return nil
}

// DropCheckpoints deletes every checkpoint of a chat that worked in dir.
func (s *Service) DropCheckpoints(ctx context.Context, dir, chatID string) error {
	if !cpChatID.MatchString(chatID) {
		return nil
	}
	ctx, stop := context.WithTimeout(ctx, cpTimeout)
	defer stop()
	top, _, repo, err := s.topOf(ctx, dir)
	if err != nil || !repo {
		return err
	}
	defer s.repoLock(top)()
	refs, err := s.cpRefs(ctx, top, chatID)
	if err != nil {
		return err
	}
	return s.deleteRefs(ctx, top, refs)
}
