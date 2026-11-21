package chat

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	ansiSeq    = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]|\x1b\\][^\x07\x1b]*(?:\x07|\x1b\\\\)|\x1b[@-Z\\\\-_]")
	authValue  = regexp.MustCompile(`(?i)(authorization[=: ]+)(?:bearer\s+)?\S+`)
	secretWord = regexp.MustCompile(`(?i)((?:key|token|secret|password|bearer)\s*[=:]\s*|(?:key|token|secret|password|bearer)\s+)\S+`)
	longRun    = regexp.MustCompile(`[A-Za-z0-9_\-+/=.]{24,}`)
)

// cleanLine makes one line of agent output safe to show and store: no escape codes or control
// characters, token-like values hidden, and at most n runes.
func cleanLine(s string, n int) string {
	s = ansiSeq.ReplaceAllString(s, "")
	s = strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = authValue.ReplaceAllString(s, "${1}[hidden]")
	s = secretWord.ReplaceAllString(s, "${1}[hidden]")
	s = longRun.ReplaceAllString(s, "[hidden]")
	if r := []rune(s); len(r) > n {
		s = string(r[:n]) + "..."
	}
	return strings.TrimSpace(s)
}
