package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func migs(n int) fstest.MapFS {
	f := fstest.MapFS{}
	for i := 1; i <= n; i++ {
		name := "migrations/00" + string(rune('0'+i)) + "_t.sql"
		f[name] = &fstest.MapFile{Data: []byte("CREATE TABLE t" + string(rune('0'+i)) + " (a INTEGER); INSERT INTO t1 VALUES (1);")}
	}
	return f
}

func clock(t time.Time) func() time.Time { return func() time.Time { return t } }

func backups(t *testing.T, dir string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(dir, "backups", "*.db"))
	for i := range m {
		m[i] = filepath.Base(m[i])
	}
	return m
}

func TestFreshDatabaseIsNotBackedUp(t *testing.T) {
	dir := t.TempDir()
	s, err := open(dir, migs(2), clock(time.Now()), nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	if got := backups(t, dir); len(got) != 0 {
		t.Fatalf("backups on a fresh database: %v", got)
	}
}

func TestPendingMigrationBacksUpFirstAndKeepsThree(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	s, err := open(dir, migs(1), clock(start), nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	for n := 2; n <= 6; n++ {
		s, err = open(dir, migs(n), clock(start.Add(time.Duration(n)*time.Hour)), nil)
		if err != nil {
			t.Fatal(err)
		}
		_ = s.Close()
	}
	got := backups(t, dir)
	if len(got) != 3 {
		t.Fatalf("kept %v", got)
	}
	want := map[string]bool{
		"mooring-schema3-20261101T140000Z.db": true,
		"mooring-schema4-20261101T150000Z.db": true,
		"mooring-schema5-20261101T160000Z.db": true,
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("kept %v", got)
		}
	}
	info, err := os.Stat(filepath.Join(dir, "backups", got[0]))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode: %v %v", info, err)
	}
	// The same build again has nothing pending, so no new backup.
	s, _ = open(dir, migs(6), clock(start.Add(24*time.Hour)), nil)
	_ = s.Close()
	if len(backups(t, dir)) != 3 {
		t.Fatalf("backup on a start without migrations: %v", backups(t, dir))
	}
}

func TestBackupHoldsTheOldSchema(t *testing.T) {
	dir := t.TempDir()
	s, _ := open(dir, migs(1), clock(time.Now()), nil)
	_ = s.Close()
	s, err := open(dir, migs(2), clock(time.Now()), nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	b := backups(t, dir)
	if len(b) != 1 || !strings.HasPrefix(b[0], "mooring-schema1-") {
		t.Fatalf("backups: %v", b)
	}
	// Restoring it gives a database at schema 1 that the old build opens without migrating.
	raw, err := os.ReadFile(filepath.Join(dir, "backups", b[0]))
	if err != nil {
		t.Fatal(err)
	}
	restored := t.TempDir()
	if err := os.WriteFile(filepath.Join(restored, "mooring.db"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	old, err := open(restored, migs(1), clock(time.Now()), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = old.Close() }()
	var rows, tables int
	_ = old.DB.QueryRow(`SELECT COUNT(*) FROM t1`).Scan(&rows)
	_ = old.DB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 't2'`).Scan(&tables)
	if rows != 1 || tables != 0 {
		t.Fatalf("restored: rows=%d t2=%d", rows, tables)
	}
	if len(backups(t, restored)) != 0 {
		t.Fatal("the old build backed up again")
	}
}

func TestFailedBackupStopsMigration(t *testing.T) {
	dir := t.TempDir()
	s, _ := open(dir, migs(1), clock(time.Now()), nil)
	_ = s.Close()
	if err := os.WriteFile(filepath.Join(dir, "backups"), []byte("a file, not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := open(dir, migs(2), clock(time.Now()), nil)
	if err == nil || !strings.Contains(err.Error(), "back up the database before upgrading") {
		t.Fatalf("err = %v", err)
	}
	s, err = open(dir, migs(1), clock(time.Now()), nil)
	if err != nil {
		t.Fatalf("old build can still open: %v", err)
	}
	var n int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n)
	_ = s.Close()
	if n != 1 {
		t.Fatalf("migrations applied after a failed backup: %d", n)
	}
}

func TestNewerSchemaIsRefused(t *testing.T) {
	dir := t.TempDir()
	s, _ := open(dir, migs(3), clock(time.Now()), nil)
	_ = s.Close()
	_, err := open(dir, migs(2), clock(time.Now()), nil)
	want := "newer Mooring (schema 3; this build knows 2)"
	if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), filepath.Join(dir, "backups")) {
		t.Fatalf("err = %v", err)
	}
	if got := backups(t, dir); len(got) != 0 {
		t.Fatalf("refusal wrote %v", got)
	}
}
