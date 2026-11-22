package store

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"
)

const (
	backupFolder = "backups"
	backupsKept  = 3
	backupStamp  = "20060102T150405Z"
)

var backupName = regexp.MustCompile(`^mooring-schema(\d+)-(\d{8}T\d{6}Z)\.db$`)

// backup copies the database to <dir>/backups before a migration, then keeps
// the newest backupsKept files.
func (s *Store) backup(dir string, schema int, at time.Time, log *slog.Logger) error {
	folder := filepath.Join(dir, backupFolder)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return err
	}
	file := filepath.Join(folder, fmt.Sprintf("mooring-schema%d-%s.db", schema, at.UTC().Format(backupStamp)))
	if _, err := s.DB.Exec(`VACUUM INTO ?`, file); err != nil {
		return err
	}
	if err := os.Chmod(file, 0o600); err != nil {
		return err
	}
	if log != nil {
		log.Info("store: backup written before migrating", "file", file, "schema", schema)
	}
	pruneBackups(folder, log)
	return nil
}

// pruneBackups deletes all but the newest backupsKept backups. Files that do not
// match the backup name are left alone. A failed delete is logged, not fatal.
func pruneBackups(folder string, log *slog.Logger) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		if log != nil {
			log.Warn("store: list backups", "err", err)
		}
		return
	}
	type found struct {
		name, stamp string
		schema      int
	}
	var all []found
	for _, e := range entries {
		if m := backupName.FindStringSubmatch(e.Name()); m != nil {
			n, _ := strconv.Atoi(m[1])
			all = append(all, found{e.Name(), m[2], n})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].stamp != all[j].stamp {
			return all[i].stamp > all[j].stamp
		}
		return all[i].schema > all[j].schema
	})
	for i := backupsKept; i < len(all); i++ {
		if err := os.Remove(filepath.Join(folder, all[i].name)); err != nil && log != nil {
			log.Warn("store: delete old backup", "file", all[i].name, "err", err)
		}
	}
}
