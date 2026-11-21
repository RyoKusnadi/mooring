package chat

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryokusnadi/mooring/internal/git"
)

// chatFolders is what the server wires between chats and git.
type chatFolders struct{ m *Manager }

func (c chatFolders) FolderUse(id string) map[string]git.ChatUse {
	out := map[string]git.ChatUse{}
	for _, u := range c.m.FolderUse(id) {
		out[u.Dir] = git.ChatUse{ID: u.ID, Live: u.Live}
	}
	return out
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestChatStartInWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	r := newRig(t)
	real, err := filepath.EvalSymlinks(r.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range [][]string{{"init", "-q"}, {"config", "user.name", "T"}, {"config", "user.email", "t@example.com"},
		{"commit", "-q", "--allow-empty", "-m", "first"}} {
		gitIn(t, real, a...)
	}
	gs := git.New(git.Config{Dir: func(string) (string, error) { return real, nil }, TrashDir: "-"})
	gs.SetChats(chatFolders{r.m})
	r.m.Worktrees = gs

	if _, err := r.run("chat.start", map[string]any{"id": "w1", "projectId": r.project,
		"spec": map[string]any{"folder": map[string]any{"worktree": true, "branch": "fix-login"}}}); err != nil {
		t.Fatalf("start: %v", err)
	}
	c := r.m.get("w1")
	r.waitState(c, StateIdle)
	wts, err := gs.Worktrees(context.Background(), r.project)
	if err != nil || len(wts) != 1 || wts[0].Branch != "fix-login" || wts[0].ChatID != "w1" {
		t.Fatalf("worktrees: %+v %v", wts, err)
	}
	want := filepath.Join(real, filepath.FromSlash(wts[0].Folder))
	if c.dir != want || c.spec.Folder.Path != want || !c.spec.Folder.Worktree {
		t.Fatalf("chat dir %q, spec %+v, want %q", c.dir, c.spec.Folder, want)
	}
	if !strings.Contains(c.spec.marshal(), want) {
		t.Errorf("saved spec lacks the worktree folder: %s", c.spec.marshal())
	}
	if err := gs.WorktreeRemove(context.Background(), r.project, wts[0].Folder); !strings.HasPrefix(err.Error(), "BUSY") {
		t.Fatalf("remove while running: %v", err)
	}
	_, _ = r.run("chat.close", map[string]any{"id": "w1"})
	r.waitState(c, StateStopped)
	if err := gs.WorktreeRemove(context.Background(), r.project, wts[0].Folder); err != nil {
		t.Fatalf("remove after stop: %v", err)
	}

	// Both a folder and a worktree is refused, and nothing is created.
	_, err = r.run("chat.start", map[string]any{"id": "w2", "projectId": r.project,
		"spec": map[string]any{"folder": map[string]any{"worktree": true, "path": "x"}}})
	if err == nil || !strings.HasPrefix(err.Error(), "BAD_REQUEST") {
		t.Fatalf("worktree plus path: %v", err)
	}
}

type fakeWorktrees struct{ made, dropped int }

func (f *fakeWorktrees) WorktreeFolder(string, string) (string, string, error) {
	f.made++
	return "", "mooring/x", nil
}
func (f *fakeWorktrees) DropWorktree(string, string, string) { f.dropped++ }

// A chat that cannot start does not leave a worktree behind.
func TestFailedChatStartDropsItsWorktree(t *testing.T) {
	r := newRig(t)
	fw := &fakeWorktrees{}
	r.m.Worktrees = fw
	spec := map[string]any{"folder": map[string]any{"worktree": true}}

	r.m.mu.Lock()
	r.m.closed = true
	r.m.mu.Unlock()
	if _, err := r.run("chat.start", map[string]any{"id": "d1", "projectId": r.project, "spec": spec}); err == nil {
		t.Fatal("start on a closing server succeeded")
	}
	if fw.made != 1 || fw.dropped != 1 {
		t.Fatalf("made %d, dropped %d", fw.made, fw.dropped)
	}
	r.m.mu.Lock()
	r.m.closed = false
	r.m.mu.Unlock()

	// A full project is refused before a worktree is made.
	for i := 0; i < MaxPerProject; i++ {
		r.start(string(rune('a' + i)))
	}
	_, err := r.run("chat.start", map[string]any{"id": "d2", "projectId": r.project, "spec": spec})
	if err == nil || !strings.HasPrefix(err.Error(), "LIMIT_REACHED") {
		t.Fatalf("want LIMIT_REACHED, got %v", err)
	}
	if fw.made != 1 {
		t.Fatalf("a worktree was made for a full project")
	}
}
