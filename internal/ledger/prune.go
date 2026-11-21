package ledger

import "fmt"

// hashes keeps the args hash of commands running in this process, so a
// repeat id can be checked without a database read. Callers hold l.mu.
func (l *Ledger) hashes() map[string]string {
	if l.hashMap == nil {
		l.hashMap = map[string]string{}
	}
	return l.hashMap
}

func (l *Ledger) activeHash(id string) string { return l.hashes()[id] }

// Prune reduces old outcomes to bare ids and drops old bare ids, within the
// limits in the contract.
func (l *Ledger) Prune() error {
	now := l.now().UnixMilli()
	stmts := []struct {
		sql  string
		args []any
	}{
		{`UPDATE commands SET status = 'unknown', outcome = NULL WHERE status = 'done' AND done_at < ?`,
			[]any{now - OutcomeTTL.Milliseconds()}},
		{`UPDATE commands SET status = 'unknown', outcome = NULL WHERE id IN (
			SELECT id FROM commands WHERE status = 'done' ORDER BY done_at DESC, rowid DESC LIMIT -1 OFFSET ?)`,
			[]any{l.maxOutcomes}},
		{`DELETE FROM commands WHERE status = 'unknown' AND done_at < ?`,
			[]any{now - BareTTL.Milliseconds()}},
		{`DELETE FROM commands WHERE id IN (
			SELECT id FROM commands WHERE status = 'unknown' ORDER BY done_at DESC, rowid DESC LIMIT -1 OFFSET ?)`,
			[]any{l.maxBare}},
	}
	// One transaction: one commit and one sync, not four.
	tx, err := l.db.Begin()
	if err != nil {
		return fmt.Errorf("prune commands: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, s := range stmts {
		if _, err := tx.Exec(s.sql, s.args...); err != nil {
			return fmt.Errorf("prune commands: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("prune commands: %w", err)
	}
	return nil
}
