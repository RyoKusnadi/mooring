package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/fakessh"
)

func TestSilentServerTimesOutButPromptsDoNotCount(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.m.DialTimeout = 400 * time.Millisecond
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer func() { _ = c.Close() }()
		}
	}()
	h, _ := e.m.SaveHost(Host{Name: "mute", Address: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, User: "u", Auth: "password"})
	start := time.Now()
	if _, err := e.connect("c1", h, standard("")); code(err) != "HOST_UNREACHABLE" {
		t.Fatalf("silent server: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("the handshake had no deadline")
	}
	// Time spent on a prompt is the user's, not the server's.
	h2 := e.saveHost("password", "")
	slow := func(p *prompt) map[string]any {
		time.Sleep(600 * time.Millisecond)
		return standard(testPassword)(p)
	}
	if _, err := e.connect("c2", h2, slow); err != nil {
		t.Fatalf("slow answers: %v", err)
	}
}

func TestRefusedPasswordIsNotRemembered(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	n := 0
	_, err := e.connect("c1", h, func(p *prompt) map[string]any {
		if p.Kind != "password" {
			return standard("")(p)
		}
		n++
		if n == 1 {
			return map[string]any{"value": "wrong-one", "remember": true}
		}
		return map[string]any{"value": testPassword}
	})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := e.store.Get(h.ID); err == nil {
		t.Errorf("a password was remembered: %q", v)
	}
}

func TestHostKeyAnswerWithoutAcceptKeepsPromptOpen(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	done := make(chan error, 1)
	go func() { _, err := e.m.Connect(t.Context(), "c1", h.ID); done <- err }()
	p := <-e.pub.prompts
	if err := e.m.Answer(mustJSON(map[string]any{"promptId": p.ID})); code(err) != "BAD_REQUEST" {
		t.Fatalf("answer without accept: %v", err)
	}
	if got := e.m.Snapshot()["sshPrompts"].([]*prompt); len(got) != 1 {
		t.Fatalf("prompts: %d", len(got))
	}
	_ = e.m.Answer(mustJSON(map[string]any{"promptId": p.ID, "cancel": true}))
	if err := <-done; code(err) != "CANCELLED" {
		t.Errorf("got %v", err)
	}
	if got := e.m.Snapshot()["sshPrompts"].([]*prompt); len(got) != 0 {
		t.Errorf("a finished prompt stays in the snapshot: %d", len(got))
	}
}

func TestForwardLimitHoldsUnderConcurrency(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.mustConnect("c1", e.saveHost("password", ""))
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.m.Forward("c1", "127.0.0.1", 0, "127.0.0.1", 9); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != MaxForwards {
		t.Errorf("%d forwards opened, want %d", ok, MaxForwards)
	}
}

func TestKnownHostsSaveAndForgetTogether(t *testing.T) {
	e := newEnv(t, envOpts{})
	k, _ := fakessh.GenerateKey()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		addr := fmt.Sprintf("127.0.0.1:%d", 3000+i)
		go func() {
			defer wg.Done()
			if err := e.m.keys.save(addr, k.PublicKey()); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := e.m.keys.forget("127.0.0.1:9999"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	raw, _ := os.ReadFile(filepath.Join(e.data, "known_hosts"))
	if n := strings.Count(string(raw), "\n"); n != 20 {
		t.Errorf("%d lines kept, want 20", n)
	}
	if left, _ := filepath.Glob(filepath.Join(e.data, "*.tmp")); len(left) != 0 {
		t.Errorf("temporary files: %v", left)
	}
}

func TestSSHConfigMutualIncludesReadOnce(t *testing.T) {
	e := newEnv(t, envOpts{})
	writeCfg(t, e, "config", "Include a.conf\nInclude b.conf\nInclude a.conf\nHost top\n")
	writeCfg(t, e, "a.conf", "Include b.conf\nInclude b.conf\nHost a\n")
	writeCfg(t, e, "b.conf", "Include a.conf\nInclude a.conf\nHost b\n")
	done := make(chan map[string]ConfigHost, 1)
	go func() { done <- byName(e.m.ReadConfig()) }()
	select {
	case got := <-done:
		if len(got) != 3 {
			t.Errorf("hosts: %v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("includes were read over and over")
	}
}

func TestHostSavedKeySource(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	if h.SavedKey != "" {
		t.Fatalf("new host: %q", h.SavedKey)
	}
	line := otherKeyLine(t, e)
	own := filepath.Join(e.data, "known_hosts")
	user := filepath.Join(e.home, ".ssh", "known_hosts")
	check := func(want string) {
		t.Helper()
		hosts, err := e.m.Hosts()
		if err != nil || hosts[0].SavedKey != want {
			t.Errorf("savedKey %q, want %q (%v)", hosts[0].SavedKey, want, err)
		}
	}
	check("")
	_ = os.WriteFile(user, []byte(line), 0o600)
	check("user")
	_ = os.WriteFile(own, []byte(line), 0o600)
	check("mooring")
	_ = os.Remove(own)
	if err := e.m.ForgetHostKey("127.0.0.1", e.fake.Port()); err != nil {
		t.Fatal(err)
	}
	check("user")
	again, err := e.m.SaveHost(h)
	if err != nil || again.SavedKey != "user" {
		t.Errorf("hostSave answer: %+v %v", again, err)
	}
}
