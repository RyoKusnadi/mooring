package projects

import (
	"encoding/json"
	"fmt"
)

const maxLayoutBytes = 64 << 10

type layoutArgs struct {
	ProjectID string          `json:"projectId"`
	Layout    json.RawMessage `json:"layout"`
}

func (s *Service) layoutSave(raw json.RawMessage) (any, error) {
	var a layoutArgs
	if err := decode(raw, &a); err != nil {
		return nil, err
	}
	var probe map[string]json.RawMessage
	if len(a.Layout) == 0 || json.Unmarshal(a.Layout, &probe) != nil || probe == nil {
		return nil, apiErr("BAD_REQUEST", "The layout must be a JSON object.")
	}
	if len(a.Layout) > maxLayoutBytes {
		return nil, apiErr("BAD_REQUEST", "The layout is larger than 64 KiB.")
	}
	if _, err := s.get(a.ProjectID); err != nil {
		return nil, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO layouts (project_id, body, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET body = excluded.body, updated_at = excluded.updated_at`,
		a.ProjectID, string(a.Layout), s.now().UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("save layout: %w", err)
	}
	s.pub.Publish("layout.changed", a)
	return struct{}{}, nil
}

// Snapshot is the state a client resets from.
func (s *Service) Snapshot() (any, error) {
	projects, err := s.list()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT project_id, body FROM layouts`)
	if err != nil {
		return nil, fmt.Errorf("list layouts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	layouts := map[string]json.RawMessage{}
	for rows.Next() {
		var id, body string
		if err := rows.Scan(&id, &body); err != nil {
			return nil, fmt.Errorf("read layout row: %w", err)
		}
		layouts[id] = json.RawMessage(body)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"home": s.root, "projects": projects, "layouts": layouts}, nil
}
