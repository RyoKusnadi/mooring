package store

import (
	"sync"
	"testing"
)

// Transactions that read and then write, run from several connections at once,
// must wait for each other instead of failing with "database is locked".
func TestConcurrentReadThenWriteTransactionsWait(t *testing.T) {
	s, err := Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if _, err := s.DB.Exec(`CREATE TABLE counter (n INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`INSERT INTO counter (n) VALUES (0)`); err != nil {
		t.Fatal(err)
	}
	const workers, rounds = 4, 50
	var wg sync.WaitGroup
	errs := make(chan error, workers*rounds)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				errs <- bump(s)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := s.DB.QueryRow(`SELECT n FROM counter`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != workers*rounds {
		t.Fatalf("counter is %d, want %d", n, workers*rounds)
	}
}

func bump(s *Store) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var n int
	if err := tx.QueryRow(`SELECT n FROM counter`).Scan(&n); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE counter SET n = ?`, n+1); err != nil {
		return err
	}
	return tx.Commit()
}
