package ledger

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/store"
)

func commandRows(t *testing.T, st *store.Store) int {
	t.Helper()
	var n int
	if err := st.DB.QueryRow(`SELECT COUNT(*) FROM commands`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Pruning is a write; doing it after every command kept the database busy through a burst and made
// writes from elsewhere fail with SQLITE_BUSY. It runs once every pruneEvery commands.
func TestPruneRunsOnceEveryPruneEveryCommands(t *testing.T) {
	st := newStore(t)
	l := newLedger(t, st, &fakeClock{t: time.Unix(1000, 0)})
	l.maxBare = 1
	l.maxOutcomes = 0
	run := func(context.Context) (any, error) { return nil, nil }
	for i := 0; i < pruneEvery-1; i++ {
		wait(t, l.Submit(fmt.Sprintf("a%d", i), "x.do", nil, false, run))
	}
	l.Wait()
	if n := commandRows(t, st); n != pruneEvery-1 {
		t.Fatalf("pruned too early: %d rows", n)
	}
	wait(t, l.Submit("last", "x.do", nil, false, run))
	l.Wait()
	if n := commandRows(t, st); n >= pruneEvery {
		t.Fatalf("not pruned after %d commands: %d rows", pruneEvery, n)
	}
}

// A burst of commands must leave room for another writer to get in.
func TestBurstOfCommandsDoesNotStarveOtherWrites(t *testing.T) {
	st := newStore(t)
	l := newLedger(t, st, &fakeClock{t: time.Unix(1000, 0)})
	if _, err := st.DB.Exec(`CREATE TABLE neighbour (n INTEGER)`); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				done <- nil
				return
			default:
			}
			if _, err := st.DB.Exec(`INSERT INTO neighbour VALUES (?)`, i); err != nil {
				done <- err
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	run := func(context.Context) (any, error) { return nil, nil }
	for i := 0; i < 300; i++ {
		wait(t, l.Submit(fmt.Sprintf("b%d", i), "x.do", nil, false, run))
	}
	close(stop)
	if err := <-done; err != nil {
		t.Fatalf("a write beside the burst failed: %v", err)
	}
}

func TestNewPrunesOldRecords(t *testing.T) {
	st := newStore(t)
	clk := &fakeClock{t: time.Unix(1000, 0)}
	if _, err := st.DB.Exec(`INSERT INTO commands (id, name, args_hash, status, done_at) VALUES ('old', 'x', 'h', 'unknown', ?)`,
		clk.now().Add(-2*BareTTL).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	newLedger(t, st, clk)
	if n := commandRows(t, st); n != 0 {
		t.Fatalf("old record kept after start: %d rows", n)
	}
}

func TestOnlyOnePruneRunsAtATime(t *testing.T) {
	st := newStore(t)
	l := newLedger(t, st, &fakeClock{t: time.Unix(1000, 0)})
	l.pruning.Store(true)
	l.sincePrune.Store(pruneEvery - 1)
	l.pruneIfDue()
	if got := l.sincePrune.Load(); got != pruneEvery {
		t.Fatalf("a prune started while another ran: counter %d", got)
	}
	l.pruning.Store(false)
	l.pruneIfDue()
	if l.sincePrune.Load() != 0 || l.pruning.Load() {
		t.Fatalf("prune did not run and release: %d %v", l.sincePrune.Load(), l.pruning.Load())
	}
}
