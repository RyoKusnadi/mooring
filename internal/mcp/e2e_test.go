package mcp

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/chat"
	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/fakemcp"
)

func (r *rig) chatManager(t *testing.T) (*chat.Manager, func(id, spec string) string) {
	t.Helper()
	dir := t.TempDir()
	if _, err := r.st.DB.Exec(`INSERT INTO projects (id, name, path, position, created_at) VALUES ('p1', 'p', ?, 0, 0)`, dir); err != nil {
		t.Fatal(err)
	}
	env := []string{"MOORING_FAKE_AGENT=1", "FAKE_STATE=" + t.TempDir()}
	m, err := chat.New(chat.Config{
		DB: r.st.DB, Pub: r.pub, Tools: r.svc, DefaultAgent: "fake-a",
		Catalog: []chat.AgentDef{
			{ID: "fake-a", Name: "Fake A", Command: os.Args[0], Env: env},
			{ID: "fake-b", Name: "Fake B", Command: os.Args[0], Env: env},
		},
		Dir: func(string) (string, error) { return dir, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Shutdown)
	// ask starts a chat and sends TOOLS; it returns everything the chat said.
	ask := func(id, spec string) string {
		t.Helper()
		run := func(name, args string) {
			if _, err := m.Run(events.Command{ID: "c", Name: name, Args: []byte(args)}); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		run("chat.start", `{"id":"`+id+`","projectId":"p1",`+spec+`}`)
		deadline := time.Now().Add(10 * time.Second)
		for {
			_, err := m.Run(events.Command{ID: "c", Name: "chat.prompt", Args: []byte(`{"id":"` + id + `","text":"TOOLS"}`)})
			if err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("prompt: %v", err)
			}
			time.Sleep(20 * time.Millisecond)
		}
		for time.Now().Before(deadline) {
			var body string
			_ = r.st.DB.QueryRow(`SELECT coalesce(group_concat(body, ' '), '') FROM chat_messages WHERE chat_id = ?`, id).Scan(&body)
			if strings.Contains(body, "tools:") {
				return body
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("no answer from the agent")
		return ""
	}
	return m, ask
}

func TestToolServersEndToEnd(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	local := r.save(r.stdioArgs("local echo", nil))
	remote := r.save(r.remoteArgs("remote"))
	for _, ts := range []ToolServer{local, remote} {
		if ts.Allowed != true || ts.Enabled {
			t.Fatalf("saved server: %+v", ts)
		}
	}
	got, err := r.svc.Check(ctx, local.ID)
	if err != nil || len(got.Tools) != 2 {
		t.Fatalf("stdio check: %v %+v", err, got)
	}
	if _, err := r.svc.Check(ctx, remote.ID); err == nil || !strings.Contains(err.Error(), "SIGN_IN_REQUIRED") {
		t.Fatalf("remote check before sign-in: %v", err)
	}
	if _, err := r.svc.Enable(local.ID, true); err != nil {
		t.Fatal(err)
	}

	_, ask := r.chatManager(t)
	for i, spec := range []string{`"spec":{"agent":{"id":"fake-a"},"toolServers":[{"id":"` + local.ID + `"}]}`,
		`"spec":{"agent":{"id":"fake-b"},"toolServers":[{"id":"` + local.ID + `"}]}`} {
		if body := ask("chat"+string(rune('a'+i)), spec); !strings.Contains(body, "local-echo(stdio,auth=no)") {
			t.Fatalf("agent %d did not get the stdio server: %s", i, body)
		}
	}

	// Sign in through the callback route.
	r.signInViaCallback(remote.ID)
	if _, err := r.svc.Check(ctx, remote.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Enable(remote.ID, true); err != nil {
		t.Fatal(err)
	}
	if body := ask("chatr", `"spec":{"toolServers":[{"id":"`+remote.ID+`"}]}`); !strings.Contains(body, "remote(http,auth=yes)") {
		t.Fatalf("remote server without a token: %s", body)
	}

	// A second sign-in, finished by pasting the address.
	if _, err := r.svc.SignOut(remote.ID); err != nil {
		t.Fatal(err)
	}
	authURL, _, err := r.svc.SignIn(ctx, remote.ID)
	if err != nil {
		t.Fatal(err)
	}
	back, err := fakemcp.Approve(authURL)
	if err != nil {
		t.Fatal(err)
	}
	if ts, err := r.svc.SignInFinish(ctx, remote.ID, back); err != nil || ts.SignIn != "signedIn" {
		t.Fatalf("signInFinish: %v %+v", err, ts)
	}
	if st := r.fake.Stats(); st.LastMethod != "S256" || st.PKCEFailures != 0 {
		t.Fatalf("pkce: %+v", st)
	}
	if ck, err := r.svc.Check(ctx, remote.ID); err != nil || len(ck.Tools) != 2 {
		t.Fatalf("remote check after sign-in: %v %+v", err, ck)
	}

	// Private addresses are refused unless the server is allowed to use them.
	for _, u := range []string{r.fts.URL + "/mcp", "http://10.1.2.3/mcp", "https://192.168.0.5/mcp"} {
		_, err := r.svc.Save(ctx, SaveArgs{Server: ToolServer{Name: "p", Kind: "http", URL: u}, Allow: true})
		if err == nil || !strings.Contains(err.Error(), "PRIVATE_ADDRESS") {
			t.Fatalf("%s: %v", u, err)
		}
	}
	for _, leak := range []string{"at-", "rt-"} {
		if strings.Contains(r.pub.all(), leak) {
			t.Fatalf("token in events: %s", r.pub.all())
		}
	}
}

func (r *rig) signInViaCallback(id string) {
	r.t.Helper()
	authURL, redirect, err := r.svc.SignIn(context.Background(), id)
	if err != nil {
		r.t.Fatal(err)
	}
	if !strings.HasPrefix(redirect, r.web.URL+callbackPath) {
		r.t.Fatalf("redirect %s", redirect)
	}
	back, err := fakemcp.Approve(authURL)
	if err != nil {
		r.t.Fatal(err)
	}
	resp, err := http.Get(back)
	if err != nil {
		r.t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		r.t.Fatalf("callback answered %d", resp.StatusCode)
	}
	// The same state does not work twice.
	resp, err = http.Get(back)
	if err != nil {
		r.t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 400 {
		r.t.Fatalf("second callback answered %d", resp.StatusCode)
	}
}
