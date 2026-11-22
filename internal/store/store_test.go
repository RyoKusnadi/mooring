package store

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesPrivateDatabaseAndMigratesOnce(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "mooring.db"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("db file: %v, %v", info, err)
	}
	var mode string
	if err := s.DB.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q, %v", mode, err)
	}
	var fk int
	if err := s.DB.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d, %v", fk, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	files, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("migration files: %v %v", files, err)
	}
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n != len(files) {
		t.Fatalf("migrations recorded = %d of %d, %v", n, len(files), err)
	}
}
