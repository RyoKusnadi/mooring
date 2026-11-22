package accounts

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/paths"
)

// Handles reports whether name is an account command.
func Handles(name string) bool {
	return strings.HasPrefix(name, "account.") || name == "project.setAccount"
}

type saveArgs struct {
	Account struct {
		ID    string `json:"id"`
		Agent string `json:"agent"`
		Name  string `json:"name"`
		Dir   string `json:"dir"`
	} `json:"account"`
}

// Run runs an account command. Errors are *ledger.Error.
func (s *Service) Run(cmd events.Command) (any, error) {
	var a struct {
		saveArgs
		ID        string `json:"id"`
		ProjectID string `json:"projectId"`
		Agent     string `json:"agent"`
		AccountID string `json:"accountId"`
	}
	if err := json.Unmarshal(cmd.Args, &a); err != nil {
		return nil, apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	switch cmd.Name {
	case "account.save":
		return s.save(a.Account.ID, a.Account.Agent, a.Account.Name, a.Account.Dir)
	case "account.remove":
		return map[string]any{}, s.remove(a.ID)
	case "account.setDefault":
		return s.setDefault(a.ID)
	case "account.login":
		return s.login(a.ID, a.ProjectID)
	case "project.setAccount":
		return s.setProjectAccount(a.ProjectID, a.Agent, a.AccountID)
	}
	return nil, apiErr("BAD_REQUEST", "Unknown command %s.", cmd.Name)
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n < 1 || n > maxName || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", apiErr("BAD_REQUEST", "An account name needs 1 to %d characters.", maxName)
	}
	return name, nil
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("make account id: %w", err)
	}
	return "a" + hex.EncodeToString(b), nil
}

// resolveDir checks a folder from the browser: inside the home folder, existing, a folder.
func (s *Service) resolveDir(p string) (string, error) {
	if !strings.HasPrefix(p, "/") || strings.ContainsRune(p, 0) {
		return "", apiErr("BAD_REQUEST", "The account folder must be an absolute path.")
	}
	got, err := paths.Resolve(s.Home, p)
	switch {
	case errors.Is(err, paths.ErrOutsideRoot):
		return "", apiErr("OUTSIDE_ROOT", "That folder is outside your home folder.")
	case err != nil:
		return "", apiErr("BAD_REQUEST", "That path cannot be used.")
	}
	if fi, err := os.Stat(got); err != nil || !fi.IsDir() {
		return "", apiErr("BAD_REQUEST", "The account folder must be an existing folder.")
	}
	return got, nil
}

func (s *Service) save(id, agent, name, dir string) (any, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var cur row
	if id != "" {
		if cur, err = s.get(id); err != nil {
			return nil, err
		}
		if id == BuiltinID(cur.agent) {
			return nil, apiErr("BAD_REQUEST", "The agent's own login cannot be changed.")
		}
		if agent != "" && agent != cur.agent {
			return nil, apiErr("BAD_REQUEST", "An account cannot move to another agent.")
		}
		agent = cur.agent
	}
	adapter, ok := s.Adapters[agent]
	if !ok {
		return nil, apiErr("BAD_REQUEST", "%s has no separate logins.", s.agentName(agent))
	}
	if dir != "" {
		if id != "" && cur.managed {
			return nil, apiErr("BAD_REQUEST", "The folder of this account is managed by Mooring.")
		}
		if dir, err = s.resolveDir(dir); err != nil {
			return nil, err
		}
	}
	all, err := s.list()
	if err != nil {
		return nil, err
	}
	if dir != "" {
		if err := s.dirFree(dir, id, agent, adapter, all); err != nil {
			return nil, err
		}
	}
	users := 0
	for _, r := range all {
		if r.agent != agent {
			continue
		}
		if r.id != id && strings.EqualFold(r.name, name) {
			return nil, apiErr("ALREADY_EXISTS", "There is already an account called %q.", name)
		}
		if r.id != BuiltinID(agent) {
			users++
		}
	}
	if id == "" {
		return s.add(agent, name, dir, users, adapter)
	}
	if dir != "" && dir != cur.dir && s.chats != nil && s.chats.LiveOn(id) > 0 {
		return nil, apiErr("BUSY", "A chat is running on this account. Close it first.")
	}
	cur.name = name
	if dir != "" {
		cur.dir = dir
	}
	if err := s.put(cur); err != nil {
		return nil, err
	}
	return s.announce(cur), nil
}

func (s *Service) add(agent, name, dir string, users int, adapter Adapter) (any, error) {
	if users >= MaxPerAgent {
		return nil, apiErr("LIMIT_REACHED", "There are already %d accounts for %s.", MaxPerAgent, s.agentName(agent))
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	r := row{id: id, agent: agent, name: name, dir: dir, managed: dir == "", login: LoginUnknown, usage: newUsage()}
	if r.managed {
		if r.dir, err = s.makeFolder(id, adapter); err != nil {
			s.log.Error("accounts: make folder", "err", err)
			return nil, apiErr("INTERNAL", "The account folder could not be made.")
		}
	}
	var max int
	_ = s.DB.QueryRow(`SELECT COALESCE(MAX(position), 0) FROM accounts WHERE agent = ?`, agent).Scan(&max)
	r.position = max + 1
	if err := s.put(r); err != nil {
		return nil, err
	}
	s.log.Info("accounts: added", "agent", agent, "id", id, "managed", r.managed)
	return s.announce(r), nil
}

// announce publishes the account's new state and returns it.
func (s *Service) announce(r row) Account {
	a := s.view(r, s.defaults())
	s.Pub.Publish("account.changed", a)
	return a
}

func (s *Service) remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.get(id)
	if err != nil {
		return err
	}
	if id == BuiltinID(r.agent) {
		return apiErr("BAD_REQUEST", "The agent's own login cannot be removed.")
	}
	// From here no chat can start on the account (Pick and Env refuse it), so the
	// count below cannot be overtaken by a chat that starts later.
	s.setRemoving(id, true)
	defer s.setRemoving(id, false)
	if s.chats != nil && s.chats.LiveOn(id) > 0 {
		return apiErr("BUSY", "A chat is running on this account. Close it first.")
	}
	rows, err := s.DB.Query(`SELECT project_id FROM project_accounts WHERE account_id = ?`, id)
	if err != nil {
		return fmt.Errorf("list projects of account: %w", err)
	}
	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			projects = append(projects, p)
		}
	}
	_ = rows.Close()
	wasDefault := s.defaultID(r.agent) == id
	if _, err := s.DB.Exec(`DELETE FROM accounts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("remove account: %w", err)
	}
	if r.managed {
		if err := s.retire(id); err != nil {
			s.log.Error("accounts: keep folder of removed account", "id", id, "err", err)
		}
	}
	s.Pub.Publish("account.removed", map[string]string{"id": id})
	for _, p := range projects {
		s.Pub.Publish("project.accountChanged", map[string]string{"projectId": p, "agent": r.agent, "accountId": s.defaultID(r.agent)})
	}
	if wasDefault {
		if b, err := s.get(BuiltinID(r.agent)); err == nil {
			s.announce(b)
		}
	}
	return nil
}

func (s *Service) setDefault(id string) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.get(id)
	if err != nil {
		return nil, err
	}
	prev := s.defaultID(r.agent)
	if err := s.ensureRow(r); err != nil {
		return nil, err
	}
	if _, err := s.DB.Exec(`INSERT INTO account_defaults (agent, account_id) VALUES (?, ?)
		ON CONFLICT(agent) DO UPDATE SET account_id = excluded.account_id`, r.agent, id); err != nil {
		return nil, fmt.Errorf("save default account: %w", err)
	}
	if prev != id {
		if old, err := s.get(prev); err == nil {
			s.announce(old)
		}
	}
	return s.announce(r), nil
}

// ensureRow saves the built-in login's row, which other tables point at.
func (s *Service) ensureRow(r row) error {
	if r.id != BuiltinID(r.agent) {
		return nil
	}
	return s.put(r)
}

// dirFree refuses a folder that another login already uses: the agent's own
// folder, another account's, or anything in or above Mooring's account folders.
func (s *Service) dirFree(dir, self, agent string, a Adapter, all []row) error {
	if b := s.builtinDir(a); b != "" && sameOrNested(dir, b) {
		return apiErr("BAD_REQUEST", "That is the folder of the agent's own login. Choose another folder.")
	}
	if sameOrNested(dir, s.accountsDir()) || sameOrNested(s.accountsDir(), dir) {
		return apiErr("BAD_REQUEST", "That folder is inside Mooring's own account folders.")
	}
	for _, r := range all {
		if r.id != self && r.dir != "" && sameOrNested(dir, r.dir) {
			return apiErr("BAD_REQUEST", "Another account already uses that folder.")
		}
	}
	return nil
}

// sameOrNested reports whether a is b or inside b, or b is inside a.
func sameOrNested(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	return a == b || strings.HasPrefix(a, b+string(filepath.Separator)) || strings.HasPrefix(b, a+string(filepath.Separator))
}
