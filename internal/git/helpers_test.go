package git

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

type recorder struct {
	mu   sync.Mutex
	kind []string
	data []json.RawMessage
}

func (r *recorder) Publish(kind string, data any) {
	raw, _ := json.Marshal(data)
	r.mu.Lock()
	r.kind, r.data = append(r.kind, kind), append(r.data, raw)
	r.mu.Unlock()
}

func (r *recorder) count(kind string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, k := range r.kind {
		if k == kind {
			n++
		}
	}
	return n
}

type rig struct {
	t      *testing.T
	s      *Service
	rec    *recorder
	base   string
	root   string // the project folder, a repository
	remote string // a bare repository
	env    []string
	chats  fakeChats
}

type fakeChats map[string]ChatUse

func (f fakeChats) FolderUse(string) map[string]ChatUse { return f }

type opts struct {
	noIdentity bool
	noRepo     bool
	noTrash    bool
	subfolder  string // the project is this folder of the repository
	noCommit   bool
}

func newRig(t *testing.T, o opts) *rig {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "home")
	r := &rig{t: t, rec: &recorder{}, base: base, root: filepath.Join(base, "proj"), remote: filepath.Join(base, "remote.git"), chats: fakeChats{}}
	r.env = []string{"HOME=" + home, "XDG_CONFIG_HOME=" + home, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	for _, d := range []string{home, r.root} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	project := r.root
	if o.subfolder != "" {
		project = filepath.Join(r.root, o.subfolder)
		if err := os.MkdirAll(project, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if !o.noRepo {
		r.git(r.root, "init", "-q")
		r.git(r.root, "symbolic-ref", "HEAD", "refs/heads/main")
		if !o.noIdentity {
			r.git(r.root, "config", "user.name", "Test")
			r.git(r.root, "config", "user.email", "test@example.com")
		}
		r.git(r.base, "init", "-q", "--bare", r.remote)
		r.git(r.remote, "symbolic-ref", "HEAD", "refs/heads/main")
		r.git(r.root, "config", "commit.gpgsign", "false")
		if !o.noCommit && !o.noIdentity {
			r.write("README.md", "hello\n")
			if o.subfolder != "" {
				r.write(o.subfolder+"/inner.txt", "inner\n")
			}
			r.git(r.root, "add", "-A")
			r.git(r.root, "commit", "-q", "-m", "first")
		}
	}
	trash := filepath.Join(base, "trash")
	if o.noTrash {
		trash = "-"
	}
	r.s = New(Config{
		Pub: r.rec, Dir: func(id string) (string, error) {
			if id != "p1" {
				return "", &ledger.Error{Code: "NOT_FOUND", Message: "no project"}
			}
			return project, nil
		},
		TrashDir: trash, Env: r.env,
	})
	r.s.SetChats(r.chats)
	return r
}

// git runs git in dir for set-up and fails the test on error.
func (r *rig) git(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), r.env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// write makes a file below the repository root, creating folders.
func (r *rig) write(rel, content string) {
	r.t.Helper()
	p := filepath.Join(r.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *rig) read(rel string) string {
	r.t.Helper()
	b, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(rel)))
	if err != nil {
		r.t.Fatal(err)
	}
	return string(b)
}

func (r *rig) status(folder string) *Status {
	r.t.Helper()
	st, err := r.s.Status(context.Background(), "p1", folder)
	if err != nil {
		r.t.Fatalf("status: %v", err)
	}
	return st
}

func (r *rig) commitAll(msg string) {
	r.t.Helper()
	r.git(r.root, "add", "-A")
	r.git(r.root, "commit", "-q", "-m", msg)
}

// find returns the change for path, or nil.
func find(st *Status, path string) *Change {
	for i := range st.Files {
		if st.Files[i].Path == path {
			return &st.Files[i]
		}
	}
	return nil
}

func code(err error) string {
	var le *ledger.Error
	if errors.As(err, &le) {
		return le.Code
	}
	if err == nil {
		return ""
	}
	return "?" + err.Error()
}

func (r *rig) wantCode(err error, want string) {
	r.t.Helper()
	if got := code(err); got != want {
		r.t.Fatalf("error code = %q, want %q (%v)", got, want, err)
	}
}

func (r *rig) must(err error) {
	r.t.Helper()
	if err != nil {
		r.t.Fatal(err)
	}
}

func newCmd(r *rig, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.root
	cmd.Env = append(os.Environ(), r.env...)
	return cmd
}
