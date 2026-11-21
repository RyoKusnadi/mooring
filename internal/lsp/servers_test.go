package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

func program(t *testing.T, dir, name, body string) string {
	body = "PATH=$PATH:/bin:/usr/bin\n" + body
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func code(err error) string {
	if le, ok := err.(*ledger.Error); ok {
		return le.Code
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func serverOf(t *testing.T, e *env, id string) Server {
	t.Helper()
	d, ok, err := e.svc.def(id)
	if err != nil || !ok {
		t.Fatalf("no server %s: %v", id, err)
	}
	return e.svc.view(e.svc.resolve(d))
}

func TestDetectionOrder(t *testing.T) {
	pathDir, extra, cargo := t.TempDir(), t.TempDir(), t.TempDir()
	e := newEnv(t, nil, func(c *Config) { c.PathDirs = []string{pathDir}; c.ExtraDirs = []string{extra, cargo} })
	if s := serverOf(t, e, "go"); s.State != StateNotInstalled || s.Path != "" || !slices.Equal(s.Command, []string{"gopls", "serve"}) {
		t.Fatalf("nothing installed: %+v", s)
	}
	last := program(t, cargo, "gopls", "")
	if got := serverOf(t, e, "go").Path; got != last {
		t.Fatalf("got %s, want the extra folder %s", got, last)
	}
	second := program(t, extra, "gopls", "")
	if got := serverOf(t, e, "go").Path; got != second {
		t.Fatalf("got %s, want %s", got, second)
	}
	onPath := program(t, pathDir, "gopls", "")
	if got := serverOf(t, e, "go").Path; got != onPath {
		t.Fatalf("got %s, want the PATH %s", got, onPath)
	}
	own := program(t, filepath.Join(e.data, "lsp", "bin"), "gopls", "")
	if got := serverOf(t, e, "go").Path; got != own {
		t.Fatalf("got %s, want Mooring's folder %s", got, own)
	}
	// npm installs go to node_modules/.bin; bin comes first.
	ts := program(t, filepath.Join(e.data, "lsp", "node_modules", ".bin"), "typescript-language-server", "")
	if got := serverOf(t, e, "typescript").Path; got != ts {
		t.Fatalf("got %s, want %s", got, ts)
	}
	// A custom command beats all of them.
	custom := program(t, t.TempDir(), "mygopls", "")
	if _, err := e.svc.SetCustom(Def{ID: "go", Name: "Mine", Endings: []string{".go"}, LanguageID: "go", Command: []string{custom, "-x"}}); err != nil {
		t.Fatal(err)
	}
	if s := serverOf(t, e, "go"); s.Path != custom || !s.Custom || !slices.Equal(s.Command, []string{custom, "-x"}) {
		t.Fatalf("custom: %+v", s)
	}
	if err := e.svc.RemoveCustom("go"); err != nil {
		t.Fatal(err)
	}
	if got := serverOf(t, e, "go").Path; got != own {
		t.Fatalf("after removing the override got %s, want %s", got, own)
	}
}

func TestTSServerPathComesFromTheTypescriptNextToTheServer(t *testing.T) {
	e := newEnv(t, nil)
	nm := filepath.Join(e.data, "lsp", "node_modules")
	if err := os.MkdirAll(filepath.Join(nm, "typescript", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(nm, "typescript", "lib", "tsserver.js")
	if err := os.WriteFile(want, []byte("//"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := program(t, filepath.Join(nm, ".bin"), "typescript-language-server", "")
	if got := tsserverPath(bin); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := tsserverPath(program(t, t.TempDir(), "typescript-language-server", "")); got != "" {
		t.Fatalf("no typescript there, got %q", got)
	}
}

func TestConsent(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	s := serverOf(t, e, "go")
	if s.State != StateNeedsConsent || s.Path == "" || s.Command[0] != s.Path {
		t.Fatalf("found, not allowed: %+v", s)
	}
	if _, err := e.svc.Allow("go", []string{"gopls", "serve"}); code(err) != "COMMAND_CHANGED" {
		t.Fatalf("a command that is not what would run: %v", err)
	}
	if _, err := e.svc.Allow("python", []string{"x"}); code(err) != "NOT_INSTALLED" {
		t.Fatalf("not installed: %v", err)
	}
	if _, err := e.svc.Allow("nope", nil); code(err) != "NOT_FOUND" {
		t.Fatalf("unknown: %v", err)
	}
	v, err := e.svc.Allow("go", s.Command)
	if err != nil || v.State != StateReady {
		t.Fatalf("allow: %+v, %v", v, err)
	}
	e.pub.wait(t, "lsp.changed", func(m map[string]any) bool { return m["server"].(map[string]any)["state"] == "ready" })
	// A different file at the same name (the path changed) asks again.
	moved := program(t, t.TempDir(), "gopls", "")
	e.svc.cfg.PathDirs = []string{filepath.Dir(moved)}
	if s := serverOf(t, e, "go"); s.State != StateNeedsConsent || s.Path != moved {
		t.Fatalf("the path changed: %+v", s)
	}
	if _, err := e.svc.Allow("go", s.Command); code(err) != "COMMAND_CHANGED" {
		t.Fatalf("the old command: %v", err)
	}
	// Custom arguments are part of the command.
	if _, err := e.svc.SetCustom(Def{ID: "go", Name: "Go", Endings: []string{".go"}, LanguageID: "go", Command: []string{moved, "serve"}}); err != nil {
		t.Fatal(err)
	}
	c1 := serverOf(t, e, "go").Command
	if _, err := e.svc.Allow("go", c1); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetCustom(Def{ID: "go", Name: "Go", Endings: []string{".go"}, LanguageID: "go", Command: []string{moved, "serve", "-rpc.trace"}}); err != nil {
		t.Fatal(err)
	}
	if s := serverOf(t, e, "go"); s.State != StateNeedsConsent {
		t.Fatalf("new arguments: %+v", s)
	}
}

func TestConsentIsStored(t *testing.T) {
	e := newEnv(t, []string{"gopls"})
	argv := e.allow("go")
	var raw string
	if err := e.svc.db.QueryRow(`SELECT command FROM lsp_consent WHERE server_id = 'go'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var got []string
	_ = json.Unmarshal([]byte(raw), &got)
	if !slices.Equal(got, argv) {
		t.Fatalf("stored %v, want %v", got, argv)
	}
}

func TestCustomServers(t *testing.T) {
	e := newEnv(t, []string{"mine"})
	ok := Def{ID: "my-lang", Name: "Mine", Endings: []string{".ml2"}, LanguageID: "ml2", Command: []string{"mine", "--stdio"}}
	v, err := e.svc.SetCustom(ok)
	if err != nil || !v.Custom || v.State != StateNeedsConsent || !slices.Equal(v.Endings, []string{".ml2"}) {
		t.Fatalf("set: %+v, %v", v, err)
	}
	for name, bad := range map[string]func(*Def){
		"id upper": func(d *Def) { d.ID = "My" },
		"id empty": func(d *Def) { d.ID = "" },
		"id long":  func(d *Def) { d.ID = strings.Repeat("a", 33) },
		"name":     func(d *Def) { d.Name = " " },
		"lang":     func(d *Def) { d.LanguageID = "" },
		"endings":  func(d *Def) { d.Endings = nil },
		"ending":   func(d *Def) { d.Endings = []string{"py"} },
		"command":  func(d *Def) { d.Command = nil },
	} {
		d := ok
		bad(&d)
		if _, err := e.svc.SetCustom(d); code(err) != "BAD_REQUEST" {
			t.Errorf("%s: %v", name, err)
		}
	}
	list, _ := e.svc.List()
	servers := list.(map[string]any)["servers"].([]Server)
	if last := servers[len(servers)-1]; last.ID != "my-lang" {
		t.Fatalf("the custom server is not listed last: %v", last.ID)
	}
	if err := e.svc.RemoveCustom("my-lang"); err != nil {
		t.Fatal(err)
	}
	e.pub.wait(t, "lsp.removed", func(m map[string]any) bool { return m["id"] == "my-lang" })
	if err := e.svc.RemoveCustom("my-lang"); code(err) != "NOT_FOUND" {
		t.Fatalf("second remove: %v", err)
	}
}

func TestInstall(t *testing.T) {
	fakeGo := `echo "downloading $2"; mkdir -p "$GOBIN"; printf '#!/bin/sh\n' > "$GOBIN/gopls"; chmod +x "$GOBIN/gopls"`
	fakeNpm := `prefix=; while [ $# -gt 0 ]; do [ "$1" = --prefix ] && prefix=$2; shift; done
mkdir -p "$prefix/node_modules/.bin"; printf '#!/bin/sh\n' > "$prefix/node_modules/.bin/typescript-language-server"; chmod +x "$prefix/node_modules/.bin/typescript-language-server"
echo "added 2 packages from https://user:hunter2@registry.example/x"`
	fakeBin := t.TempDir()
	program(t, fakeBin, "go", fakeGo)
	program(t, fakeBin, "npm", fakeNpm)
	program(t, fakeBin, "rustup", `echo "rustup: no network" >&2; exit 1`)
	e := newEnv(t, nil, func(c *Config) { c.PathDirs = []string{fakeBin} })

	v, err := e.svc.Install(context.Background(), "go")
	if err != nil || v.State != StateNeedsConsent || !strings.HasPrefix(v.Path, filepath.Join(e.data, "lsp", "bin")) {
		t.Fatalf("go install: %+v, %v", v, err)
	}
	e.pub.wait(t, "lsp.install.progress", func(m map[string]any) bool {
		return m["id"] == "go" && m["line"] == "downloading golang.org/x/tools/gopls@latest"
	})
	e.pub.wait(t, "lsp.changed", func(m map[string]any) bool { return m["server"].(map[string]any)["state"] == "installing" })

	v, err = e.svc.Install(context.Background(), "typescript")
	if err != nil || !strings.HasPrefix(v.Path, filepath.Join(e.data, "lsp", "node_modules", ".bin")) {
		t.Fatalf("npm install: %+v, %v", v, err)
	}
	line := e.pub.wait(t, "lsp.install.progress", func(m map[string]any) bool {
		return m["id"] == "typescript" && strings.HasPrefix(m["line"].(string), "added 2")
	})["line"].(string)
	if strings.Contains(line, "hunter2") {
		t.Fatalf("a password reached the progress line: %q", line)
	}

	_, err = e.svc.Install(context.Background(), "rust")
	if code(err) != "INSTALL_FAILED" || !strings.Contains(err.Error(), "no network") {
		t.Fatalf("failing installer: %v", err)
	}
	if serverOf(t, e, "rust").State != StateNotInstalled {
		t.Fatal("a failed install left the server in the wrong state")
	}
	if _, err := e.svc.Install(context.Background(), "c"); code(err) != "BAD_REQUEST" {
		t.Fatalf("no install: %v", err)
	}
	if _, err := e.svc.Install(context.Background(), "nope"); code(err) != "NOT_FOUND" {
		t.Fatalf("unknown: %v", err)
	}
}

func TestInstallerMissing(t *testing.T) {
	e := newEnv(t, nil) // nothing on the PATH
	_, err := e.svc.Install(context.Background(), "python")
	if code(err) != "INSTALLER_MISSING" || !strings.Contains(err.Error(), "npm") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(e.data, "lsp")); err == nil {
		t.Fatal("a folder was made although nothing was installed")
	}
}

func TestScrub(t *testing.T) {
	for in, banned := range map[string]string{
		"fetch https://bob:secretpw@git.example/x failed": "secretpw",
		"Authorization: Bearer abcdef123456":              "abcdef123456",
		"npm_config_token=npm_XYZ123 set":                 "npm_XYZ123",
		`password: "p w"`:                                 "p w",
		"API_KEY = sk-12345":                              "sk-12345",
		`{"api_key": "sk-live-999", "x": 1}`:              "sk-live-999",
		`{"password":"hunter2"}`:                          "hunter2",
		"run --token abc123xyz now":                       "abc123xyz",
		"run -password 'two words' now":                   "two words",
		"Cookie: sid=111; theme=dark; csrf=222":           "csrf=222",
		"Set-Cookie: a=1; Path=/":                         "a=1",
		`"Authorization": "Basic dXNlcjpwYXNz"`:           "dXNlcjpwYXNz",
		"--client-secret=topsecretvalue":                  "topsecretvalue",
	} {
		if out := scrub(in); strings.Contains(out, banned) {
			t.Errorf("%q -> %q still has %q", in, out, banned)
		}
	}
	for _, plain := range []string{"plain error: cannot find module", "token count is fine", "run --verbose now"} {
		if out := scrub(plain); out != plain {
			t.Errorf("plain text changed: %q -> %q", plain, out)
		}
	}
}
