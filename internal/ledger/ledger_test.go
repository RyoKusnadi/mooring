package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/store"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newLedger(t *testing.T, st *store.Store, clk *fakeClock) *Ledger {
	t.Helper()
	l, err := New(context.Background(), st.DB, clk.now, nil)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func wait(t *testing.T, c *Call) Outcome {
	t.Helper()
	select {
	case <-c.Done():
		return c.Outcome()
	case <-time.After(5 * time.Second):
		t.Fatal("command did not finish")
		return Outcome{}
	}
}

func counting(n *atomic.Int32, v any) RunFunc {
	return func(context.Context) (any, error) { n.Add(1); return v, nil }
}

func TestSameIdRunsOnceAndReplaysOutcome(t *testing.T) {
	l := newLedger(t, newStore(t), &fakeClock{t: time.Unix(1000, 0)})
	var n atomic.Int32
	args := json.RawMessage(`{"a": 1}`)
	first := wait(t, l.Submit("c1", "x.do", args, false, counting(&n, map[string]int{"v": 7})))
	l.Wait()
	again := wait(t, l.Submit("c1", "x.do", json.RawMessage(`{"a":1}`), false, counting(&n, nil)))
	if n.Load() != 1 || !first.OK || string(first.Value) != `{"v":7}` || string(again.Value) != string(first.Value) {
		t.Fatalf("runs=%d first=%+v again=%+v", n.Load(), first, again)
	}
}

func TestSameIdWhileRunningSharesTheCall(t *testing.T) {
	l := newLedger(t, newStore(t), &fakeClock{t: time.Unix(1000, 0)})
	release := make(chan struct{})
	var n atomic.Int32
	run := func(context.Context) (any, error) { n.Add(1); <-release; return "ok", nil }
	a := l.Submit("c1", "x.do", nil, false, run)
	b := l.Submit("c1", "x.do", nil, false, run)
	if a != b {
		t.Fatal("expected the same call")
	}
	close(release)
	if o := wait(t, b); !o.OK || n.Load() != 1 {
		t.Fatalf("outcome %+v runs %d", o, n.Load())
	}
}

func TestConflictOnDifferentNameOrArgs(t *testing.T) {
	l := newLedger(t, newStore(t), &fakeClock{t: time.Unix(1000, 0)})
	var n atomic.Int32
	wait(t, l.Submit("c1", "x.do", json.RawMessage(`{"a":1}`), false, counting(&n, nil)))
	l.Wait()
	for _, c := range []struct{ name, args string }{{"x.do", `{"a":2}`}, {"y.do", `{"a":1}`}} {
		o := wait(t, l.Submit("c1", c.name, json.RawMessage(c.args), false, counting(&n, nil)))
		if o.OK || o.Error.Code != "COMMAND_CONFLICT" {
			t.Fatalf("%v: %+v", c, o)
		}
	}
	if n.Load() != 1 {
		t.Fatalf("ran %d times", n.Load())
	}
}

func TestRunningAtStartIsOutcomeUnknown(t *testing.T) {
	st := newStore(t)
	clk := &fakeClock{t: time.Unix(1000, 0)}
	l1 := newLedger(t, st, clk)
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	l1.Submit("c1", "x.do", nil, false, func(context.Context) (any, error) { <-block; return nil, nil })

	l2 := newLedger(t, st, clk) // a restart: the first process is gone
	var n atomic.Int32
	o := wait(t, l2.Submit("c1", "x.do", nil, false, counting(&n, nil)))
	if o.OK || o.Error.Code != "OUTCOME_UNKNOWN" || n.Load() != 0 {
		t.Fatalf("outcome %+v runs %d", o, n.Load())
	}
}

func TestPendingAfterDelayThenResult(t *testing.T) {
	l := newLedger(t, newStore(t), &fakeClock{t: time.Unix(1000, 0)})
	release := make(chan struct{})
	c := l.Submit("c1", "x.do", nil, false, func(context.Context) (any, error) { <-release; return 1, nil })
	pending := make(chan struct{}, 1)
	go func() {
		<-pending
		close(release)
	}()
	o, ok := c.Wait(context.Background(), 20*time.Millisecond, func() { pending <- struct{}{} })
	if !ok || !o.OK {
		t.Fatalf("%+v %v", o, ok)
	}
	if PendingAfter != 25*time.Second {
		t.Fatal("contract says 25 s")
	}
}

func TestOutcomeReducedToBareIdThenForgotten(t *testing.T) {
	st := newStore(t)
	clk := &fakeClock{t: time.Unix(1000, 0)}
	l := newLedger(t, st, clk)
	var n atomic.Int32
	wait(t, l.Submit("c1", "x.do", nil, false, counting(&n, "v")))
	l.Wait()

	clk.advance(OutcomeTTL + time.Second)
	if err := l.Prune(); err != nil {
		t.Fatal(err)
	}
	o := wait(t, l.Submit("c1", "x.do", nil, false, counting(&n, nil)))
	if o.OK || o.Error.Code != "OUTCOME_UNKNOWN" || n.Load() != 1 {
		t.Fatalf("bare id: %+v runs %d", o, n.Load())
	}

	clk.advance(BareTTL)
	if err := l.Prune(); err != nil {
		t.Fatal(err)
	}
	o = wait(t, l.Submit("c1", "x.do", nil, false, counting(&n, "again")))
	l.Wait()
	if !o.OK || n.Load() != 2 {
		t.Fatalf("forgotten id should run again: %+v runs %d", o, n.Load())
	}
}

func TestOutcomeCountLimit(t *testing.T) {
	st := newStore(t)
	clk := &fakeClock{t: time.Unix(1000, 0)}
	l := newLedger(t, st, clk)
	now := clk.now().UnixMilli()
	tx, _ := st.DB.Begin()
	for i := 0; i < MaxOutcomes+10; i++ {
		_, err := tx.Exec(`INSERT INTO commands (id, name, args_hash, status, outcome, done_at) VALUES (?, 'x', 'h', 'done', '{"ok":true}', ?)`,
			fmt.Sprintf("id%d", i), now+int64(i))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := l.Prune(); err != nil {
		t.Fatal(err)
	}
	var kept, bare int
	_ = st.DB.QueryRow(`SELECT COUNT(*) FROM commands WHERE status='done'`).Scan(&kept)
	_ = st.DB.QueryRow(`SELECT COUNT(*) FROM commands WHERE status='unknown'`).Scan(&bare)
	if kept != MaxOutcomes || bare != 10 {
		t.Fatalf("kept %d bare %d", kept, bare)
	}
	var newest string
	_ = st.DB.QueryRow(`SELECT status FROM commands WHERE id = ?`, fmt.Sprintf("id%d", MaxOutcomes+9)).Scan(&newest)
	if newest != "done" {
		t.Fatalf("newest was reduced: %s", newest)
	}
}

func TestBareCountLimit(t *testing.T) {
	st := newStore(t)
	clk := &fakeClock{t: time.Unix(1000, 0)}
	l := newLedger(t, st, clk)
	l.maxBare = 100
	now := clk.now().UnixMilli()
	tx, _ := st.DB.Begin()
	for i := 0; i < l.maxBare+5; i++ {
		if _, err := tx.Exec(`INSERT INTO commands (id, name, args_hash, status, done_at) VALUES (?, 'x', 'h', 'unknown', ?)`,
			fmt.Sprintf("id%d", i), now+int64(i)); err != nil {
			t.Fatal(err)
		}
	}
	_ = tx.Commit()
	if err := l.Prune(); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = st.DB.QueryRow(`SELECT COUNT(*) FROM commands`).Scan(&n)
	if n != l.maxBare {
		t.Fatalf("kept %d", n)
	}
}

func TestIdAndMessageLimitsAndSecrets(t *testing.T) {
	st := newStore(t)
	l := newLedger(t, st, &fakeClock{t: time.Unix(1000, 0)})
	for _, id := range []string{"", "has space", strings.Repeat("a", 129)} {
		o := wait(t, l.Submit(id, "x", nil, false, nil))
		if o.OK || o.Error.Code != "BAD_REQUEST" {
			t.Fatalf("id %q: %+v", id, o)
		}
	}
	long := strings.Repeat("é", 600)
	o := wait(t, l.Submit("c1", "x", nil, false, func(context.Context) (any, error) {
		return nil, &Error{Code: "NOT_FOUND", Message: long}
	}))
	if o.Error.Code != "NOT_FOUND" || len([]rune(o.Error.Message)) != MaxMessageChars {
		t.Fatalf("message length %d", len([]rune(o.Error.Message)))
	}
	o = wait(t, l.Submit("c2", "x", nil, false, func(context.Context) (any, error) { return nil, errors.New("disk path /secret") }))
	if o.Error.Code != "INTERNAL" || strings.Contains(o.Error.Message, "secret") {
		t.Fatalf("%+v", o)
	}

	wait(t, l.Submit("s1", "login", json.RawMessage(`{"password":"hunter2"}`), true,
		func(context.Context) (any, error) { return map[string]string{"token": "tok-abc"}, nil }))
	l.Wait()
	var dump string
	if err := st.DB.QueryRow(`SELECT group_concat(id||name||args_hash||ifnull(outcome,''), '|') FROM commands`).Scan(&dump); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dump, "hunter2") || strings.Contains(dump, "tok-abc") {
		t.Fatalf("secret stored: %s", dump)
	}
}
