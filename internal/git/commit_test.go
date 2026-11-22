package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommit(t *testing.T) {
	r := newRig(t, opts{})
	_, err := r.s.Commit(bg, "p1", "", "nothing")
	r.wantCode(err, "NOTHING_TO_COMMIT")
	_, err = r.s.Commit(bg, "p1", "", "   \n")
	r.wantCode(err, "BAD_REQUEST")
	r.write("a.txt", "a\n")
	r.must(r.s.Stage(bg, "p1", "", []string{""}))
	res, err := r.s.Commit(bg, "p1", "", "add a\n\nbody line")
	r.must(err)
	if res.Branch != "main" || res.Commit != r.git(r.root, "rev-parse", "--short", "HEAD") {
		t.Fatalf("result: %+v", res)
	}
	if got := r.git(r.root, "log", "-1", "--format=%B"); got != "add a\n\nbody line" {
		t.Errorf("message %q", got)
	}
}

func TestCommitFirstCommit(t *testing.T) {
	r := newRig(t, opts{noCommit: true})
	r.write("a.txt", "a\n")
	r.must(r.s.Stage(bg, "p1", "", []string{""}))
	if _, err := r.s.Commit(bg, "p1", "", "first"); err != nil {
		t.Fatal(err)
	}
}

func TestCommitNoIdentity(t *testing.T) {
	r := newRig(t, opts{noIdentity: true})
	r.write("a.txt", "a\n")
	r.must(r.s.Stage(bg, "p1", "", []string{""}))
	_, err := r.s.Commit(bg, "p1", "", "who am i")
	r.wantCode(err, "NO_IDENTITY")
}

func TestCommitHookFailed(t *testing.T) {
	r := newRig(t, opts{})
	hook := filepath.Join(r.root, ".git", "hooks", "pre-commit")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho 'lint says no' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.write("a.txt", "a\n")
	r.must(r.s.Stage(bg, "p1", "", []string{""}))
	_, err := r.s.Commit(bg, "p1", "", "blocked")
	r.wantCode(err, "HOOK_FAILED")
	if !strings.Contains(err.Error(), "lint says no") {
		t.Errorf("hook output missing: %v", err)
	}
}

func TestPush(t *testing.T) {
	r := newRig(t, opts{})
	r.git(r.root, "remote", "add", "origin", r.remote)
	_, err := r.s.Push(bg, "p1", "", false)
	r.wantCode(err, "NO_UPSTREAM")
	res, err := r.s.Push(bg, "p1", "", true)
	r.must(err)
	if res.Upstream != "origin/main" {
		t.Fatalf("push: %+v", res)
	}
	if st := r.status(""); st.Upstream != "origin/main" || st.Ahead != 0 {
		t.Fatalf("status: %+v", st)
	}
	r.write("b.txt", "b\n")
	r.commitAll("b")
	if st := r.status(""); st.Ahead != 1 {
		t.Fatalf("ahead = %d", st.Ahead)
	}
	if _, err := r.s.Push(bg, "p1", "", false); err != nil {
		t.Fatal(err)
	}
	if r.git(r.remote, "rev-parse", "main") != r.git(r.root, "rev-parse", "HEAD") {
		t.Error("remote did not get the commit")
	}

	// Someone else pushes first.
	other := filepath.Join(r.base, "other")
	r.git(r.base, "clone", "-q", r.remote, other)
	r.git(other, "config", "user.name", "O")
	r.git(other, "config", "user.email", "o@example.com")
	if err := os.WriteFile(filepath.Join(other, "o.txt"), []byte("o\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.git(other, "add", "-A")
	r.git(other, "commit", "-q", "-m", "other")
	r.git(other, "push", "-q")
	r.write("c.txt", "c\n")
	r.commitAll("c")
	_, err = r.s.Push(bg, "p1", "", false)
	r.wantCode(err, "PUSH_REJECTED")
}

func TestPushNoRemoteAndDetached(t *testing.T) {
	r := newRig(t, opts{})
	_, err := r.s.Push(bg, "p1", "", true)
	r.wantCode(err, "NO_REMOTE")
	r.git(r.root, "checkout", "-q", "--detach")
	_, err = r.s.Push(bg, "p1", "", true)
	r.wantCode(err, "BAD_REQUEST")
	if st := r.status(""); !st.Detached || st.Branch != "" {
		t.Fatalf("detached status: %+v", st)
	}
}

func TestCredentialsAreScrubbed(t *testing.T) {
	got := clean("fatal: unable to access 'https://user:secret@example.com/x.git/': nope")
	if strings.Contains(got, "secret") || strings.Contains(got, "user") {
		t.Fatalf("not scrubbed: %s", got)
	}
}

func TestStaged(t *testing.T) {
	r := newRig(t, opts{})
	stat, diff, err := r.s.Staged(bg, "p1", "")
	r.must(err)
	if stat != "" || diff != "" {
		t.Fatalf("nothing staged, got %q %q", stat, diff)
	}
	r.write("a.txt", "alpha\n")
	r.must(r.s.Stage(bg, "p1", "", []string{""}))
	stat, diff, err = r.s.Staged(bg, "p1", "")
	r.must(err)
	if !strings.Contains(stat, "a.txt") || !strings.Contains(diff, "+alpha") {
		t.Fatalf("got %q %q", stat, diff)
	}
}
