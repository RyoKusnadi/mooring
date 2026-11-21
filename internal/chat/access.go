package chat

import (
	"context"
	"encoding/json"
	"sort"
)

// OpenPermission is a permission request that still waits for an answer.
type OpenPermission struct {
	RequestID string
	Title     string
	Paths     []string // files the tool names; relative ones are relative to the chat folder
	Options   []Option
}

// Info returns the current state of a chat.
func (m *Manager) Info(id string) (Info, error) {
	c, err := m.must(id)
	if err != nil {
		return Info{}, err
	}
	return c.Info(), nil
}

// ChatDir returns the folder a chat works in.
func (m *Manager) ChatDir(id string) (string, error) {
	c, err := m.must(id)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dir, nil
}

// OpenPermissions lists the permission requests of a chat that wait for an answer,
// oldest first. The bench answers them without a browser.
func (m *Manager) OpenPermissions(id string) ([]OpenPermission, error) {
	c, err := m.must(id)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	idx := make([]int, 0, len(c.perms))
	for _, pp := range c.perms {
		idx = append(idx, pp.msg)
	}
	sort.Ints(idx)
	out := make([]OpenPermission, 0, len(idx))
	for _, i := range idx {
		msg := c.msgs[i]
		var locs []struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(msg.Locations, &locs)
		op := OpenPermission{RequestID: msg.RequestID, Title: msg.Title, Options: append([]Option{}, msg.Options...)}
		for _, l := range locs {
			if l.Path != "" {
				op.Paths = append(op.Paths, l.Path)
			}
		}
		out = append(out, op)
	}
	return out, nil
}

// WaitJobs waits until the title jobs started so far have ended, or ctx is done.
func (m *Manager) WaitJobs(ctx context.Context) error {
	done := make(chan struct{})
	go func() { m.jobsWG.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
