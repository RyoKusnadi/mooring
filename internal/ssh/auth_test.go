package ssh

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/fakessh"
	gossh "golang.org/x/crypto/ssh"
)

func TestPasswordAuth(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	c, err := e.connect("c1", h, standard(testPassword))
	if err != nil || c.State != "connected" || c.ID != "c1" || c.HostID != h.ID {
		t.Fatalf("connect: %+v %v", c, err)
	}
	if e.pub.count("ssh.connected") != 1 {
		t.Error("no ssh.connected event")
	}
	if snap := e.m.Snapshot()["sshConnections"].([]Connection); len(snap) != 1 {
		t.Errorf("snapshot: %v", snap)
	}
	if _, err := e.connect("c1", h, standard(testPassword)); code(err) != "ALREADY_EXISTS" {
		t.Errorf("same id twice: %v", err)
	}
	if _, err := e.m.Connect(context.Background(), "bad id!", h.ID); code(err) != "BAD_REQUEST" {
		t.Errorf("bad id: %v", err)
	}
}

func TestWrongPasswordIsAuthFailed(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	_, err := e.connect("c1", h, standard("not-the-password"))
	if code(err) != "AUTH_FAILED" {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("message does not list the methods: %v", err)
	}
	if n := len(e.m.Snapshot()["sshConnections"].([]Connection)); n != 0 {
		t.Errorf("a failed connection is listed: %d", n)
	}
}

func TestKeyAuth(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "id_test")
	signer, err := fakessh.NewKeyFile(keyFile, "")
	if err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, envOpts{fake: fakessh.Options{AuthorizedKeys: []gossh.PublicKey{signer.PublicKey()}}})
	dst := filepath.Join(e.home, ".ssh", "id_test")
	raw, _ := os.ReadFile(keyFile)
	if err := os.WriteFile(dst, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	h := e.saveHost("key", dst)
	if _, err := e.connect("c1", h, standard("")); err != nil {
		t.Fatalf("key sign-in: %v", err)
	}
	if _, err := e.m.SaveHost(Host{Name: "x", Address: "127.0.0.1", User: "u", Auth: "key", KeyPath: keyFile}); code(err) != "OUTSIDE_ROOT" {
		t.Errorf("a key outside the home folder: %v", err)
	}
}

func TestKeyWithPassphraseAsks(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "id_pass")
	signer, err := fakessh.NewKeyFile(keyFile, "open sesame")
	if err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, envOpts{fake: fakessh.Options{AuthorizedKeys: []gossh.PublicKey{signer.PublicKey()}}})
	dst := filepath.Join(e.home, ".ssh", "id_ed25519")
	raw, _ := os.ReadFile(keyFile)
	if err := os.WriteFile(dst, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	h := e.saveHost("auto", "")
	asked := ""
	_, err = e.connect("c1", h, func(p *prompt) map[string]any {
		if p.Kind == "passphrase" {
			asked = p.Text
			return map[string]any{"value": "open sesame"}
		}
		return standard(testPassword)(p)
	})
	if err != nil || !strings.Contains(asked, "id_ed25519") {
		t.Fatalf("err=%v asked=%q", err, asked)
	}
}

func TestKeyboardInteractiveAuth(t *testing.T) {
	e := newEnv(t, envOpts{fake: fakessh.Options{Password: "-", Questions: []fakessh.Question{{Text: "Code: ", Echo: true, Answer: "424242"}}}})
	h := e.saveHost("auto", "")
	var q *prompt
	_, err := e.connect("c1", h, func(p *prompt) map[string]any {
		if p.Kind == "question" {
			q = p
			return map[string]any{"value": "424242"}
		}
		return standard("-")(p)
	})
	if err != nil {
		t.Fatal(err)
	}
	if q == nil || q.Text != "Code: " || !q.Echo || q.Host != "u@127.0.0.1:"+itoa(e.fake.Port()) {
		t.Errorf("question prompt: %+v", q)
	}
}

func TestPromptCancelAndTimeout(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	_, err := e.connect("c1", h, func(p *prompt) map[string]any {
		if p.Kind == "password" {
			return map[string]any{"cancel": true}
		}
		return standard("")(p)
	})
	if code(err) != "CANCELLED" {
		t.Fatalf("cancel: %v", err)
	}
	if e.pub.count("ssh.prompt.closed") == 0 {
		t.Error("no ssh.prompt.closed event")
	}

	e.m.PromptTimeout = 150 * time.Millisecond
	_, err = e.connect("c2", h, func(p *prompt) map[string]any {
		if p.Kind == "password" {
			return nil // never answered
		}
		return standard("")(p)
	})
	if code(err) != "CANCELLED" {
		t.Fatalf("timeout: %v", err)
	}
	if len(e.m.Snapshot()["sshPrompts"].([]*prompt)) != 0 {
		t.Error("a closed prompt is still in the snapshot")
	}
}

func TestFirstAnswerWins(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	done := make(chan error, 1)
	go func() {
		_, err := e.m.Connect(context.Background(), "c1", h.ID)
		done <- err
	}()
	var p *prompt
	for p == nil || p.Kind != "password" {
		p = <-e.pub.prompts
		if p.Kind == "hostkey" {
			_ = e.m.Answer(mustJSON(map[string]any{"promptId": p.ID, "accept": true}))
		}
	}
	if err := e.m.Answer(mustJSON(map[string]any{"promptId": p.ID, "value": testPassword})); err != nil {
		t.Fatal(err)
	}
	if err := e.m.Answer(mustJSON(map[string]any{"promptId": p.ID, "value": "late"})); code(err) != "NOT_FOUND" {
		t.Errorf("second answer: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPromptRetryAfterRefusal(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	var retries []bool
	_, _ = e.connect("c1", h, func(p *prompt) map[string]any {
		if p.Kind == "password" {
			retries = append(retries, p.Retry)
		}
		return standard("wrong")(p)
	})
	if len(retries) < 2 || retries[0] || !retries[1] {
		t.Errorf("retry flags: %v", retries)
	}
}
