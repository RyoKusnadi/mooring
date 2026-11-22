package accounts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// row is one saved account.
type row struct {
	id, agent, name, dir string
	managed              bool
	login                string
	usage                Usage
	position             int
}

func (s *Service) scan(sc interface{ Scan(...any) error }) (row, error) {
	var r row
	var raw string
	if err := sc.Scan(&r.id, &r.agent, &r.name, &r.dir, &r.managed, &r.login, &raw, &r.position); err != nil {
		return row{}, err
	}
	r.usage = newUsage()
	if raw != "" && json.Unmarshal([]byte(raw), &r.usage) != nil {
		r.usage = newUsage()
	}
	if r.usage.Windows == nil {
		r.usage.Windows = []Window{}
	}
	return r, nil
}

const cols = `id, agent, name, dir, managed, login, usage, position`

// get reads an account; the built-in login of an agent exists even before it has a row.
func (s *Service) get(id string) (row, error) {
	r, err := s.scan(s.DB.QueryRow(`SELECT `+cols+` FROM accounts WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		for agent := range s.Adapters {
			if id == BuiltinID(agent) {
				return s.builtin(agent), nil
			}
		}
		return row{}, apiErr("NOT_FOUND", "That account does not exist.")
	}
	if err != nil {
		return row{}, fmt.Errorf("read account: %w", err)
	}
	return r, nil
}

func (s *Service) builtin(agent string) row {
	return row{id: BuiltinID(agent), agent: agent, name: "Default", dir: s.builtinDir(s.Adapters[agent]), login: LoginUnknown, usage: newUsage()}
}

// list reads every account, the built-in logins included.
func (s *Service) list() ([]row, error) {
	rows, err := s.DB.Query(`SELECT ` + cols + ` FROM accounts`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []row
	have := map[string]bool{}
	for rows.Next() {
		r, err := s.scan(rows)
		if err != nil {
			return nil, fmt.Errorf("read account row: %w", err)
		}
		if _, ok := s.Adapters[r.agent]; !ok {
			continue
		}
		if r.id == BuiltinID(r.agent) {
			r.dir = s.builtinDir(s.Adapters[r.agent])
		}
		have[r.id] = true
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	for agent := range s.Adapters {
		if !have[BuiltinID(agent)] {
			out = append(out, s.builtin(agent))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.agent != b.agent {
			return a.agent < b.agent
		}
		if (a.position == 0) != (b.position == 0) {
			return a.position == 0
		}
		if a.position != b.position {
			return a.position < b.position
		}
		return a.id < b.id
	})
	return out, nil
}

// put writes an account row (insert or update).
func (s *Service) put(r row) error {
	raw, _ := json.Marshal(r.usage)
	_, err := s.DB.Exec(`INSERT INTO accounts (id, agent, name, dir, managed, login, usage, position) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, dir = excluded.dir, login = excluded.login, usage = excluded.usage`,
		r.id, r.agent, r.name, r.dir, r.managed, r.login, string(raw), r.position)
	if err != nil {
		return fmt.Errorf("save account: %w", err)
	}
	return nil
}

// defaultID is the account new projects use for an agent.
func (s *Service) defaultID(agent string) string {
	var id string
	if err := s.DB.QueryRow(`SELECT account_id FROM account_defaults WHERE agent = ?`, agent).Scan(&id); err != nil {
		return BuiltinID(agent)
	}
	return id
}

// projectChoice reads the account a project chose for an agent, "" when it did not.
func (s *Service) projectChoice(projectID, agent string) string {
	var id string
	_ = s.DB.QueryRow(`SELECT account_id FROM project_accounts WHERE project_id = ? AND agent = ?`, projectID, agent).Scan(&id)
	return id
}

func (s *Service) view(r row, defaults map[string]string) Account {
	u := r.usage
	if u.LimitHit != 0 && !u.limited(s.now().UnixMilli()) {
		u.LimitHit, u.LimitResetsAt = 0, 0
	}
	return Account{ID: r.id, Agent: r.agent, Name: r.name, Dir: r.dir, Managed: r.managed,
		IsDefault: defaults[r.agent] == r.id, Login: r.login, Usage: u}
}

func (s *Service) defaults() map[string]string {
	out := map[string]string{}
	for agent := range s.Adapters {
		out[agent] = s.defaultID(agent)
	}
	return out
}
