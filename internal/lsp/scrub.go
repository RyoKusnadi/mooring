package lsp

import "regexp"

const secretName = `(?:token|password|passwd|secret|api[_-]?key|access[_-]?key|private[_-]?key|credential)`

var (
	urlCreds = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/@\s]+@`)
	bearer   = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]{6,}`)
	// Whole header values: Cookie and Authorization lines, quoted or not.
	header = regexp.MustCompile(`(?i)(["']?\b(?:set-)?(?:cookie|authorization|proxy-authorization)["']?\s*[=:]\s*)("[^"]*"|'[^']*'|[^\r\n]+)`)
	// name=value, name: value and "name": "value".
	secretKV = regexp.MustCompile(`(?i)(["']?\b[a-z0-9_.-]*` + secretName + `[a-z0-9_.-]*["']?\s*[=:]\s*)("[^"]*"|'[^']*'|[^\s,;}]+)`)
	// --token abc, -password abc.
	secretFlag = regexp.MustCompile(`(?i)(\B--?[a-z0-9_.-]*` + secretName + `[a-z0-9_.-]*\s+)("[^"]*"|'[^']*'|[^\s-][^\s]*)`)
)

// scrub removes credentials from text that may be shown or logged: user info
// in URLs, bearer tokens, whole Cookie and Authorization values, and values of
// keys and flags such as password or token.
func scrub(s string) string {
	s = urlCreds.ReplaceAllString(s, "$1")
	s = header.ReplaceAllString(s, "$1[removed]")
	s = bearer.ReplaceAllString(s, "$1 [removed]")
	s = secretKV.ReplaceAllString(s, "$1[removed]")
	return secretFlag.ReplaceAllString(s, "$1[removed]")
}
