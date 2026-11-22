package git

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ryokusnadi/mooring/internal/files"
	"github.com/ryokusnadi/mooring/internal/paths"
)

// wtDir is where worktrees live inside a project.
const wtDir = ".mooring/worktrees"

// Worktree is a second checkout of the repository on its own branch.
type Worktree struct {
	Folder string `json:"folder"`
	Branch string `json:"branch"`
	Head   string `json:"head"`
	ChatID string `json:"chatId,omitempty"`
	// Unpushed is how many commits of the branch no remote has.
	Unpushed int `json:"unpushed,omitempty"`
}

// worktreeFolder is the place, relative to the project, for a worktree whose checkout is at root.
func (p *place) worktreeFolder(root string) string {
	return joinPrefix(p.folderOf(root), p.prefix)
}

func randomName() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// excludeLine is the info/exclude line that hides the worktrees of the
// project folder, which is prefix inside the repository.
func excludeLine(prefix string) string {
	esc := strings.NewReplacer(`\`, `\\`, "*", `\*`, "?", `\?`, "[", `\[`, " ", `\ `, "#", `\#`, "!", `\!`)
	if prefix == "" {
		return "/.mooring/"
	}
	return "/" + esc.Replace(prefix) + "/.mooring/"
}

func (s *Service) addExclude(ctx context.Context, pl *place) error {
	out, err := s.run(ctx, pl.top, false, nil, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return s.failure(err)
	}
	file := strings.TrimRight(string(out), "\r\n")
	if !filepath.IsAbs(file) {
		file = filepath.Join(pl.top, file)
	}
	line := excludeLine(pl.prefix)
	if f, err := os.Open(file); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if strings.TrimSpace(sc.Text()) == line {
				_ = f.Close()
				return nil
			}
		}
		_ = f.Close()
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return apiErr("INTERNAL", "The exclude file could not be written.")
	}
	prev, _ := os.ReadFile(file)
	add := line + "\n"
	if len(prev) > 0 && prev[len(prev)-1] != '\n' {
		add = "\n" + add
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return apiErr("INTERNAL", "The exclude file could not be written.")
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(add); err != nil {
		return apiErr("INTERNAL", "The exclude file could not be written.")
	}
	return nil
}

// WorktreeAdd makes a worktree of the project's repository, on a new branch
// that starts at the project's current commit.
func (s *Service) WorktreeAdd(ctx context.Context, projectID, branch string) (*Worktree, error) {
	pl, err := s.need(projectID, "")
	if err != nil {
		return nil, err
	}
	var wt *Worktree
	err = s.change(pl, func() error {
		name := randomName()
		if branch == "" {
			branch = "mooring/" + name
		}
		if strings.HasPrefix(branch, "-") || len(branch) > 200 {
			return apiErr("BAD_REQUEST", "That branch name is not valid.")
		}
		if _, _, err := s.runX(ctx, pl.top, false, nil, 0, nil, "check-ref-format", "--branch", branch); err != nil {
			return apiErr("BAD_REQUEST", "%q is not a valid branch name.", branch)
		}
		if _, _, err := s.runX(ctx, pl.top, false, nil, 0, nil, "rev-parse", "--verify", "-q", "HEAD^{commit}"); err != nil {
			return apiErr("NOT_FOUND", "This repository has no commits yet. Commit something first.")
		}
		if _, _, err := s.runX(ctx, pl.top, false, nil, 0, nil, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
			return apiErr("ALREADY_EXISTS", "The branch %q already exists.", branch)
		}
		if err := s.addExclude(ctx, pl); err != nil {
			return err
		}
		root := filepath.Join(pl.root, filepath.FromSlash(wtDir), name)
		if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
			return apiErr("INTERNAL", "The worktree folder could not be made.")
		}
		if _, err := s.run(ctx, pl.top, true, nil, "worktree", "add", "-q", "-b", branch, root, "HEAD"); err != nil {
			return s.failure(err)
		}
		real, err := filepath.EvalSymlinks(root)
		if err != nil {
			return apiErr("INTERNAL", "The worktree folder could not be read.")
		}
		wt = &Worktree{Folder: pl.worktreeFolder(real), Branch: branch}
		if out, err := s.run(ctx, real, false, nil, "rev-parse", "--short", "HEAD"); err == nil {
			wt.Head = strings.TrimSpace(string(out))
		}
		return nil
	})
	return wt, err
}

// WorktreeFolder is WorktreeAdd for callers that need only the folder (chats).
func (s *Service) WorktreeFolder(projectID, branch string) (string, string, error) {
	wt, err := s.WorktreeAdd(context.Background(), projectID, branch)
	if err != nil {
		return "", "", err
	}
	return wt.Folder, wt.Branch, nil
}

// DropWorktree removes a worktree that was just made and never used, and its
// branch, which still points at the commit it started from.
func (s *Service) DropWorktree(projectID, folder, branch string) {
	ctx := context.Background()
	if err := s.WorktreeRemove(ctx, projectID, folder); err != nil {
		s.log.Warn("git: drop unused worktree", "err", err)
		return
	}
	if pl, err := s.need(projectID, ""); err == nil {
		_, _ = s.run(ctx, pl.top, true, nil, "branch", "-d", "--", branch)
	}
}

// anyOutput reports whether a git read in dir printed anything.
func (s *Service) anyOutput(ctx context.Context, dir string, args ...string) (bool, error) {
	out, _, err := s.runX(ctx, dir, false, nil, 64<<10, nil, args...)
	var re *runError
	if err != nil && asRun(err, &re) && re.tooLarge {
		return true, nil
	}
	if err != nil {
		return false, s.failure(err)
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

type wtEntry struct{ path, head, branch string }

func parseWorktrees(out []byte) []wtEntry {
	var list []wtEntry
	var cur *wtEntry
	for _, line := range strings.Split(string(out), "\n") {
		k, v, _ := strings.Cut(line, " ")
		switch k {
		case "worktree":
			list = append(list, wtEntry{path: v})
			cur = &list[len(list)-1]
		case "HEAD":
			if cur != nil && len(v) >= 7 {
				cur.head = v[:7]
			}
		case "branch":
			if cur != nil {
				cur.branch = strings.TrimPrefix(v, "refs/heads/")
			}
		}
	}
	return list
}

// Worktrees lists the worktrees under .mooring/worktrees that git knows about.
func (s *Service) Worktrees(ctx context.Context, projectID string) ([]Worktree, error) {
	pl, err := s.need(projectID, "")
	if err != nil {
		return nil, err
	}
	out, err := s.run(ctx, pl.top, false, nil, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, s.failure(err)
	}
	base := filepath.Join(pl.root, filepath.FromSlash(wtDir))
	use := s.chatUse(projectID)
	list := []Worktree{}
	for _, e := range parseWorktrees(out) {
		real := e.path
		if r, err := filepath.EvalSymlinks(e.path); err == nil {
			real = r
		}
		if filepath.Dir(real) != base {
			continue
		}
		w := Worktree{Folder: pl.worktreeFolder(real), Branch: e.branch, Head: e.head}
		if u, ok := use[filepath.Join(real, filepath.FromSlash(pl.prefix))]; ok {
			w.ChatID = u.ID
		}
		w.Unpushed = s.unpushed(ctx, real)
		list = append(list, w)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Folder < list[j].Folder })
	return list, nil
}

// WorktreeRemove removes a worktree; its branch and commits stay.
func (s *Service) WorktreeRemove(ctx context.Context, projectID, folder string) error {
	pl, err := s.need(projectID, "")
	if err != nil {
		return err
	}
	if folder = path.Clean(folder); strings.HasPrefix(folder, "/") || folder == "." {
		return apiErr("BAD_REQUEST", "That is not the folder of a worktree.")
	}
	real, err := paths.Resolve(pl.root, filepath.FromSlash(folder))
	if err != nil {
		return mapPathErr(err)
	}
	base := filepath.Join(pl.root, filepath.FromSlash(wtDir))
	rel, err := filepath.Rel(base, real)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return apiErr("BAD_REQUEST", "That is not the folder of a worktree.")
	}
	root := filepath.Join(base, strings.SplitN(filepath.ToSlash(rel), "/", 2)[0])
	for dir, u := range s.chatUse(projectID) {
		if u.Live && paths.Inside(root, dir) {
			return apiErr("BUSY", "A chat is running in this worktree. Stop it first.")
		}
	}
	return s.change(pl, func() error {
		if s.terminalIn(projectID, root) {
			return apiErr("BUSY", terminalOpen)
		}
		if _, err := os.Lstat(root); err != nil {
			if _, err := s.run(ctx, pl.top, true, nil, "worktree", "prune"); err != nil {
				return s.failure(err)
			}
			return nil
		}
		if dirty, err := s.anyOutput(ctx, root, "status", "--porcelain", "--untracked-files=all"); err != nil {
			return err
		} else if dirty {
			return apiErr("DIRTY", "This worktree has uncommitted changes or new files. Commit or discard them first.")
		}
		// To the trash, so ignored files in it (.env, a local database) can be got back.
		if files.MoveToTrash(s.trash, s.log, root) == nil {
			if _, err := s.run(ctx, pl.top, true, nil, "worktree", "prune"); err != nil {
				return s.failure(err)
			}
			return nil
		}
		if kept, err := s.anyOutput(ctx, root, "status", "--porcelain", "--ignored"); err != nil {
			return err
		} else if kept {
			return apiErr("DIRTY", "This worktree has ignored files, such as .env or node_modules, and there is no trash to move them to. Remove them first.")
		}
		if _, err := s.run(ctx, pl.top, true, nil, "worktree", "remove", root); err != nil {
			return s.failure(err)
		}
		return nil
	})
}
