package fakelsp

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
)

type position struct{ Line, Character int }

func (s *server) didChange(params json.RawMessage) {
	var p struct {
		TextDocument textDoc
		Changes      []struct {
			Range *struct{ Start, End position }
			Text  string
		} `json:"contentChanges"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	text := s.docs[p.TextDocument.URI]
	for _, c := range p.Changes {
		if c.Range == nil {
			text = c.Text
			continue
		}
		start := offset(text, c.Range.Start)
		end := offset(text, c.Range.End)
		text = text[:start] + c.Text + text[end:]
	}
	s.docs[p.TextDocument.URI] = text
	s.changed(p.TextDocument.URI, text, true)
}

// offset turns a line and character (counted in bytes; the tests use ASCII) into an index.
func offset(text string, p position) int {
	i := 0
	for line := 0; line < p.Line; line++ {
		n := strings.IndexByte(text[i:], '\n')
		if n < 0 {
			return len(text)
		}
		i += n + 1
	}
	return min(i+p.Character, len(text))
}

func (s *server) watched(params json.RawMessage) {
	var p struct{ Changes []struct{ URI string } }
	if json.Unmarshal(params, &p) != nil {
		return
	}
	for _, c := range p.Changes {
		path := strings.TrimPrefix(c.URI, "file://")
		delete(s.broken, path)
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), "BREAKS_OTHERS") {
			s.broken[path] = path[strings.LastIndexByte(path, '/')+1:]
		}
	}
	for uri := range s.docs {
		s.publish(uri)
	}
}

func dirOf(path string) string { return path[:strings.LastIndexByte(path, '/')+1] }

func (s *server) publish(uri string) {
	diags := []map[string]any{}
	for n, line := range strings.Split(s.docs[uri], "\n") {
		if strings.Contains(line, "ERROR") {
			diags = append(diags, diag(n, len(line), "found ERROR"))
		}
	}
	dir := dirOf(strings.TrimPrefix(uri, "file://"))
	names := make([]string, 0, len(s.broken))
	for path, name := range s.broken {
		if dirOf(path) == dir {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		diags = append(diags, diag(0, 1, "broken by "+name))
	}
	s.notify("textDocument/publishDiagnostics", map[string]any{"uri": uri, "diagnostics": diags})
}

func diag(line, width int, message string) map[string]any {
	return map[string]any{
		"range":    map[string]any{"start": map[string]int{"line": line, "character": 0}, "end": map[string]int{"line": line, "character": width}},
		"severity": 1, "source": "fakelsp", "message": message,
	}
}

func (s *server) completions() []map[string]any {
	seen := map[string]bool{"fakeCompletion": true}
	for _, text := range s.docs {
		for _, w := range word.FindAllString(text, -1) {
			seen[w] = true
		}
	}
	labels := make([]string, 0, len(seen))
	for w := range seen {
		labels = append(labels, w)
	}
	sort.Strings(labels)
	items := make([]map[string]any, 0, len(labels))
	for _, w := range labels {
		items = append(items, map[string]any{"label": w})
	}
	return items
}
