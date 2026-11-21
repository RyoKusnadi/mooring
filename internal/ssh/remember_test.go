package ssh

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/events"
)

func TestRememberPasswordWithSecretStore(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	_, err := e.connect("c1", h, func(p *prompt) map[string]any {
		if p.Kind == "password" {
			return map[string]any{"value": testPassword, "remember": true}
		}
		return standard("")(p)
	})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := e.store.Get(h.ID); v != testPassword {
		t.Fatalf("stored %q", v)
	}
	hosts, _ := e.m.Hosts()
	if len(hosts) != 1 || !hosts[0].Remembered {
		t.Fatalf("remembered flag: %+v", hosts)
	}
	_ = e.m.Disconnect("c1")

	// A later connection uses it without asking.
	if _, err := e.connect("c2", hosts[0], func(p *prompt) map[string]any {
		if p.Kind == "password" {
			t.Error("asked for a remembered password")
		}
		return standard("")(p)
	}); err != nil {
		t.Fatal(err)
	}
	_ = e.m.Disconnect("c2")

	// A refused one is deleted and asked again.
	_ = e.store.Set(h.ID, "stale")
	asked := false
	if _, err := e.connect("c3", hosts[0], func(p *prompt) map[string]any {
		if p.Kind == "password" {
			asked = true
		}
		return standard(testPassword)(p)
	}); err != nil || !asked {
		t.Fatalf("err=%v asked=%v", err, asked)
	}
	if _, err := e.store.Get(h.ID); err == nil {
		t.Error("the refused password was kept")
	}
	_ = e.m.Disconnect("c3")

	// Removing the host forgets the password.
	_ = e.store.Set(h.ID, "x")
	if err := e.m.RemoveHost(h.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.store.Get(h.ID); err == nil {
		t.Error("the password outlived the host")
	}
}

func TestRememberIgnoredWithoutStore(t *testing.T) {
	e := newEnv(t, envOpts{noStore: true})
	if e.m.CanRemember() {
		t.Fatal("CanRemember without a store")
	}
	h := e.saveHost("password", "")
	if _, err := e.connect("c1", h, func(p *prompt) map[string]any {
		if p.Kind == "password" {
			return map[string]any{"value": testPassword, "remember": true}
		}
		return standard("")(p)
	}); err != nil {
		t.Fatal(err)
	}
	if hosts, _ := e.m.Hosts(); hosts[0].Remembered {
		t.Error("remembered without a store")
	}
}

func TestSecretsNeverInLogsOrLedger(t *testing.T) {
	e := newEnv(t, envOpts{})
	h := e.saveHost("password", "")
	cmds := ledgerCommands{e}
	done := make(chan error, 1)
	go func() {
		_, err := e.m.Connect(context.Background(), "c1", h.ID)
		done <- err
	}()
	n := 0
	for {
		select {
		case p := <-e.pub.prompts:
			args := map[string]any{"promptId": p.ID, "value": testPassword, "remember": true}
			if p.Kind == "hostkey" {
				args = map[string]any{"promptId": p.ID, "accept": true}
			}
			raw, _ := json.Marshal(args)
			n++
			call := e.led.Submit("cmd-"+itoa(n), "ssh.answer", raw, cmds.Secret("ssh.answer"), func(ctx context.Context) (any, error) {
				return cmds.Run(ctx, "ssh.answer", raw)
			})
			if _, ok := call.Wait(context.Background(), time.Second, func() {}); !ok {
				t.Fatal("answer did not finish")
			}
			continue
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		}
		break
	}
	if !cmds.Secret("ssh.answer") || cmds.Secret("ssh.connect") {
		t.Error("only ssh.answer is secret")
	}
	rows, err := e.m.DB.Query(`SELECT id || '|' || name || '|' || args_hash || '|' || status || '|' || COALESCE(outcome, '') FROM commands`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		var row string
		_ = rows.Scan(&row)
		count++
		if strings.Contains(row, testPassword) {
			t.Errorf("password in the ledger: %s", row)
		}
	}
	if count == 0 {
		t.Fatal("no ledger rows to check")
	}
	_ = e.m.Disconnect("c1")
	if strings.Contains(e.logs.String(), testPassword) {
		t.Error("password in the logs")
	}
	hosts, _ := e.m.Hosts()
	raw, _ := json.Marshal(hosts)
	if strings.Contains(string(raw), testPassword) {
		t.Error("password in the hosts")
	}
	var dump string
	_ = e.m.DB.QueryRow(`SELECT group_concat(id || name || address || user || key_path || root, ' ') FROM ssh_hosts`).Scan(&dump)
	if strings.Contains(dump, testPassword) {
		t.Error("password in ssh_hosts")
	}
}

// ledgerCommands runs ssh commands the way the server does.
type ledgerCommands struct{ e *env }

func (l ledgerCommands) Secret(name string) bool { return Secret(name) }

func (l ledgerCommands) Run(ctx context.Context, name string, args json.RawMessage) (any, error) {
	return l.e.m.Run(ctx, events.Command{Name: name, Args: args})
}

func mustJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}
