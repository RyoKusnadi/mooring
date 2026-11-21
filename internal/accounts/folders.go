package accounts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func (s *Service) accountsDir() string { return filepath.Join(s.DataDir, "accounts") }

// makeFolder creates the managed folder of an account (mode 0700) and its shared link.
func (s *Service) makeFolder(id string, a Adapter) (string, error) {
	if err := os.MkdirAll(s.accountsDir(), 0o700); err != nil {
		return "", fmt.Errorf("make accounts folder: %w", err)
	}
	dir := filepath.Join(s.accountsDir(), id)
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("make account folder: %w", err)
	}
	if err := s.linkShared(dir, a); err != nil {
		return "", err
	}
	return dir, nil
}

// linkShared makes <dir>/<shared> a link to the same folder in the built-in
// login, so every account sees the same saved sessions. It creates the target
// folder if missing and never reads what is in it. A real folder or file already
// in the way is left alone.
func (s *Service) linkShared(dir string, a Adapter) error {
	if a.Shared == "" {
		return nil
	}
	target := filepath.Join(s.builtinDir(a), a.Shared)
	if err := os.MkdirAll(target, 0o700); err != nil {
		return fmt.Errorf("make shared folder: %w", err)
	}
	link := filepath.Join(dir, a.Shared)
	if cur, err := os.Readlink(link); err == nil {
		if cur == target {
			return nil
		}
		if err := os.Remove(link); err != nil {
			return fmt.Errorf("replace shared link: %w", err)
		}
	} else if _, err := os.Lstat(link); err == nil {
		return nil
	}
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("link shared folder: %w", err)
	}
	return nil
}

// retire moves a managed folder out of the way: it holds the login, so it is kept.
func (s *Service) retire(id string) error {
	src := filepath.Join(s.accountsDir(), id)
	if _, err := os.Lstat(src); err != nil {
		return nil
	}
	gone := filepath.Join(s.accountsDir(), "removed")
	if err := os.MkdirAll(gone, 0o700); err != nil {
		return fmt.Errorf("make removed folder: %w", err)
	}
	dst := filepath.Join(gone, id)
	for n := 2; ; n++ {
		if _, err := os.Lstat(dst); err != nil {
			break
		}
		dst = filepath.Join(gone, id+"-"+strconv.Itoa(n))
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("move account folder: %w", err)
	}
	return nil
}

// Env is the environment an agent starts with for an account: the folder
// variable for an account other than the built-in login. A managed folder that
// is missing is made again. An unknown account gives an error.
func (s *Service) Env(agent, id string) ([]string, error) {
	a, ok := s.Adapters[agent]
	if !ok || id == "" || id == BuiltinID(agent) {
		return nil, nil
	}
	if s.isRemoving(id) {
		return nil, apiErr("NOT_FOUND", "That account is being removed.")
	}
	r, err := s.get(id)
	if err != nil {
		return nil, err
	}
	if r.agent != agent {
		return nil, apiErr("BAD_REQUEST", "That account belongs to another agent.")
	}
	if r.managed {
		if _, err := s.makeFolder(r.id, a); err != nil {
			return nil, err
		}
	}
	return []string{a.Env + "=" + r.dir}, nil
}
