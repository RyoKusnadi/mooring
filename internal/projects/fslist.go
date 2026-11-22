package projects

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type entry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	Git  bool   `json:"git"`
}

type listing struct {
	Path    string  `json:"path"`
	Parent  string  `json:"parent"`
	Entries []entry `json:"entries"`
}

// fsList lists the folders in a folder. Files are left out.
func (s *Service) fsList(raw json.RawMessage) (any, error) {
	var a struct {
		Path string `json:"path"`
	}
	if err := decode(raw, &a); err != nil {
		return nil, err
	}
	dir, err := s.resolveDir(a.Path)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}
	showHidden := strings.HasPrefix(filepath.Base(dir), ".")
	out := listing{Path: dir, Entries: []entry{}}
	if dir != s.root {
		out.Parent = filepath.Dir(dir)
	}
	for _, it := range items {
		name := it.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dir, name)
		info, err := os.Stat(full) // follows symlinks, so a linked folder counts
		if err != nil || !info.IsDir() {
			continue
		}
		_, gitErr := os.Stat(filepath.Join(full, ".git"))
		out.Entries = append(out.Entries, entry{Name: name, Dir: true, Git: gitErr == nil})
	}
	sort.Slice(out.Entries, func(i, j int) bool {
		return strings.ToLower(out.Entries[i].Name) < strings.ToLower(out.Entries[j].Name)
	})
	return out, nil
}
