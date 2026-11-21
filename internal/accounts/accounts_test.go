package accounts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/store"
)

type rec struct {
	mu    sync.Mutex
	kinds []string
}

func (r *rec) Publish(kind string, _ any) {
	r.mu.Lock()
	r.kinds = append(r.kinds, kind)
	r.mu.Unlock()
}

func (r *rec) has(kind string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, k := range r.kinds {
		if k == kind {
			return true
		}
	}
	return false
}

type fakeChats struct {
	live  int
	moved []string
	err   error
}

func (f *fakeChats) LiveOn(string) int { return f.live }
func (f *fakeChats) MoveAccount(_, _, id string) ([]string, error) {
	f.moved = append(f.moved, id)
	return []string{"c1"}, f.err
}

type fakeTerms struct {
	env    []string
	banner string
	id     string
}

func (f *fakeTerms) OpenWith(_, id string, env []string, banner string) error {
	f.id, f.env, f.banner = id, env, banner
	return nil
}

type rig struct {
	t     *testing.T
	s     *Service
	pub   *rec
	home  string
	data  string
	chats *fakeChats
	terms *fakeTerms
}

func realDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func newRig(t *testing.T) *rig {
	t.Helper()
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.DB.Exec(`INSERT INTO projects (id, name, path, position, created_at) VALUES ('p1', 'p', '/x', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	r := &rig{t: t, pub: &rec{}, home: realDir(t), data: realDir(t), chats: &fakeChats{}, terms: &fakeTerms{}}
	r.s, err = New(Config{DB: st.DB, Pub: r.pub, DataDir: r.data, Home: r.home,
		Adapters: map[string]Adapter{"claude": {Env: "CLAUDE_CONFIG_DIR", Login: "claude, then /login", Shared: "projects", Usage: "claude", Home: ".claude"}},
		Agents:   map[string]string{"claude": "Claude", "gemini": "Gemini"}})
	if err != nil {
		t.Fatal(err)
	}
	r.s.SetChats(r.chats)
	r.s.SetTerminals(r.terms)
	return r
}

func (r *rig) run(name string, args map[string]any) (any, error) {
	r.t.Helper()
	raw, _ := json.Marshal(args)
	return r.s.Run(events.Command{ID: "c", Name: name, Args: raw})
}

func (r *rig) save(name string) Account {
	r.t.Helper()
	v, err := r.run("account.save", map[string]any{"account": map[string]any{"agent": "claude", "name": name}})
	if err != nil {
		r.t.Fatalf("save %s: %v", name, err)
	}
	return v.(Account)
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), code) {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func TestManagedFolderHasPrivateModeAndSharedLink(t *testing.T) {
	r := newRig(t)
	a := r.save("Work")
	if !a.Managed || a.Dir != filepath.Join(r.data, "accounts", a.ID) || a.Login != LoginUnknown || a.Usage.Source != "unknown" {
		t.Fatalf("account: %+v", a)
	}
	fi, err := os.Stat(a.Dir)
	if err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("folder mode: %v %v", fi, err)
	}
	link, err := os.Readlink(filepath.Join(a.Dir, "projects"))
	if want := filepath.Join(r.home, ".claude", "projects"); err != nil || link != want {
		t.Fatalf("shared link %q want %q (%v)", link, want, err)
	}
	if fi, err := os.Stat(link); err != nil || !fi.IsDir() {
		t.Fatalf("shared target not made: %v", err)
	}
	env, err := r.s.Env("claude", a.ID)
	if err != nil || len(env) != 1 || env[0] != "CLAUDE_CONFIG_DIR="+a.Dir {
		t.Fatalf("env: %v %v", env, err)
	}
	if env, _ := r.s.Env("claude", "default:claude"); env != nil {
		t.Fatalf("the agent's own login sets nothing: %v", env)
	}
	if !r.pub.has("account.changed") {
		t.Fatal("no account.changed event")
	}
}

func TestSaveChecksNamesFoldersAndLimits(t *testing.T) {
	r := newRig(t)
	r.save("Work")
	_, err := r.run("account.save", map[string]any{"account": map[string]any{"agent": "claude", "name": "work"}})
	wantCode(t, err, "ALREADY_EXISTS")
	_, err = r.run("account.save", map[string]any{"account": map[string]any{"agent": "claude", "name": "default"}})
	wantCode(t, err, "ALREADY_EXISTS")
	_, err = r.run("account.save", map[string]any{"account": map[string]any{"agent": "gemini", "name": "x"}})
	wantCode(t, err, "BAD_REQUEST")
	_, err = r.run("account.save", map[string]any{"account": map[string]any{"agent": "claude", "name": " "}})
	wantCode(t, err, "BAD_REQUEST")
	for _, dir := range []string{"relative/dir", "/etc", filepath.Join(r.home, "missing")} {
		_, err = r.run("account.save", map[string]any{"account": map[string]any{"agent": "claude", "name": "Own", "dir": dir}})
		if err == nil {
			t.Fatalf("folder %q accepted", dir)
		}
	}
	own := filepath.Join(r.home, "own-login")
	if err := os.Mkdir(own, 0o700); err != nil {
		t.Fatal(err)
	}
	v, err := r.run("account.save", map[string]any{"account": map[string]any{"agent": "claude", "name": "Own", "dir": own}})
	if err != nil || v.(Account).Managed || v.(Account).Dir != own {
		t.Fatalf("own folder: %v %v", v, err)
	}
	if _, err := os.Lstat(filepath.Join(own, "projects")); err == nil {
		t.Fatal("shared link made in a folder Mooring does not manage")
	}
	for i := 2; i < MaxPerAgent; i++ {
		r.save("n" + string(rune('a'+i)))
	}
	_, err = r.run("account.save", map[string]any{"account": map[string]any{"agent": "claude", "name": "one more"}})
	wantCode(t, err, "LIMIT_REACHED")
}

func TestRemoveMovesFolderAndResetsProjects(t *testing.T) {
	r := newRig(t)
	a := r.save("Work")
	if _, err := r.run("account.setDefault", map[string]any{"id": a.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.run("project.setAccount", map[string]any{"projectId": "p1", "agent": "claude", "accountId": a.ID}); err != nil {
		t.Fatal(err)
	}
	r.chats.live = 1
	_, err := r.run("account.remove", map[string]any{"id": a.ID})
	wantCode(t, err, "BUSY")
	r.chats.live = 0
	_, err = r.run("account.remove", map[string]any{"id": "default:claude"})
	wantCode(t, err, "BAD_REQUEST")
	if _, err := r.run("account.remove", map[string]any{"id": a.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.data, "accounts", "removed", a.ID)); err != nil {
		t.Fatalf("folder was not kept: %v", err)
	}
	if _, err := os.Stat(a.Dir); err == nil {
		t.Fatal("folder still in place")
	}
	if got := r.s.Effective("p1", "claude"); got != "default:claude" {
		t.Fatalf("project account after removal: %q", got)
	}
	if !r.pub.has("account.removed") || !r.pub.has("project.accountChanged") {
		t.Fatal("events missing")
	}
}

func TestProjectAccountAndDefault(t *testing.T) {
	r := newRig(t)
	a, b := r.save("A"), r.save("B")
	if got := r.s.Effective("p1", "claude"); got != "default:claude" {
		t.Fatalf("start: %q", got)
	}
	if v, err := r.run("account.setDefault", map[string]any{"id": a.ID}); err != nil || !v.(Account).IsDefault {
		t.Fatalf("setDefault: %v %v", v, err)
	}
	if got := r.s.Effective("p1", "claude"); got != a.ID {
		t.Fatalf("default not used: %q", got)
	}
	v, err := r.run("project.setAccount", map[string]any{"projectId": "p1", "agent": "claude", "accountId": b.ID})
	if err != nil || len(v.(map[string]any)["restarted"].([]string)) != 1 || r.chats.moved[0] != b.ID {
		t.Fatalf("setAccount: %v %v %v", v, err, r.chats.moved)
	}
	_, err = r.run("project.setAccount", map[string]any{"projectId": "nope", "agent": "claude", "accountId": b.ID})
	wantCode(t, err, "NOT_FOUND")
	_, err = r.run("project.setAccount", map[string]any{"projectId": "p1", "agent": "gemini", "accountId": b.ID})
	wantCode(t, err, "BAD_REQUEST")
	if got, err := r.s.Pick("p1", "claude", "", false); err != nil || got != b.ID {
		t.Fatalf("pick: %q %v", got, err)
	}
	if _, err := r.s.Pick("p1", "claude", "zz", false); err == nil {
		t.Fatal("unknown account picked")
	}
	if got, err := r.s.Pick("p1", "claude", "zz", true); err != nil || got != b.ID {
		t.Fatalf("lenient pick: %q %v", got, err)
	}
	body, _ := r.s.Snapshot()
	if body["projects"].(map[string]map[string]string)["p1"]["claude"] != b.ID {
		t.Fatalf("snapshot: %v", body["projects"])
	}
	if _, err := r.s.DB.Exec(`DELETE FROM projects WHERE id = 'p1'`); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = r.s.DB.QueryRow(`SELECT COUNT(*) FROM project_accounts`).Scan(&n)
	if n != 0 {
		t.Fatal("project account kept after the project went")
	}
}

func TestLoginOpensTerminalWithFolderAndHint(t *testing.T) {
	r := newRig(t)
	a := r.save("Work")
	v, err := r.run("account.login", map[string]any{"id": a.ID, "projectId": "p1"})
	if err != nil || v.(map[string]string)["terminalId"] != r.terms.id {
		t.Fatalf("login: %v %v", v, err)
	}
	if len(r.terms.env) != 1 || r.terms.env[0] != "CLAUDE_CONFIG_DIR="+a.Dir || !strings.Contains(r.terms.banner, "claude, then /login") {
		t.Fatalf("terminal got %v %q", r.terms.env, r.terms.banner)
	}
}

func TestHTTPListNeedsNoSecrets(t *testing.T) {
	r := newRig(t)
	r.save("Work")
	mux := http.NewServeMux()
	r.s.Mount(mux, func(h http.Handler) http.Handler { return h })
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/accounts", nil))
	var body struct {
		Accounts []Account                    `json:"accounts"`
		Projects map[string]map[string]string `json:"projects"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != 200 || len(body.Accounts) != 2 {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	if body.Accounts[0].ID != "default:claude" || !body.Accounts[0].IsDefault || body.Projects["p1"]["claude"] != "default:claude" {
		t.Fatalf("list body: %+v", body)
	}
}
