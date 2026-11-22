package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pushed gives the rig a pushed main with an upstream, and returns a second
// clone that can push to the same remote.
func pushed(t *testing.T, r *rig) string {
	t.Helper()
	r.git(r.root, "remote", "add", "origin", r.remote)
	_, err := r.s.Push(bg, "p1", "", true)
	r.must(err)
	other := filepath.Join(r.base, "other")
	r.git(r.base, "clone", "-q", r.remote, other)
	r.git(other, "config", "user.name", "O")
	r.git(other, "config", "user.email", "o@example.com")
	r.git(other, "config", "commit.gpgsign", "false")
	return other
}

func pushFrom(t *testing.T, r *rig, other, file string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(other, file), []byte(file+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.git(other, "add", "-A")
	r.git(other, "commit", "-q", "-m", "other "+file)
	r.git(other, "push", "-q")
}

func TestFetchMovesBehind(t *testing.T) {
	r := newRig(t, opts{})
	other := pushed(t, r)
	pushFrom(t, r, other, "o1.txt")
	pushFrom(t, r, other, "o2.txt")
	if st := r.status(""); st.Behind != 0 {
		t.Fatalf("behind before fetch: %d", st.Behind)
	}
	res, err := r.s.Fetch(bg, "p1", "")
	r.must(err)
	if res.Upstream != "origin/main" || res.New != 2 {
		t.Fatalf("fetch: %+v", res)
	}
	if st := r.status(""); st.Behind != 2 {
		t.Fatalf("behind after fetch: %d", st.Behind)
	}
	res, err = r.s.Fetch(bg, "p1", "")
	r.must(err)
	if res.New != 0 {
		t.Fatalf("second fetch: %+v", res)
	}
}

func TestPullFastForwards(t *testing.T) {
	r := newRig(t, opts{})
	other := pushed(t, r)
	pushFrom(t, r, other, "o1.txt")
	res, err := r.s.Pull(bg, "p1", "")
	r.must(err)
	if res.Pulled != 1 || res.Upstream != "origin/main" {
		t.Fatalf("pull: %+v", res)
	}
	if r.read("o1.txt") != "o1.txt\n" {
		t.Error("file did not arrive")
	}
	if st := r.status(""); st.Behind != 0 || st.Ahead != 0 {
		t.Fatalf("status after pull: %+v", st)
	}
	res, err = r.s.Pull(bg, "p1", "")
	r.must(err)
	if res.Pulled != 0 {
		t.Fatalf("up to date pull: %+v", res)
	}
}

func TestPullRefusesDivergedWithoutTouching(t *testing.T) {
	r := newRig(t, opts{})
	other := pushed(t, r)
	pushFrom(t, r, other, "theirs.txt")
	r.write("mine.txt", "mine\n")
	r.commitAll("mine")
	r.write("README.md", "edited, not committed\n")
	head := r.git(r.root, "rev-parse", "HEAD")
	_, err := r.s.Pull(bg, "p1", "")
	r.wantCode(err, "PULL_DIVERGED")
	if !strings.Contains(err.Error(), "Merge or rebase in a terminal") {
		t.Errorf("message: %v", err)
	}
	if got := r.git(r.root, "rev-parse", "HEAD"); got != head {
		t.Error("HEAD moved")
	}
	if r.git(r.root, "rev-list", "--merges", "--count", "HEAD") != "0" {
		t.Error("a merge commit was made")
	}
	if r.read("README.md") != "edited, not committed\n" {
		t.Error("work tree changed")
	}
	if _, err := os.Stat(filepath.Join(r.root, "theirs.txt")); err == nil {
		t.Error("their file arrived")
	}
}

func TestPullNeedsUpstreamAndBranch(t *testing.T) {
	r := newRig(t, opts{})
	r.git(r.root, "remote", "add", "origin", r.remote)
	_, err := r.s.Pull(bg, "p1", "")
	r.wantCode(err, "NO_UPSTREAM")
	r.git(r.root, "checkout", "-q", "--detach")
	_, err = r.s.Pull(bg, "p1", "")
	r.wantCode(err, "BAD_REQUEST")
}

func TestNetworkFailureLeaksNoCredentials(t *testing.T) {
	r := newRig(t, opts{})
	r.git(r.root, "remote", "add", "origin", "http://alice:hunter2secret@127.0.0.1:1/x.git")
	r.git(r.root, "config", "branch.main.remote", "origin")
	r.git(r.root, "config", "branch.main.merge", "refs/heads/main")
	for _, call := range []func() error{
		func() error { _, err := r.s.Fetch(bg, "p1", ""); return err },
		func() error { _, err := r.s.Pull(bg, "p1", ""); return err },
		func() error { _, err := r.s.Push(bg, "p1", "", false); return err },
	} {
		err := call()
		if err == nil {
			t.Fatal("expected a failure")
		}
		if strings.Contains(err.Error(), "hunter2secret") || strings.Contains(err.Error(), "alice") {
			t.Errorf("credentials in error: %v", err)
		}
	}
}

func TestSSHCommandGetsBatchMode(t *testing.T) {
	r := newRig(t, opts{})
	pl, err := r.s.need("p1", "")
	r.must(err)
	cases := []struct{ env, cfg, want string }{
		{"", "", "ssh -o BatchMode=yes"},
		{"ssh -i k", "", "ssh -i k -o BatchMode=yes"},
		{"", "ssh -p 2222", "ssh -p 2222 -o BatchMode=yes"},
		{"/usr/bin/ssh -i k", "", "/usr/bin/ssh -i k -o BatchMode=yes"},
		{"plink -batch", "", ""},
		{"ssh -o BatchMode=no", "", ""},
		{"", "my-wrapper --x", ""},
	}
	for _, c := range cases {
		r.s.cfg.Env = append([]string(nil), r.env...)
		if c.env != "" {
			r.s.cfg.Env = append(r.s.cfg.Env, "GIT_SSH_COMMAND="+c.env)
		}
		if c.cfg != "" {
			r.git(r.root, "config", "core.sshCommand", c.cfg)
		} else {
			r.git(r.root, "config", "core.sshCommand", "")
		}
		got, _ := r.s.netCtx(bg, pl).Value(sshKey{}).(string)
		if got != c.want {
			t.Errorf("env %q cfg %q: got %q, want %q", c.env, c.cfg, got, c.want)
		}
	}
}
