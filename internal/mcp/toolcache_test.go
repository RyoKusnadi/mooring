package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func starts(t *testing.T, file string) int {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return len(b)
}

func TestToolListCache(t *testing.T) {
	r := newRig(t)
	file := filepath.Join(t.TempDir(), "starts")
	clock := time.Now()
	r.svc.Now = func() time.Time { return clock }
	ts := r.save(r.stdioArgs("cached", map[string]string{"FAKE_MCP_STARTFILE": file, "FAKE_MCP_TTL": "60000"}))
	ctx := context.Background()

	first, err := r.svc.Check(ctx, ts.ID)
	if err != nil || len(first.Tools) != 2 || starts(t, file) != 1 {
		t.Fatalf("first check: %v %+v starts=%d", err, first, starts(t, file))
	}
	clock = clock.Add(30 * time.Second)
	again, err := r.svc.Check(ctx, ts.ID)
	if err != nil || len(again.Tools) != 2 || starts(t, file) != 1 {
		t.Fatalf("second check inside the ttl: %v starts=%d", err, starts(t, file))
	}
	if again.CheckedAt != first.CheckedAt {
		t.Fatalf("a reused list keeps its listing time: %d then %d", first.CheckedAt, again.CheckedAt)
	}
	if first.Cached || !again.Cached {
		t.Fatalf("cached flags: first %v, reused %v", first.Cached, again.Cached)
	}
	if fresh, err := r.svc.CheckFresh(ctx, ts.ID); err != nil || starts(t, file) != 2 || fresh.Cached {
		t.Fatalf("check again: %v starts=%d cached=%v", err, starts(t, file), fresh.Cached)
	}
	clock = clock.Add(2 * time.Minute)
	if _, err := r.svc.Check(ctx, ts.ID); err != nil || starts(t, file) != 3 {
		t.Fatalf("after the ttl: %v starts=%d", err, starts(t, file))
	}
}

func TestToolListCacheDefaultsAndLimits(t *testing.T) {
	r := newRig(t)
	file := filepath.Join(t.TempDir(), "starts")
	clock := time.Now()
	r.svc.Now = func() time.Time { return clock }
	ts := r.save(r.stdioArgs("plain", map[string]string{"FAKE_MCP_STARTFILE": file}))
	ctx := context.Background()
	if _, err := r.svc.Check(ctx, ts.ID); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(9 * time.Minute)
	if _, err := r.svc.Check(ctx, ts.ID); err != nil || starts(t, file) != 1 {
		t.Fatalf("no ttlMs: the list stays fresh for 10 minutes: %v starts=%d", err, starts(t, file))
	}
	clock = clock.Add(2 * time.Minute)
	if _, err := r.svc.Check(ctx, ts.ID); err != nil || starts(t, file) != 2 {
		t.Fatalf("after 10 minutes: %v starts=%d", err, starts(t, file))
	}
}

func TestToolListCacheKeyFollowsTheConfig(t *testing.T) {
	r := newRig(t)
	file := filepath.Join(t.TempDir(), "starts")
	a := r.stdioArgs("keyed", map[string]string{"FAKE_MCP_STARTFILE": file})
	ts := r.save(a)
	ctx := context.Background()
	if _, err := r.svc.Check(ctx, ts.ID); err != nil {
		t.Fatal(err)
	}
	// A new env value is a new config: the next check starts the server again.
	a.Server.ID = ts.ID
	a.Secrets.Env["FAKE_MCP_SECRET"] = "other"
	a.Server.Env = append(a.Server.Env, Var{Name: "FAKE_MCP_SECRET"})
	r.save(a)
	if _, err := r.svc.Check(ctx, ts.ID); err != nil || starts(t, file) != 2 {
		t.Fatalf("after an edit: %v starts=%d", err, starts(t, file))
	}
	if _, err := r.svc.Check(ctx, ts.ID); err != nil || starts(t, file) != 2 {
		t.Fatalf("same config again: %v starts=%d", err, starts(t, file))
	}
}

func TestToolListCacheHonoursRemoteTTL(t *testing.T) {
	r := newRig(t)
	r.fake.TTLms = 90000
	ts := r.save(r.remoteArgs("remote ttl"))
	r.signInViaCallback(ts.ID)
	if _, err := r.svc.Check(context.Background(), ts.ID); err != nil {
		t.Fatal(err)
	}
	e := r.svc.cache.m[ts.ID]
	if e.ttl != 90*time.Second || len(e.tools) != 2 {
		t.Fatalf("cache entry: ttl=%v tools=%d", e.ttl, len(e.tools))
	}
	if _, err := r.svc.SignOut(ts.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.svc.cache.m[ts.ID]; ok {
		t.Fatal("sign-out kept the tool list")
	}
}

func TestToolListWithTTLZeroIsNotKept(t *testing.T) {
	var c toolCache
	now := time.Now()
	c.put("a", "k", []Tool{{Name: "x"}}, now, 0)
	if _, _, ok := c.get("a", "k", now); ok {
		t.Fatal("a list with ttl 0 was kept")
	}
	c.put("a", "k", []Tool{{Name: "x"}}, now, time.Minute)
	if _, _, ok := c.get("a", "other", now); ok {
		t.Fatal("served for another config")
	}
}

func TestTTLSpyReadsLinesAndEvents(t *testing.T) {
	for name, body := range map[string]string{
		"lines":  `{"jsonrpc":"2.0","id":2,"result":{"tools":[],"ttlMs":5000}}` + "\n",
		"events": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"tools\":[],\"ttlMs\":5000}}\n\n",
		"no eol": `{"jsonrpc":"2.0","id":2,"result":{"tools":[],"ttlMs":5000}}`,
	} {
		spy := &ttlSpy{}
		buf := make([]byte, 7)
		rd := spy.wrap(strings.NewReader(body))
		for {
			if _, err := rd.Read(buf); err != nil {
				break
			}
		}
		if got := spy.ttl(); got != 5*time.Second {
			t.Errorf("%s: ttl %v", name, got)
		}
	}
	spy := &ttlSpy{}
	_, _ = spy.wrap(strings.NewReader(`{"result":{"tools":[]}}` + "\n")).Read(make([]byte, 100))
	if spy.ttl() != defaultToolTTL {
		t.Errorf("no ttlMs: %v", spy.ttl())
	}
}
