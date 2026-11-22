package server

import (
	"github.com/ryokusnadi/mooring/internal/chat"
	"github.com/ryokusnadi/mooring/internal/git"
)

// chatFolders lets the git service ask which chats work in which folders.
type chatFolders struct{ m *chat.Manager }

func (c chatFolders) FolderUse(projectID string) map[string]git.ChatUse {
	out := map[string]git.ChatUse{}
	for _, u := range c.m.FolderUse(projectID) {
		if prev, ok := out[u.Dir]; ok && prev.Live && !u.Live {
			continue
		}
		out[u.Dir] = git.ChatUse{ID: u.ID, Live: u.Live}
	}
	return out
}
