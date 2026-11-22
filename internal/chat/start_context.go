package chat

import "strings"

// checkContext validates the summary a fresh chat starts from: at most 16 KB, and
// fromChat, when given, names a chat and comes with the text. With withChanges it
// also returns that chat's uncommitted changes, read now.
func (m *Manager) checkContext(a args) (changes string, err error) {
	if len(a.Context) > maxContextBytes {
		return "", apiErr("BAD_REQUEST", "The summary is over %d KB.", maxContextBytes>>10)
	}
	if strings.TrimSpace(a.Context) == "" && a.FromChat != "" {
		return "", apiErr("BAD_REQUEST", "A chat started from another chat needs the summary text.")
	}
	if a.FromChat != "" && (!idPattern.MatchString(a.FromChat) || m.get(a.FromChat) == nil) {
		return "", apiErr("NOT_FOUND", "The chat to continue from does not exist.")
	}
	if !a.WithChanges {
		return "", nil
	}
	if a.FromChat == "" {
		return "", apiErr("BAD_REQUEST", "Changes to review come from another chat. Start from that chat.")
	}
	return m.changesOf(a.FromChat)
}
