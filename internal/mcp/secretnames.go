package mcp

import (
	"errors"
	"fmt"

	"github.com/ryokusnadi/mooring/internal/secrets"
)

// Secret store names. They fit the store's name rule (no slash), and var names
// are at most 64 characters, so a name stays under 128.
func envKey(id, name string) string    { return "mcp:" + id + ":env:" + name }
func headerKey(id, name string) string { return "mcp:" + id + ":header:" + name }
func tokenKey(id string) string        { return "mcp:" + id + ":token" }
func clientKey(id string) string       { return "mcp:" + id + ":client" }

func (s *Service) secretGet(name string) (string, bool, error) {
	if s.Secrets == nil {
		return "", false, nil
	}
	v, err := s.Secrets.Get(name)
	if errors.Is(err, secrets.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read a saved secret: %w", err)
	}
	return v, true, nil
}

func (s *Service) secretDelete(name string) {
	if s.Secrets == nil {
		return
	}
	if err := s.Secrets.Delete(name); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		s.log.Warn("tool server: could not delete a secret", "err", err)
	}
}

// values reads the saved values of vars (by key function). Missing required ones
// are returned by name.
func (s *Service) values(id string, vars []Var, key func(id, name string) string) (vals map[string]string, missing []string, err error) {
	vals = map[string]string{}
	for _, v := range vars {
		val, ok, err := s.secretGet(key(id, v.Name))
		if err != nil {
			return nil, nil, err
		}
		if ok && val != "" {
			vals[v.Name] = val
		} else if v.Required {
			missing = append(missing, v.Name)
		}
	}
	return vals, missing, nil
}
