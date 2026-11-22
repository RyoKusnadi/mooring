package fakelsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func capabilities() map[string]any {
	return map[string]any{
		"textDocumentSync":           map[string]any{"openClose": true, "change": 2},
		"completionProvider":         map[string]any{},
		"hoverProvider":              true,
		"documentFormattingProvider": true,
		"documentSymbolProvider":     true,
		"referencesProvider":         true,
		"renameProvider":             true,
	}
}

// refactor answers references and rename the simple way: every whole-word match of the
// name under the cursor, in the files of the same folder (open ones as the editor has them).
func (s *server) refactor(method string, params json.RawMessage) any {
	var p struct {
		TextDocument textDoc
		Position     position
		NewName      string `json:"newName"`
	}
	if json.Unmarshal(params, &p) != nil {
		return nil
	}
	name := wordAt(s.text(p.TextDocument.URI), p.Position)
	if name == "" {
		return nil
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	var locs []map[string]any
	changes := map[string][]map[string]any{}
	for _, uri := range s.folderURIs(p.TextDocument.URI) {
		for n, line := range strings.Split(s.text(uri), "\n") {
			for _, m := range re.FindAllStringIndex(line, -1) {
				r := map[string]any{"start": map[string]int{"line": n, "character": m[0]}, "end": map[string]int{"line": n, "character": m[1]}}
				locs = append(locs, map[string]any{"uri": uri, "range": r})
				changes[uri] = append(changes[uri], map[string]any{"range": r, "newText": p.NewName})
			}
		}
	}
	if method == "textDocument/rename" {
		return map[string]any{"changes": changes}
	}
	return locs
}

func (s *server) text(uri string) string {
	if t, ok := s.docs[uri]; ok {
		return t
	}
	data, _ := os.ReadFile(strings.TrimPrefix(uri, "file://"))
	return string(data)
}

func (s *server) folderURIs(uri string) []string {
	dir := filepath.Dir(strings.TrimPrefix(uri, "file://"))
	set := map[string]bool{}
	for u := range s.docs {
		if filepath.Dir(strings.TrimPrefix(u, "file://")) == dir {
			set[u] = true
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			set["file://"+filepath.Join(dir, e.Name())] = true
		}
	}
	out := make([]string, 0, len(set))
	for u := range set {
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}

var wordChars = regexp.MustCompile(`[A-Za-z0-9_]+`)

func wordAt(text string, p position) string {
	lines := strings.Split(text, "\n")
	if p.Line >= len(lines) {
		return ""
	}
	for _, m := range wordChars.FindAllStringIndex(lines[p.Line], -1) {
		if m[0] <= p.Character && p.Character <= m[1] {
			return lines[p.Line][m[0]:m[1]]
		}
	}
	return ""
}
