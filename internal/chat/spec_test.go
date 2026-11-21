package chat

import (
	"strings"
	"testing"
)

func TestDefaultCatalogEntries(t *testing.T) {
	var ids []string
	for _, d := range DefaultCatalog() {
		ids = append(ids, d.ID)
		if d.ID == "claude" && (d.Args[1] != ClaudeAdapter || !strings.Contains(ClaudeAdapter, "@0.108.1") || !d.Tested) {
			t.Fatalf("claude entry: %+v", d)
		}
	}
	if strings.Join(ids, ",") != "claude,gemini,codex,grok,cursor,antigravity" {
		t.Fatalf("catalog: %v", ids)
	}
	missing := AgentDef{ID: "x", Name: "X", Needs: []string{"definitely-not-installed-xyz"}}.Info()
	if missing.Installed || missing.Missing != "definitely-not-installed-xyz" {
		t.Fatalf("missing: %+v", missing)
	}
	if !(AgentDef{ID: "y", Needs: []string{"sh"}}).Info().Installed {
		t.Fatal("sh should be found")
	}
}

func TestSpecDefaultsAndFolderMustStayInProject(t *testing.T) {
	r := newRig(t)
	var s Spec
	dir, err := s.normalize(r.m.catalog, "fake", r.dir)
	if err != nil || dir != r.dir || s.Agent.ID != "fake" || s.ToolServers == nil || s.Filters == nil || s.Account.ID != "" {
		t.Fatalf("defaults: %+v %v %v", s, dir, err)
	}
	out := Spec{Folder: FolderSlot{Path: "/etc"}}
	if _, err := out.normalize(r.m.catalog, "fake", r.dir); err == nil || !has(err.Error(), "OUTSIDE_ROOT") {
		t.Fatalf("outside folder: %v", err)
	}
	bad := Spec{Agent: AgentSlot{ID: "nope"}}
	if _, err := bad.normalize(r.m.catalog, "fake", r.dir); err == nil || !has(err.Error(), "NOT_FOUND") {
		t.Fatalf("unknown agent: %v", err)
	}
}

func TestToolServerSecretsAreNeverSaved(t *testing.T) {
	r := newRig(t)
	r.tools = stubTools{
		"a1": {ID: "a1", Name: "echo", Type: "stdio", Command: "echo-mcp", Env: map[string]string{"API_KEY": "s3cret-value"}},
		"b2": {ID: "b2", Name: "web", Type: "http", URL: "http://localhost:1/mcp", Headers: map[string]string{"Authorization": "Bearer s3cret-value"}},
	}
	r.boot()
	spec := map[string]any{"toolServers": []map[string]any{{"id": "a1"}, {"id": "b2"}},
		"account": map[string]string{"configDir": "/tmp/work-login"}}
	if _, err := r.run("chat.start", map[string]any{"id": "c1", "projectId": "p1", "spec": spec}); err != nil {
		t.Fatal(err)
	}
	c := r.m.get("c1")
	r.waitState(c, StateIdle)
	r.prompt(c, "hello")
	r.waitState(c, StateIdle)
	r.m.Shutdown()
	for _, q := range []string{`SELECT spec FROM chats`, `SELECT group_concat(body) FROM chat_messages`} {
		var body string
		if err := r.db.QueryRow(q).Scan(&body); err != nil {
			t.Fatal(err)
		}
		if has(body, "s3cret-value") {
			t.Fatalf("secret saved by %s", q)
		}
	}
	if _, err := r.run("chat.start", map[string]any{"id": "c2", "projectId": "p1", "spec": map[string]any{
		"toolServers": []map[string]any{{"name": "x", "command": "y"}}}}); err == nil || !has(err.Error(), "BAD_REQUEST") {
		t.Fatalf("inline form accepted: %v", err)
	}
	if _, err := r.run("chat.start", map[string]any{"id": "c3", "projectId": "p1", "spec": map[string]any{
		"toolServers": []map[string]any{{"id": "zz"}}}}); err == nil || !has(err.Error(), "NOT_FOUND") {
		t.Fatalf("unknown id accepted: %v", err)
	}
	info := c.Info()
	if len(info.Spec.ToolServers) != 2 || info.Spec.ToolServers[0].ID != "a1" || info.Spec.ToolServers[0].Env != nil {
		t.Fatalf("spec in info: %+v", info.Spec)
	}
}

func TestStartRefusesBadInput(t *testing.T) {
	r := newRig(t)
	cases := map[string]map[string]any{
		"bad id":        {"id": "bad id!", "projectId": "p1"},
		"unknown agent": {"id": "c2", "projectId": "p1", "agent": "nope"},
	}
	for name, a := range cases {
		if _, err := r.run("chat.start", a); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	r.start("c1")
	if _, err := r.run("chat.start", map[string]any{"id": "c1", "projectId": "p1"}); err == nil || !has(err.Error(), "ALREADY_EXISTS") {
		t.Fatalf("duplicate id: %v", err)
	}
	// Agent whose program is missing: a clear code, nothing created.
	r.m.catalog["gone"] = AgentDef{ID: "gone", Name: "Gone", Command: "x", Needs: []string{"definitely-not-installed-xyz"}}
	if _, err := r.run("chat.start", map[string]any{"id": "c3", "projectId": "p1", "agent": "gone"}); err == nil || !has(err.Error(), "AGENT_NOT_INSTALLED") || r.m.get("c3") != nil {
		t.Fatalf("missing program: %v", err)
	}
}
