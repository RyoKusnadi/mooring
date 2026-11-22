package chat

import (
	"context"
	"path/filepath"
	"strings"
	"time"
)

// Limits on the changes a second opinion starts from, in bytes.
const (
	maxChangesStat = 4 << 10
	maxChangesDiff = 24 << 10
	changesWait    = 30 * time.Second
	noChanges      = "No uncommitted changes."
)

// changesOf is the text of the uncommitted changes in the folder of chat id, for
// a fresh chat asked to review its work. A long stat or diff is cut and says so.
func (m *Manager) changesOf(id string) (string, error) {
	src := m.get(id)
	if src == nil {
		return "", apiErr("NOT_FOUND", "The chat to review does not exist.")
	}
	if m.Uncommitted == nil {
		return "", apiErr("INTERNAL", "This server cannot read git changes.")
	}
	rel, err := filepath.Rel(src.projectDir, src.dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", apiErr("BAD_REQUEST", "That chat works outside its project folder.")
	}
	ctx, cancel := context.WithTimeout(src.baseContext(), changesWait)
	defer cancel()
	stat, diff, err := m.Uncommitted(ctx, src.projectID, filepath.ToSlash(rel))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(stat) == "" {
		return noChanges, nil
	}
	return "Files:\n" + cut(stat, maxChangesStat) + "\nDiff:\n" + cut(diff, maxChangesDiff), nil
}

// cut caps s like capText and adds a line that says it was cut.
func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return capText(s, n) + "\n[cut here: the rest is longer than Mooring sends]\n"
}
