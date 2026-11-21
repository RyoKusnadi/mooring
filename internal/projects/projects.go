package projects

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Project is a folder in the sidebar.
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Position  int    `json:"position"`
	CreatedAt int64  `json:"createdAt"` // Unix milliseconds
}

const maxNameChars = 200

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > maxNameChars {
		return "", apiErr("BAD_REQUEST", "A project name needs 1 to %d characters.", maxNameChars)
	}
	return name, nil
}

// folderName checks a new folder's name: one path segment, nothing sneaky.
func folderName(name string) (string, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`+"\x00") || len(name) > 255 {
		return "", apiErr("BAD_REQUEST", "A folder name cannot be empty, contain slashes, or be . or ..")
	}
	return name, nil
}

func (s *Service) insertProject(path, name string) (Project, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	id, err := newID()
	if err != nil {
		return Project{}, err
	}
	p := Project{ID: id, Name: name, Path: path, CreatedAt: s.now().UnixMilli()}
	tx, err := s.db.Begin()
	if err != nil {
		return Project{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM projects WHERE path = ?`, path).Scan(&exists); err != nil {
		return Project{}, fmt.Errorf("check path: %w", err)
	}
	if exists > 0 {
		return Project{}, apiErr("ALREADY_ADDED", "That folder is already a project.")
	}
	if err := tx.QueryRow(`SELECT COALESCE(MAX(position) + 1, 0) FROM projects`).Scan(&p.Position); err != nil {
		return Project{}, fmt.Errorf("next position: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO projects (id, name, path, position, created_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Path, p.Position, p.CreatedAt); err != nil {
		return Project{}, fmt.Errorf("insert project: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("commit: %w", err)
	}
	s.pub.Publish("project.added", p)
	return p, nil
}

func (s *Service) projectAdd(raw json.RawMessage) (any, error) {
	var a struct{ Path, Name string }
	if err := decode(raw, &a); err != nil {
		return nil, err
	}
	dir, err := s.resolveDir(a.Path)
	if err != nil {
		return nil, err
	}
	name := a.Name
	if name == "" {
		name = filepath.Base(dir)
	}
	if name, err = cleanName(name); err != nil {
		return nil, err
	}
	return s.insertProject(dir, name)
}

func (s *Service) projectCreate(raw json.RawMessage) (any, error) {
	var a struct{ Parent, Name string }
	if err := decode(raw, &a); err != nil {
		return nil, err
	}
	folder, err := folderName(a.Name)
	if err != nil {
		return nil, err
	}
	parent, err := s.resolveDir(a.Parent)
	if err != nil {
		return nil, err
	}
	dest, err := s.resolve(filepath.Join(parent, folder))
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(dest, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, apiErr("ALREADY_EXISTS", "%q already exists there.", folder)
		}
		return nil, fmt.Errorf("create folder: %w", err)
	}
	p, err := s.insertProject(dest, folder)
	if err != nil {
		_ = os.Remove(dest)
		return nil, err
	}
	return p, nil
}

func (s *Service) projectRename(raw json.RawMessage) (any, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var a struct{ ID, Name string }
	if err := decode(raw, &a); err != nil {
		return nil, err
	}
	name, err := cleanName(a.Name)
	if err != nil {
		return nil, err
	}
	res, err := s.db.Exec(`UPDATE projects SET name = ? WHERE id = ?`, name, a.ID)
	if err != nil {
		return nil, fmt.Errorf("rename project: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, apiErr("NOT_FOUND", "That project no longer exists.")
	}
	p, err := s.get(a.ID)
	if err != nil {
		return nil, err
	}
	s.pub.Publish("project.changed", p)
	return p, nil
}

func (s *Service) get(id string) (Project, error) {
	var p Project
	err := s.db.QueryRow(`SELECT id, name, path, position, created_at FROM projects WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.Path, &p.Position, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, apiErr("NOT_FOUND", "That project no longer exists.")
	}
	if err != nil {
		return p, fmt.Errorf("read project: %w", err)
	}
	return p, nil
}

func (s *Service) list() ([]Project, error) {
	rows, err := s.db.Query(`SELECT id, name, path, position, created_at FROM projects ORDER BY position, created_at`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Position, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("read project row: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) projectReorder(raw json.RawMessage) (any, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var a struct {
		IDs []string `json:"ids"`
	}
	if err := decode(raw, &a); err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.Query(`SELECT id FROM projects`)
	if err != nil {
		return nil, fmt.Errorf("list ids: %w", err)
	}
	have := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("read id: %w", err)
		}
		have[id] = true
	}
	_ = rows.Close()
	if len(a.IDs) != len(have) {
		return nil, apiErr("BAD_REQUEST", "The new order must list every project once.")
	}
	seen := map[string]bool{}
	for i, id := range a.IDs {
		if !have[id] || seen[id] {
			return nil, apiErr("BAD_REQUEST", "The new order must list every project once.")
		}
		seen[id] = true
		if _, err := tx.Exec(`UPDATE projects SET position = ? WHERE id = ?`, i, id); err != nil {
			return nil, fmt.Errorf("set position: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	s.pub.Publish("project.reordered", map[string]any{"ids": a.IDs})
	return struct{}{}, nil
}

func (s *Service) projectRemove(raw json.RawMessage) (any, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var a struct{ ID string }
	if err := decode(raw, &a); err != nil {
		return nil, err
	}
	res, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, a.ID)
	if err != nil {
		return nil, fmt.Errorf("remove project: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, apiErr("NOT_FOUND", "That project no longer exists.")
	}
	s.pub.Publish("project.removed", map[string]string{"id": a.ID})
	return struct{}{}, nil
}

// Dir is the folder of a project, checked again against the root: where its
// terminals start.
func (s *Service) Dir(id string) (string, error) {
	p, err := s.get(id)
	if err != nil {
		return "", err
	}
	return s.resolveDir(p.Path)
}
