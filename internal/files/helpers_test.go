package files

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

type published struct {
	Kind string
	Data json.RawMessage
}

type recorder struct {
	mu  sync.Mutex
	got []published
}

func (r *recorder) Publish(kind string, data any) {
	raw, _ := json.Marshal(data)
	r.mu.Lock()
	r.got = append(r.got, published{kind, raw})
	r.mu.Unlock()
}

func (r *recorder) all(kind string) []published {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []published
	for _, p := range r.got {
		if p.Kind == kind {
			out = append(out, p)
		}
	}
	return out
}

// wait polls until pred is true for the events of kind.
func (r *recorder) wait(t *testing.T, kind string, pred func(published) bool) published {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, p := range r.all(kind) {
			if pred(p) {
				return p
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no %s event matched; got %v", kind, r.got)
	return published{}
}

type rig struct {
	t       *testing.T
	s       *Service
	rec     *recorder
	root    string // the project folder
	outside string // a folder beside it, holding a secret
}

func newRig(t *testing.T, mod func(*Config)) *rig {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, rec: &recorder{}, root: filepath.Join(base, "proj"), outside: filepath.Join(base, "outside")}
	for _, d := range []string{r.root, r.outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r.write(r.outside+"/secret.txt", "TOPSECRET needle\n")
	cfg := Config{
		Pub: r.rec, Dir: func(id string) (string, error) {
			if id != "p1" {
				return "", &ledger.Error{Code: "NOT_FOUND", Message: "no project"}
			}
			return r.root, nil
		},
		TrashDir: filepath.Join(base, "trash"), Debounce: 20 * time.Millisecond, Lease: time.Minute,
	}
	if mod != nil {
		mod(&cfg)
	}
	r.s = New(cfg)
	t.Cleanup(r.s.Close)
	return r
}

func (r *rig) write(p, content string) {
	r.t.Helper()
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *rig) link(target, name string) {
	r.t.Helper()
	if err := os.Symlink(target, filepath.Join(r.root, name)); err != nil {
		r.t.Fatal(err)
	}
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
