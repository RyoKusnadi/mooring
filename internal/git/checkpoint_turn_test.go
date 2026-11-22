package git

import (
	"context"
	"strings"
	"testing"
)

func (r *rig) lookup(dir string) {
	r.s.SetChatLookup(func(projectID, chatID string) (string, string, bool) {
		if projectID != "p1" || chatID != "c1" {
			return "", "", false
		}
		return dir, "Fix the login", true
	})
}

func (r *rig) lastTurn() *LastTurn {
	r.t.Helper()
	res, err := r.s.LastTurn(context.Background(), "p1", "c1")
	r.must(err)
	return res
}

func TestLastTurnListsOnlyWhatTheTurnChanged(t *testing.T) {
	r := newRig(t, opts{})
	r.lookup(r.root)
	// The user's own edits, made before the turn.
	r.write("README.md", "hello\nmine\n")
	r.write("mine.txt", "mine\n")
	r.write("old.txt", "one\ntwo\n")
	r.commitAll("old")
	r.write("old.txt", "one\ntwo\nthree\n")
	if res := r.lastTurn(); res.Turn != nil || len(res.Files) != 0 {
		t.Fatalf("before any turn: %+v", res)
	}

	r.checkpoint("c1", false)
	if res := r.lastTurn(); res.Turn != nil {
		t.Fatalf("a turn that has not ended shows: %+v", res)
	}
	r.write("agent.txt", "a\nb\n")
	r.write("old.txt", "one\ntwo\nthree\nfour\n")
	r.checkpoint("c1", true)

	res := r.lastTurn()
	if res.Turn == nil || res.Turn.ChatTitle != "Fix the login" || res.Turn.Start == res.Turn.End || res.Turn.At == 0 {
		t.Fatalf("turn = %+v", res.Turn)
	}
	got := map[string]TurnFile{}
	for _, f := range res.Files {
		got[f.Path] = f
	}
	if len(got) != 2 || got["agent.txt"] != (TurnFile{Path: "agent.txt", Kind: "added", Added: 2}) ||
		got["old.txt"] != (TurnFile{Path: "old.txt", Kind: "modified", Added: 1}) {
		t.Fatalf("files = %+v", res.Files)
	}

	d, err := r.s.TurnDiff(context.Background(), "p1", "c1", "old.txt", "")
	r.must(err)
	if d.Path != "old.txt" || len(d.Hunks) != 1 || d.Version == "" {
		t.Fatalf("diff = %+v", d)
	}
	var added []string
	for _, l := range d.Hunks[0].Lines {
		if l[0] == "+" {
			added = append(added, l[1])
		}
	}
	if strings.Join(added, ",") != "four" {
		t.Fatalf("added lines = %v", added)
	}
	_, err = r.s.TurnDiff(context.Background(), "p1", "c1", "../x", "")
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.TurnDiff(context.Background(), "p1", "c1", "/etc/passwd", "")
	r.wantCode(err, "BAD_REQUEST")
}

func TestLastTurnShowsDeletesAndRenames(t *testing.T) {
	r := newRig(t, opts{})
	r.lookup(r.root)
	r.write("gone.txt", "bye\n")
	r.write("moved.txt", "line one\nline two\nline three\nline four\n")
	r.commitAll("files")
	r.checkpoint("c1", false)
	r.git(r.root, "rm", "-q", "-f", "gone.txt")
	r.git(r.root, "mv", "moved.txt", "renamed.txt")
	r.checkpoint("c1", true)
	got := map[string]TurnFile{}
	for _, f := range r.lastTurn().Files {
		got[f.Path] = f
	}
	if got["gone.txt"].Kind != "deleted" || got["gone.txt"].Removed != 1 {
		t.Errorf("gone.txt = %+v", got["gone.txt"])
	}
	if f := got["renamed.txt"]; f.Kind != "renamed" || f.OrigPath != "moved.txt" {
		t.Errorf("renamed.txt = %+v", f)
	}
	d, err := r.s.TurnDiff(context.Background(), "p1", "c1", "renamed.txt", "moved.txt")
	r.must(err)
	if d.OrigPath != "moved.txt" || d.Binary {
		t.Errorf("rename diff = %+v", d)
	}
}

func TestLastTurnInASubfolder(t *testing.T) {
	r := newRig(t, opts{subfolder: "app"})
	r.lookup(r.root + "/app")
	r.checkpoint("c1", false)
	r.write("app/in.txt", "x\n")
	r.write("outside.txt", "y\n")
	r.checkpoint("c1", true)
	res := r.lastTurn()
	if len(res.Files) != 1 || res.Files[0].Path != "in.txt" {
		t.Fatalf("files = %+v", res.Files)
	}
	d, err := r.s.TurnDiff(context.Background(), "p1", "c1", "in.txt", "")
	r.must(err)
	if len(d.Hunks) != 1 {
		t.Fatalf("diff = %+v", d)
	}
}

func TestLastTurnUsesTheNewestFinishedTurn(t *testing.T) {
	r := newRig(t, opts{})
	r.lookup(r.root)
	r.checkpoint("c1", false)
	r.write("first.txt", "1\n")
	r.checkpoint("c1", true)
	r.checkpoint("c1", false)
	r.write("second.txt", "2\n")
	r.checkpoint("c1", true)
	res := r.lastTurn()
	if len(res.Files) != 1 || res.Files[0].Path != "second.txt" {
		t.Fatalf("files = %+v", res.Files)
	}
	if _, err := r.s.LastTurn(context.Background(), "p1", "other"); err != nil {
		t.Fatal(err)
	}
	if res, _ := r.s.LastTurn(context.Background(), "p1", "other"); res.Turn != nil {
		t.Fatalf("an unknown chat has a turn: %+v", res)
	}
	_, err := r.s.TurnDiff(context.Background(), "p1", "other", "a.txt", "")
	r.wantCode(err, "NOT_FOUND")
}
