package chat

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
)

// Group sizes: a group is 2 or 3 chats of one project working on one task.
const (
	minGroup = 2
	maxGroup = 3
)

// groupTitle is the title of a group, "" when it has none.
func (m *Manager) groupTitle(id string) string {
	if id == "" {
		return ""
	}
	m.groupMu.Lock()
	defer m.groupMu.Unlock()
	return m.groupTitles[id]
}

func (m *Manager) loadGroups() error {
	rows, err := m.db.Query(`SELECT id, title FROM chat_groups`)
	if err != nil {
		return fmt.Errorf("list chat groups: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			return fmt.Errorf("read chat group: %w", err)
		}
		m.groupTitles[id] = title
	}
	return rows.Err()
}

func newGroupID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "g" + hex.EncodeToString(b)
}

// group makes a group of the chats in ids and returns its id.
func (m *Manager) group(ids []string, title string) (any, error) {
	if len(ids) < minGroup || len(ids) > maxGroup {
		return nil, apiErr("BAD_REQUEST", "A group needs %d or %d chats.", minGroup, maxGroup)
	}
	moves := map[string]string{}
	project := ""
	gid := newGroupID()
	for _, id := range ids {
		if _, dup := moves[id]; dup {
			return nil, apiErr("BAD_REQUEST", "A chat can be in a group only once.")
		}
		c, err := m.must(id)
		if err != nil {
			return nil, err
		}
		if project != "" && c.projectID != project {
			return nil, apiErr("BAD_REQUEST", "Chats in a group must be in the same project.")
		}
		project = c.projectID
		moves[id] = gid
	}
	if err := m.regroup(moves, nil, &groupRow{id: gid, project: project, title: shorten(title, 120)}); err != nil {
		return nil, err
	}
	return map[string]any{"groupId": gid}, nil
}

// ungroup takes one chat out of its group; a group left with one chat is dissolved.
func (m *Manager) ungroup(id string) (any, error) {
	c, err := m.must(id)
	if err != nil {
		return nil, err
	}
	if err := m.regroup(map[string]string{c.id: ""}, nil, nil); err != nil {
		return nil, err
	}
	return map[string]any{}, nil
}

type groupRow struct{ id, project, title string }

// regroup moves chats to groups (moves: chat id to group id, "" for none), then
// dissolves every group left with fewer than two chats. touched names groups to
// check that no move names, such as the group of a chat that was removed.
func (m *Manager) regroup(moves map[string]string, touched []string, add *groupRow) error {
	m.groupOp.Lock()
	defer m.groupOp.Unlock()
	m.mu.Lock()
	all := make([]*Chat, 0, len(m.chats))
	for _, c := range m.chats {
		all = append(all, c)
	}
	m.mu.Unlock()
	final := map[string]string{} // chat id to group id after the moves
	members := map[string][]*Chat{}
	for _, c := range all {
		c.mu.Lock()
		g := c.groupID
		c.mu.Unlock()
		if to, ok := moves[c.id]; ok {
			if g != to && g != "" {
				touched = append(touched, g)
			}
			g = to
		}
		final[c.id] = g
		if g != "" {
			members[g] = append(members[g], c)
		}
	}
	changed := map[string]string{}
	for id, to := range moves {
		changed[id] = to
	}
	var drop []string
	for _, g := range touched {
		if len(members[g]) < minGroup {
			drop = append(drop, g)
			for _, c := range members[g] {
				changed[c.id] = ""
			}
			delete(members, g)
		}
	}
	if err := m.saveGroups(changed, drop, add); err != nil {
		return err
	}
	byID := map[string]*Chat{}
	for _, c := range all {
		byID[c.id] = c
	}
	m.groupMu.Lock()
	for _, g := range drop {
		delete(m.groupTitles, g)
	}
	if add != nil {
		m.groupTitles[add.id] = add.title
	}
	m.groupMu.Unlock()
	ids := make([]string, 0, len(changed))
	for id := range changed {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		c := byID[id]
		if c == nil {
			continue
		}
		c.mu.Lock()
		same := c.groupID == changed[id]
		c.groupID = changed[id]
		c.mu.Unlock()
		if !same {
			c.publishInfo("chat.changed")
		}
	}
	return nil
}

func (m *Manager) saveGroups(changed map[string]string, drop []string, add *groupRow) error {
	tx, err := m.db.Begin()
	if err != nil {
		return m.groupFail(err)
	}
	defer func() { _ = tx.Rollback() }()
	if add != nil {
		if _, err := tx.Exec(`INSERT INTO chat_groups (id, project_id, title, created_at) VALUES (?, ?, ?, ?)`,
			add.id, add.project, add.title, m.now().UnixMilli()); err != nil {
			return m.groupFail(err)
		}
	}
	for id, g := range changed {
		var v any
		if g != "" {
			v = g
		}
		if _, err := tx.Exec(`UPDATE chats SET group_id = ? WHERE id = ?`, v, id); err != nil {
			return m.groupFail(err)
		}
	}
	for _, g := range drop {
		if _, err := tx.Exec(`DELETE FROM chat_groups WHERE id = ?`, g); err != nil {
			return m.groupFail(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return m.groupFail(err)
	}
	return nil
}

func (m *Manager) groupFail(err error) error {
	m.log.Error("chat: group save failed", "err", err)
	return apiErr("INTERNAL", "The group could not be saved.")
}
