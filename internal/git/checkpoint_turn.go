package git

import (
	"context"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// TurnInfo names the two checkpoints of the last finished turn of a chat.
type TurnInfo struct {
	Start     string `json:"start"`
	End       string `json:"end"`
	At        int64  `json:"at"` // when the turn ended, ms since the epoch
	ChatTitle string `json:"chatTitle,omitempty"`
}

// TurnFile is one file the agent changed in a turn. Paths are relative to the chat's folder.
type TurnFile struct {
	Path     string `json:"path"`
	OrigPath string `json:"origPath,omitempty"`
	Kind     string `json:"kind"` // added, modified, deleted, renamed or typechange
	Added    int    `json:"added"`
	Removed  int    `json:"removed"`
}

// LastTurn is the answer of git.lastTurn. Turn is nil when the chat has no
// finished turn with checkpoints.
type LastTurn struct {
	Turn  *TurnInfo  `json:"turn"`
	Files []TurnFile `json:"files"`
}

// turnPlace is where a chat's last turn lives: its repository and folder.
type turnPlace struct {
	top    string
	prefix string // the chat's folder inside top, "" or slash-separated
	info   TurnInfo
}

// lastTurnOf finds the newest end checkpoint of a chat that follows its start.
// ok is false when there is none (or the chat's folder is not in a repository).
func (s *Service) lastTurnOf(ctx context.Context, projectID, chatID string) (tp turnPlace, ok bool, err error) {
	s.mu.Lock()
	look := s.chatInfo
	s.mu.Unlock()
	if look == nil || !cpChatID.MatchString(chatID) {
		return tp, false, nil
	}
	dir, title, found := look(projectID, chatID)
	if !found {
		return tp, false, nil
	}
	top, _, repo, err := s.topOf(ctx, dir)
	if err != nil {
		return tp, false, s.failure(err)
	}
	if !repo {
		return tp, false, nil
	}
	refs, err := s.cpRefs(ctx, top, chatID)
	if err != nil {
		return tp, false, err
	}
	n := len(refs)
	if n < 2 || !refs[n-1].end || refs[n-2].end {
		return tp, false, nil
	}
	tp.top = top
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		if rel, err := filepath.Rel(top, real); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			tp.prefix = filepath.ToSlash(rel)
		}
	}
	tp.info = TurnInfo{Start: refs[n-2].sha, End: refs[n-1].sha, At: refs[n-1].at * 1000, ChatTitle: title}
	return tp, true, nil
}

// LastTurn lists the files that differ between the start and the end of the
// chat's last turn. Edits the user made before the turn are in both and so do
// not show.
func (s *Service) LastTurn(ctx context.Context, projectID, chatID string) (*LastTurn, error) {
	res := &LastTurn{Files: []TurnFile{}}
	tp, ok, err := s.lastTurnOf(ctx, projectID, chatID)
	if err != nil || !ok {
		return res, err
	}
	pl := &place{top: tp.top, prefix: tp.prefix}
	spec := []string{"--"}
	if tp.prefix != "" {
		spec = append(spec, tp.prefix)
	}
	base := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "-M", "-z"}
	names, err := s.turnList(ctx, tp, append(append(append([]string{}, base...), "--name-status", tp.info.Start, tp.info.End), spec...))
	if err != nil {
		return nil, err
	}
	counts, err := s.turnList(ctx, tp, append(append(append([]string{}, base...), "--numstat", tp.info.Start, tp.info.End), spec...))
	if err != nil {
		return nil, err
	}
	type count struct{ add, rem int }
	byPath := map[string]count{}
	for i := 0; i < len(counts); i++ {
		f := strings.SplitN(counts[i], "\t", 3)
		if len(f) < 3 {
			continue
		}
		p := f[2]
		if p == "" && i+2 < len(counts) { // a rename: the two paths follow
			p = counts[i+2]
			i += 2
		}
		a, _ := strconv.Atoi(f[0]) // "-" for a binary file counts as 0
		r, _ := strconv.Atoi(f[1])
		byPath[p] = count{a, r}
	}
	kinds := map[byte]string{'A': "added", 'M': "modified", 'D': "deleted", 'R': "renamed", 'C': "added", 'T': "typechange"}
	for i := 0; i < len(names); i++ {
		st := names[i]
		if st == "" || i+1 >= len(names) {
			continue
		}
		i++
		f := TurnFile{Kind: kinds[st[0]], Path: names[i]}
		if st[0] == 'R' || st[0] == 'C' {
			if i+1 >= len(names) {
				break
			}
			i++
			f.OrigPath, f.Path = names[i-1], names[i]
			f.OrigPath, _ = pl.inPlace(f.OrigPath)
		}
		c := byPath[f.Path]
		f.Added, f.Removed = c.add, c.rem
		f.Path, _ = pl.inPlace(f.Path)
		if f.Kind == "" {
			f.Kind = "modified"
		}
		res.Files = append(res.Files, f)
	}
	res.Turn = &tp.info
	return res, nil
}

// turnList runs a NUL-separated diff listing and returns its fields.
func (s *Service) turnList(ctx context.Context, tp turnPlace, args []string) ([]string, error) {
	out, _, err := s.runCapped(ctx, tp.top, args...)
	if err != nil {
		return nil, s.failure(err)
	}
	f := strings.Split(string(out), "\x00")
	if n := len(f); n > 0 && f[n-1] == "" {
		f = f[:n-1]
	}
	return f, nil
}

// TurnDiff compares one file between the start and the end of the chat's last
// turn. It reads git objects only, never the working tree, so the path needs no
// check against the disk; it must still stay inside the chat's folder.
func (s *Service) TurnDiff(ctx context.Context, projectID, chatID, rel, orig string) (*Diff, error) {
	tp, ok, err := s.lastTurnOf(ctx, projectID, chatID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apiErr("NOT_FOUND", "This chat has no finished turn with checkpoints.")
	}
	pl := &place{top: tp.top, prefix: tp.prefix}
	p, err := pl.turnPath(rel)
	if err != nil {
		return nil, err
	}
	specs := []string{p}
	res := &Diff{Hunks: []Hunk{}, Path: strings.TrimPrefix(rel, "./")}
	if orig != "" {
		o, err := pl.turnPath(orig)
		if err != nil {
			return nil, err
		}
		specs = []string{o, p}
		res.OrigPath = strings.TrimPrefix(orig, "./")
	}
	args := append(append([]string{}, diffBase...), "-M", tp.info.Start, tp.info.End, "--")
	raw, tooLarge, err := s.readDiff(ctx, tp.top, []int{0}, append(args, specs...))
	if err != nil {
		return nil, err
	}
	fill(res, raw, tooLarge)
	return res, nil
}

// turnPath turns a path relative to the chat's folder into one relative to the
// repository top. It rejects absolute paths and any that leave the folder.
func (p *place) turnPath(rel string) (string, error) {
	if rel == "" || len(rel) > 4096 || strings.ContainsRune(rel, 0) || filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", apiErr("BAD_REQUEST", "A diff needs the path of one file, relative to the folder.")
	}
	c := path.Clean(rel)
	if c == "." || c == ".." || strings.HasPrefix(c, "../") {
		return "", apiErr("OUTSIDE_PROJECT", "That path leaves the project folder.")
	}
	if p.prefix != "" {
		c = p.prefix + "/" + c
	}
	return c, nil
}
