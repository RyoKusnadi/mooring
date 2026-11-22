// Package ledger makes every browser command run once. A command is recorded
// as running in SQLite before it starts, so the rule also holds across restarts.
package ledger

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"sync/atomic"
	"time"
)

// Limits.
const (
	PendingAfter    = 25 * time.Second
	OutcomeTTL      = 10 * time.Minute
	MaxOutcomes     = 4096
	BareTTL         = 60 * time.Minute
	MaxBare         = 65536
	MaxMessageChars = 512
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// Error is a failure with a code from the contract and a message fit to show the user.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// pruneEvery is how many finished commands pass between prunes.
const pruneEvery = 50

// Outcome is the result of a command.
type Outcome struct {
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value,omitempty"`
	Error *Error          `json:"error,omitempty"`
}

// RunFunc does the work of a command. Its error should be an *Error; anything
// else is logged and reported as INTERNAL.
type RunFunc func(ctx context.Context) (any, error)

// Ledger records commands and decides whether each one may run.
type Ledger struct {
	db  *sql.DB
	now func() time.Time
	log *slog.Logger
	ctx context.Context

	mu                   sync.Mutex
	active               map[string]*Call
	hashMap              map[string]string
	maxOutcomes, maxBare int
	wg                   sync.WaitGroup
	sincePrune           atomic.Int32
	pruning              atomic.Bool
}

// New opens the ledger. Commands recorded as running belong to a process that
// is gone; they become unknown and are never run again. ctx is given to every
// command; when it ends, unfinished commands stay unrecorded (unknown later).
func New(ctx context.Context, db *sql.DB, now func() time.Time, log *slog.Logger) (*Ledger, error) {
	if now == nil {
		now = time.Now
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	l := &Ledger{db: db, now: now, log: log, ctx: ctx, active: map[string]*Call{}, maxOutcomes: MaxOutcomes, maxBare: MaxBare}
	res, err := db.Exec(`UPDATE commands SET status = 'unknown', outcome = NULL, done_at = ? WHERE status = 'running'`, now().UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("mark interrupted commands: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Warn("ledger: commands were running at start; outcome unknown", "count", n)
	}
	// A restart prunes too: a server that is stopped often may never reach pruneEvery commands.
	if err := l.Prune(); err != nil {
		return nil, fmt.Errorf("prune commands at start: %w", err)
	}
	return l, nil
}

// Wait blocks until commands started by Submit have finished.
func (l *Ledger) Wait() { l.wg.Wait() }

// Call is one command, running or finished.
type Call struct {
	done    chan struct{}
	outcome Outcome
}

// Done is closed when the outcome is ready.
func (c *Call) Done() <-chan struct{} { return c.done }

// Outcome is valid after Done is closed.
func (c *Call) Outcome() Outcome { return c.outcome }

func finished(o Outcome) *Call {
	c := &Call{done: make(chan struct{}), outcome: o}
	close(c.done)
	return c
}

func failure(code, msg string) Outcome {
	return Outcome{Error: &Error{Code: code, Message: msg}}
}

func hashOf(name string, args json.RawMessage, secret bool) string {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte{0})
	if !secret {
		var buf bytes.Buffer
		if json.Compact(&buf, args) == nil {
			h.Write(buf.Bytes())
		} else {
			h.Write(args)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Submit records the command and starts run, or answers from the record.
// The Call it returns is already finished for a repeated id.
func (l *Ledger) Submit(id, name string, args json.RawMessage, secret bool, run RunFunc) *Call {
	if !idPattern.MatchString(id) {
		return finished(failure("BAD_REQUEST", "The command id must be 1 to 128 letters, digits, dashes or underscores."))
	}
	hash := hashOf(name, args, secret)

	l.mu.Lock()
	defer l.mu.Unlock()
	if c, ok := l.active[id]; ok {
		if hash != l.activeHash(id) {
			return finished(failure("COMMAND_CONFLICT", "This command id was already used for a different command."))
		}
		return c
	}
	var (
		oldName, oldHash, status string
		outcome                  sql.NullString
	)
	err := l.db.QueryRow(`SELECT name, args_hash, status, outcome FROM commands WHERE id = ?`, id).
		Scan(&oldName, &oldHash, &status, &outcome)
	switch {
	case err == nil:
		if oldHash != hash {
			return finished(failure("COMMAND_CONFLICT", "This command id was already used for a different command."))
		}
		if status == "done" && outcome.Valid {
			var o Outcome
			if json.Unmarshal([]byte(outcome.String), &o) == nil {
				return finished(o)
			}
		}
		return finished(failure("OUTCOME_UNKNOWN", "The server restarted or lost track of this command, so it is not known whether it finished. Check before trying again."))
	case !errors.Is(err, sql.ErrNoRows):
		l.log.Error("ledger: read command", "id", id, "err", err)
		return finished(failure("INTERNAL", "The server could not record this command."))
	}
	if _, err := l.db.Exec(`INSERT INTO commands (id, name, args_hash, status) VALUES (?, ?, ?, 'running')`, id, name, hash); err != nil {
		l.log.Error("ledger: record command", "id", id, "err", err)
		return finished(failure("INTERNAL", "The server could not record this command."))
	}
	call := &Call{done: make(chan struct{})}
	l.active[id] = call
	l.hashes()[id] = hash
	l.wg.Add(1)
	go l.execute(id, name, secret, call, run)
	return call
}

func (l *Ledger) execute(id, name string, secret bool, call *Call, run RunFunc) {
	defer l.wg.Done()
	value, err := run(l.ctx)
	out := l.toOutcome(id, name, value, err)
	if l.ctx.Err() != nil {
		// Shutting down mid-command: leave it recorded as running.
		l.mu.Lock()
		delete(l.active, id)
		delete(l.hashes(), id)
		l.mu.Unlock()
		call.outcome = failure("OUTCOME_UNKNOWN", "The server is shutting down. Check before trying again.")
		close(call.done)
		return
	}
	stored := out
	if secret {
		stored.Value = nil
	}
	body, mErr := json.Marshal(stored)
	if mErr != nil {
		body = nil
	}
	l.mu.Lock()
	_, dbErr := l.db.Exec(`UPDATE commands SET status = 'done', outcome = ?, done_at = ? WHERE id = ?`,
		string(body), l.now().UnixMilli(), id)
	delete(l.active, id)
	delete(l.hashes(), id)
	l.mu.Unlock()
	if dbErr != nil {
		l.log.Error("ledger: record outcome", "id", id, "name", name, "err", dbErr)
	}
	call.outcome = out
	close(call.done)
	// Pruning is a write too. Doing it after every command made a burst of commands hold the
	// database almost without a break, so a write from anywhere else could wait out its 5 seconds
	// and fail with SQLITE_BUSY. Once every pruneEvery commands is enough: the limits are loose.
	l.pruneIfDue()
}

func (l *Ledger) toOutcome(id, name string, value any, err error) Outcome {
	if err == nil {
		raw, mErr := json.Marshal(value)
		if mErr != nil {
			l.log.Error("ledger: encode value", "id", id, "name", name, "err", mErr)
			return failure("INTERNAL", "Something went wrong on the server.")
		}
		if value == nil {
			raw = []byte("{}")
		}
		return Outcome{OK: true, Value: raw}
	}
	var e *Error
	if errors.As(err, &e) {
		msg := e.Message
		if r := []rune(msg); len(r) > MaxMessageChars {
			msg = string(r[:MaxMessageChars-1]) + "…"
		}
		return failure(e.Code, msg)
	}
	l.log.Error("command failed", "id", id, "name", name, "err", err)
	return failure("INTERNAL", "Something went wrong on the server.")
}

// Wait waits for call to finish. If it is still running after `after`, it calls
// onPending once and keeps waiting. It returns false if ctx ends first.
func (c *Call) Wait(ctx context.Context, after time.Duration, onPending func()) (Outcome, bool) {
	t := time.NewTimer(after)
	defer t.Stop()
	select {
	case <-c.done:
		return c.outcome, true
	case <-t.C:
		onPending()
	case <-ctx.Done():
		return Outcome{}, false
	}
	select {
	case <-c.done:
		return c.outcome, true
	case <-ctx.Done():
		return Outcome{}, false
	}
}

// pruneIfDue prunes once every pruneEvery finished commands. Only one prune runs at a time: a
// command that finishes while one is running leaves the work to it.
func (l *Ledger) pruneIfDue() {
	if l.sincePrune.Add(1) < pruneEvery || !l.pruning.CompareAndSwap(false, true) {
		return
	}
	defer l.pruning.Store(false)
	l.sincePrune.Store(0)
	if err := l.Prune(); err != nil {
		l.log.Error("ledger: prune", "err", err)
	}
}
