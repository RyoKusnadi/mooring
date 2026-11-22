package lsp

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

func (s *Service) allowedCommand(id string) ([]string, bool) {
	var raw string
	err := s.db.QueryRow(`SELECT command FROM lsp_consent WHERE server_id = ?`, id).Scan(&raw)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.log.Error("lsp: read consent", "server", id, "err", err)
		}
		return nil, false
	}
	var cmd []string
	if json.Unmarshal([]byte(raw), &cmd) != nil {
		return nil, false
	}
	return cmd, true
}

func (s *Service) saveConsent(id string, cmd []string) error {
	raw, _ := json.Marshal(cmd)
	_, err := s.db.Exec(`INSERT INTO lsp_consent (server_id, command, allowed_at) VALUES (?, ?, ?)
		ON CONFLICT(server_id) DO UPDATE SET command = excluded.command, allowed_at = excluded.allowed_at`,
		id, string(raw), s.cfg.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("save consent: %w", err)
	}
	return nil
}

func (s *Service) loadCustom() (map[string]Def, error) {
	rows, err := s.db.Query(`SELECT id, name, endings, language_id, command FROM lsp_custom`)
	if err != nil {
		return nil, fmt.Errorf("list custom servers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]Def{}
	for rows.Next() {
		var d Def
		var endings, command string
		if err := rows.Scan(&d.ID, &d.Name, &endings, &d.LanguageID, &command); err != nil {
			return nil, fmt.Errorf("read custom server: %w", err)
		}
		if json.Unmarshal([]byte(endings), &d.Endings) != nil || json.Unmarshal([]byte(command), &d.Command) != nil || len(d.Command) == 0 {
			s.log.Error("lsp: saved custom server unreadable", "server", d.ID)
			continue
		}
		d.Custom = true
		out[d.ID] = d
	}
	return out, rows.Err()
}

func (s *Service) saveCustom(d Def) error {
	endings, _ := json.Marshal(d.Endings)
	command, _ := json.Marshal(d.Command)
	_, err := s.db.Exec(`INSERT INTO lsp_custom (id, name, endings, language_id, command) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, endings = excluded.endings,
		language_id = excluded.language_id, command = excluded.command`,
		d.ID, d.Name, string(endings), d.LanguageID, string(command))
	if err != nil {
		return fmt.Errorf("save custom server: %w", err)
	}
	return nil
}

func (s *Service) deleteCustom(id string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM lsp_custom WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("remove custom server: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
