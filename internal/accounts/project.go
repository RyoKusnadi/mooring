package accounts

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Effective is the account a project uses for an agent: its own choice, else
// the agent's default. "" when the agent has no adapter.
func (s *Service) Effective(projectID, agent string) string {
	if !s.Has(agent) {
		return ""
	}
	if id := s.projectChoice(projectID, agent); id != "" {
		return id
	}
	return s.defaultID(agent)
}

// Pick settles the account of a chat. An empty id means the project's account.
// A named account must exist and belong to the agent, unless lenient: then a
// missing one falls back to the project's account (a chat saved before the
// account was removed).
func (s *Service) Pick(projectID, agent, id string, lenient bool) (string, error) {
	if !s.Has(agent) {
		if id != "" {
			return "", apiErr("BAD_REQUEST", "%s has no separate logins.", s.agentName(agent))
		}
		return "", nil
	}
	if id == "" {
		return s.Effective(projectID, agent), nil
	}
	var r row
	var err error
	if s.isRemoving(id) {
		err = apiErr("NOT_FOUND", "That account is being removed.")
	} else if r, err = s.get(id); err == nil && r.agent != agent {
		err = apiErr("BAD_REQUEST", "That account belongs to another agent.")
	}
	if err != nil {
		if lenient {
			return s.Effective(projectID, agent), nil
		}
		return "", err
	}
	return id, nil
}

func (s *Service) setProjectAccount(projectID, agent, accountID string) (any, error) {
	if !s.Has(agent) {
		return nil, apiErr("BAD_REQUEST", "%s has no separate logins.", s.agentName(agent))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM projects WHERE id = ?`, projectID).Scan(&n); err != nil || n == 0 {
		return nil, apiErr("NOT_FOUND", "That project no longer exists.")
	}
	r, err := s.get(accountID)
	if err != nil {
		return nil, err
	}
	if r.agent != agent {
		return nil, apiErr("BAD_REQUEST", "That account belongs to another agent.")
	}
	if err := s.ensureRow(r); err != nil {
		return nil, err
	}
	// The choice is saved first, so a chat that starts during the move already
	// takes the new account; it is put back if the move fails.
	prev := s.projectChoice(projectID, agent)
	if _, err := s.DB.Exec(`INSERT INTO project_accounts (project_id, agent, account_id) VALUES (?, ?, ?)
		ON CONFLICT(project_id, agent) DO UPDATE SET account_id = excluded.account_id`, projectID, agent, accountID); err != nil {
		return nil, fmt.Errorf("save project account: %w", err)
	}
	restarted := []string{}
	if s.chats != nil {
		// Stops and restarts the open chats; fails first when one is answering.
		if restarted, err = s.chats.MoveAccount(projectID, agent, accountID); err != nil {
			s.restoreChoice(projectID, agent, prev)
			return nil, err
		}
	}
	s.Pub.Publish("project.accountChanged", map[string]string{"projectId": projectID, "agent": agent, "accountId": accountID})
	if restarted == nil {
		restarted = []string{}
	}
	return map[string]any{"restarted": restarted}, nil
}

// login opens a terminal in the project with the account's folder variable set.
func (s *Service) login(id, projectID string) (any, error) {
	r, err := s.get(id)
	if err != nil {
		return nil, err
	}
	env, err := s.Env(r.agent, id)
	if err != nil {
		return nil, err
	}
	if s.terminals == nil {
		return nil, apiErr("INTERNAL", "Terminals are not available.")
	}
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("make terminal id: %w", err)
	}
	tid := "login-" + hex.EncodeToString(b)
	hint := s.Adapters[r.agent].Login
	banner := fmt.Sprintf("Account %q for %s. To sign in, run: %s\r\n", r.name, s.agentName(r.agent), hint)
	if err := s.terminals.OpenWith(projectID, tid, env, banner); err != nil {
		return nil, err
	}
	return map[string]string{"terminalId": tid}, nil
}

// Snapshot is the body of GET /api/accounts.
func (s *Service) Snapshot() (map[string]any, error) {
	rows, err := s.list()
	if err != nil {
		return nil, err
	}
	defaults := s.defaults()
	out := make([]Account, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.view(r, defaults))
	}
	projects := map[string]map[string]string{}
	pr, err := s.DB.Query(`SELECT id FROM projects`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = pr.Close() }()
	for pr.Next() {
		var id string
		if err := pr.Scan(&id); err != nil {
			return nil, fmt.Errorf("read project: %w", err)
		}
		m := map[string]string{}
		for agent := range s.Adapters {
			m[agent] = s.Effective(id, agent)
		}
		projects[id] = m
	}
	if err := pr.Err(); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return map[string]any{"accounts": out, "projects": projects}, nil
}

// Mount registers GET /api/accounts. wrap adds the session check.
func (s *Service) Mount(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /api/accounts", wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body, err := s.Snapshot()
		status := http.StatusOK
		if err != nil {
			s.log.Error("accounts: list failed", "err", err)
			status = http.StatusInternalServerError
			body = map[string]any{"ok": false, "error": &ledger.Error{Code: "INTERNAL", Message: "Something went wrong on the server."}}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	})))
}

func (s *Service) restoreChoice(projectID, agent, prev string) {
	var err error
	if prev == "" {
		_, err = s.DB.Exec(`DELETE FROM project_accounts WHERE project_id = ? AND agent = ?`, projectID, agent)
	} else {
		_, err = s.DB.Exec(`UPDATE project_accounts SET account_id = ? WHERE project_id = ? AND agent = ?`, prev, projectID, agent)
	}
	if err != nil {
		s.log.Error("accounts: could not put back the project's account", "project", projectID, "err", err)
	}
}
