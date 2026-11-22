package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCfg(t *testing.T, e *env, rel, text string) {
	t.Helper()
	p := filepath.Join(e.home, ".ssh", rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func byName(hosts []ConfigHost) map[string]ConfigHost {
	out := map[string]ConfigHost{}
	for _, h := range hosts {
		out[h.Name] = h
	}
	return out
}

func TestSSHConfigIncludeAndMatch(t *testing.T) {
	e := newEnv(t, envOpts{})
	writeCfg(t, e, "id_work", "key")
	writeCfg(t, e, "config", `
# comment
Include conf.d/*.conf
Include missing/*.conf

Host web
  HostName web.example.com
  Port 2222
  User deploy
  IdentityFile ~/.ssh/id_work
  ForwardAgent yes

Host db staging !hidden
  HostName=10.0.0.5

Host bastion-user
  ProxyJump bastion

Match host web user deploy
  Port 2200

Match host db exec "test -f /x"
  User eve

Match originalhost web
  User ignored

Match all
  ServerAliveInterval 5

Host *.example.com
  User wild

Host *
  User fallback
  Port 22
`)
	writeCfg(t, e, "conf.d/a.conf", "Host included\n  HostName inc.example.com\n  Include more.conf\n")
	writeCfg(t, e, "more.conf", "Host deep\n  HostName deep.example.com\n")
	got := byName(e.m.ReadConfig())

	web := got["web"]
	if web.Address != "web.example.com" || web.Port != 2222 || web.User != "deploy" || !web.AgentForward || web.Skipped != "" {
		t.Errorf("web: %+v", web)
	}
	if web.KeyPath != filepath.Join(e.home, ".ssh", "id_work") {
		t.Errorf("web key: %q", web.KeyPath)
	}
	if db := got["db"]; db.Address != "10.0.0.5" || db.User != "fallback" || db.Skipped == "" {
		t.Errorf("db (Match exec is not run): %+v", db)
	}
	if st := got["staging"]; st.Address != "10.0.0.5" {
		t.Errorf("staging: %+v", st)
	}
	if b := got["bastion-user"]; b.Skipped != "jump hosts come later" {
		t.Errorf("jump host: %+v", b)
	}
	if inc := got["included"]; inc.Address != "inc.example.com" || inc.User != "fallback" {
		t.Errorf("included: %+v", inc)
	}
	if got["deep"].Address != "deep.example.com" {
		t.Errorf("a nested include: %+v", got["deep"])
	}
	for _, n := range []string{"*", "*.example.com", "hidden", "!hidden"} {
		if _, ok := got[n]; ok {
			t.Errorf("%q must not be listed", n)
		}
	}
	// The file is read, never written.
	if b, _ := os.ReadFile(filepath.Join(e.home, ".ssh", "config")); len(b) == 0 {
		t.Error("config emptied")
	}
}

func TestSSHConfigIncludeDepthAndMissingFile(t *testing.T) {
	e := newEnv(t, envOpts{})
	if got := e.m.ReadConfig(); len(got) != 0 {
		t.Errorf("no config file: %v", got)
	}
	writeCfg(t, e, "config", "Include loop.conf\nHost top\n  HostName top.example\n")
	writeCfg(t, e, "loop.conf", "Include loop.conf\n")
	got := byName(e.m.ReadConfig())
	if got["top"].Address != "top.example" {
		t.Errorf("a loop of includes: %v", got)
	}
}

func TestSSHConfigKeyOutsideHomeIsDropped(t *testing.T) {
	e := newEnv(t, envOpts{})
	writeCfg(t, e, "config", "Host odd\n  IdentityFile /etc/ssh/ssh_host_key\n")
	if odd := byName(e.m.ReadConfig())["odd"]; odd.KeyPath != "" {
		t.Errorf("a key outside the home folder: %q", odd.KeyPath)
	}
}
