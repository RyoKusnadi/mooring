package chat

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ryokusnadi/mooring/internal/git"
)

type failingCheckpoints struct{ calls atomic.Int32 }

func (f *failingCheckpoints) Checkpoint(context.Context, string, string, string, bool) error {
	f.calls.Add(1)
	return errors.New("boom")
}
func (f *failingCheckpoints) DropCheckpoints(context.Context, string, string) error {
	return errors.New("boom")
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestTurnCheckpointsShowOnlyWhatTheAgentWrote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	r := newRig(t)
	dir, err := filepath.EvalSymlinks(r.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range [][]string{{"init", "-q"}, {"config", "user.name", "T"}, {"config", "user.email", "t@example.com"}} {
		gitIn(t, dir, a...)
	}
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "first")
	gs := git.New(git.Config{Dir: func(string) (string, error) { return dir, nil }, TrashDir: "-", Env: []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}})
	gs.SetChatLookup(r.m.ChatFolder)
	r.m.Checkpoints = gs

	// The user's own uncommitted edits, before the turn.
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\nmine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mine.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := r.start("c1")
	r.prompt(c, "WRITEFILE "+filepath.Join(dir, "agent.txt")+" hi")
	waitFor(t, "the turn to end", func() bool {
		return strings.Count(gitOut(t, dir, "for-each-ref", "refs/mooring/cp/c1/"), "\n") == 1 && c.Info().State == StateIdle
	})

	res, err := gs.LastTurn(context.Background(), r.project, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Turn == nil || len(res.Files) != 1 || res.Files[0].Path != "agent.txt" || res.Files[0].Kind != "added" || res.Files[0].Added != 1 {
		t.Fatalf("last turn = %+v %+v", res.Turn, res.Files)
	}

	if _, err := r.run("chat.remove", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	if out := gitOut(t, dir, "for-each-ref", "refs/mooring/cp/"); out != "" {
		t.Fatalf("refs left after the chat was removed:\n%s", out)
	}
}

func TestACheckpointFailureDoesNotFailTheTurn(t *testing.T) {
	r := newRig(t)
	f := &failingCheckpoints{}
	r.m.Checkpoints = f
	c := r.start("c1")
	r.prompt(c, "hello")
	waitFor(t, "the turn to end", func() bool { return f.calls.Load() >= 2 && c.Info().State == StateIdle })
	if c.find(KindError) != nil {
		t.Fatal("the turn shows an error")
	}
	if _, err := r.run("chat.remove", map[string]any{"id": "c1"}); err != nil {
		t.Fatalf("remove: %v", err)
	}
}
