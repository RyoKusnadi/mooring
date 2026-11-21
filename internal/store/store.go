// Package store opens Mooring's SQLite database and applies its migrations.
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	// Pure Go driver: keeps Mooring a single static binary.
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Store is the open database.
type Store struct {
	DB *sql.DB
}

// Open creates dir if needed, opens <dir>/mooring.db (mode 0600, WAL, foreign
// keys on) and applies pending migrations. Before migrating a database that
// already has a schema it writes a backup (see backup.go); a database from a
// newer build is refused.
func Open(dir string, log *slog.Logger) (*Store, error) {
	return open(dir, migrationFiles, time.Now, log)
}

func open(dir string, files fs.FS, now func() time.Time, log *slog.Logger) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create data folder: %w", err)
	}
	file := filepath.Join(dir, "mooring.db")
	// Create the file ourselves so its mode is 0600 from the first byte.
	f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create database file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("create database file: %w", err)
	}
	// FULL sync: a command recorded as running must survive a power cut, or a
	// resend after it could run the command a second time.
	// Immediate transactions take the write lock at BEGIN, so they wait out
	// busy_timeout; a deferred one that reads first fails at once with SQLITE_BUSY
	// when it tries to write while another connection is writing.
	dsn := "file:" + file + "?_txlock=immediate&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(4)
	s := &Store{DB: db}
	if err := s.migrate(dir, files, now, log); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.DB.Close() }

type migration struct {
	version int
	name    string
}

// listMigrations returns the migration files in version order.
func listMigrations(files fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(files, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	list := make([]migration, 0, len(entries))
	for _, e := range entries {
		version, err := strconv.Atoi(strings.SplitN(e.Name(), "_", 2)[0])
		if err != nil {
			return nil, fmt.Errorf("migration %q: name must start with a number: %w", e.Name(), err)
		}
		list = append(list, migration{version, e.Name()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].version < list[j].version })
	return list, nil
}

func (s *Store) migrate(dir string, files fs.FS, now func() time.Time, log *slog.Logger) error {
	if _, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	list, err := listMigrations(files)
	if err != nil {
		return err
	}
	applied, current, err := s.appliedVersions()
	if err != nil {
		return err
	}
	known := 0
	if len(list) > 0 {
		known = list[len(list)-1].version
	}
	if current > known {
		return fmt.Errorf( //nolint:staticcheck // shown to the user as written
			"This data folder was used by a newer Mooring (schema %d; this build knows %d). "+
				"Use the newer build, or restore a backup from %s", current, known, filepath.Join(dir, backupFolder))
	}
	var pending []migration
	for _, m := range list {
		if !applied[m.version] {
			pending = append(pending, m)
		}
	}
	if len(pending) > 0 && len(applied) > 0 {
		if err := s.backup(dir, current, now(), log); err != nil {
			return fmt.Errorf("back up the database before upgrading: %w", err)
		}
	}
	for _, m := range pending {
		body, err := fs.ReadFile(files, "migrations/"+m.name)
		if err != nil {
			return fmt.Errorf("read migration %q: %w", m.name, err)
		}
		if err := s.apply(m.version, string(body)); err != nil {
			return fmt.Errorf("apply migration %q: %w", m.name, err)
		}
		if log != nil {
			log.Info("store: migration applied", "version", m.version)
		}
	}
	return nil
}

// appliedVersions returns the recorded versions and the highest of them.
func (s *Store) appliedVersions() (map[int]bool, int, error) {
	rows, err := s.DB.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, 0, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	applied, highest := map[int]bool{}, 0
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, 0, fmt.Errorf("read schema_migrations: %w", err)
		}
		applied[v], highest = true, max(highest, v)
	}
	return applied, highest, rows.Err()
}

func (s *Store) apply(version int, body string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(body); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, unixepoch())`, version); err != nil {
		return err
	}
	return tx.Commit()
}
