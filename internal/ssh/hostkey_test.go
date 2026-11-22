package ssh

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ryokusnadi/mooring/internal/fakessh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestUnknownHostKeyPromptsAndIsSavedAfterAccept(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	prompted := 0
	r := func(p *prompt) map[string]any {
		if p.Kind == "hostkey" {
			prompted++
			if !strings.HasPrefix(p.Fingerprint, "SHA256:") || p.KeyType == "" {
				t.Errorf("prompt: %+v", p)
			}
		}
		return standard(testPassword)(p)
	}
	if _, err := e.connect("c1", h, r); err != nil {
		t.Fatal(err)
	}
	if prompted != 1 {
		t.Fatalf("host key prompts: %d", prompted)
	}
	raw, err := os.ReadFile(filepath.Join(e.data, "known_hosts"))
	if err != nil || !strings.Contains(string(raw), "["+"127.0.0.1"+"]:"+strconv.Itoa(e.fake.Port())) {
		t.Fatalf("own known_hosts: %q %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(e.home, ".ssh", "known_hosts")); err == nil {
		t.Error("the user's known_hosts was written")
	}
	if err := e.m.Disconnect("c1"); err != nil {
		t.Fatal(err)
	}
	prompted = 0
	if _, err := e.connect("c2", h, r); err != nil {
		t.Fatal(err)
	}
	if prompted != 0 {
		t.Errorf("a saved key prompted again")
	}
}

func TestUnknownHostKeyRefusedAfterReject(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	_, err := e.connect("c1", h, func(p *prompt) map[string]any {
		if p.Kind == "hostkey" {
			return map[string]any{"accept": false}
		}
		return standard(testPassword)(p)
	})
	if code(err) != "HOST_KEY_REJECTED" {
		t.Fatalf("got %v", err)
	}
	if raw, _ := os.ReadFile(filepath.Join(e.data, "known_hosts")); len(raw) != 0 {
		t.Errorf("a rejected key was saved: %q", raw)
	}
	if got := e.fake.Attempts(); len(got) != 0 {
		t.Errorf("signed in after a rejected key: %v", got)
	}
}

func otherKeyLine(t *testing.T, e *env) string {
	t.Helper()
	k, err := fakessh.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := knownhosts.Normalize(net.JoinHostPort("127.0.0.1", strconv.Itoa(e.fake.Port())))
	return knownhosts.Line([]string{addr}, k.PublicKey()) + "\n"
}

func TestChangedHostKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		file func(e *env) string
	}{
		{"own file", func(e *env) string { return filepath.Join(e.data, "known_hosts") }},
		{"user known_hosts", func(e *env) string { return filepath.Join(e.home, ".ssh", "known_hosts") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, envOpts{})
			h := e.saveHost("password", "")
			path := tc.file(e)
			if err := os.WriteFile(path, []byte(otherKeyLine(t, e)), 0o600); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(path)
			_, err := e.connect("c1", h, standard(testPassword))
			if code(err) != "HOST_KEY_CHANGED" {
				t.Fatalf("got %v", err)
			}
			msg := err.Error()
			if !strings.Contains(msg, "SHA256:") {
				t.Errorf("message names no fingerprint: %s", msg)
			}
			if tc.name == "user known_hosts" && !strings.Contains(msg, "ssh-keygen -R") {
				t.Errorf("message does not say how to remove the old key: %s", msg)
			}
			if after, _ := os.ReadFile(path); string(after) != string(before) {
				t.Error("known_hosts was changed")
			}
			select {
			case p := <-e.pub.prompts:
				t.Errorf("a changed key must not prompt: %+v", p)
			default:
			}
		})
	}
}

func TestForgetHostKeyOnlyOwnFile(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	e.mustConnect("c1", h)
	if err := e.m.Disconnect("c1"); err != nil {
		t.Fatal(err)
	}
	if err := e.m.ForgetHostKey("127.0.0.1", e.fake.Port()); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(e.data, "known_hosts"))
	if strings.Contains(string(raw), "127.0.0.1") {
		t.Errorf("host still in file: %q", raw)
	}
	asked := false
	_, err := e.connect("c2", h, func(p *prompt) map[string]any {
		asked = asked || p.Kind == "hostkey"
		return standard(testPassword)(p)
	})
	if err != nil || !asked {
		t.Fatalf("err=%v asked=%v", err, asked)
	}
}
