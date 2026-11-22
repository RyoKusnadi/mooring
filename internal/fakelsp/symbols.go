package fakelsp

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Document symbols come from lines that start (after any indent) with "func", "type" or "class"
// and a name; a symbol ends before the next line indented no deeper, so a more indented one is its
// child. A file named .fakelsp-slow-init in the folder the server starts in makes it answer
// initialize late: after the duration written in the file (Go syntax, 4s when empty or unreadable).
var symbolLine = regexp.MustCompile(`^(\s*)(?:func|type|class)\s+(\w+)`)

type symbol struct {
	Name           string         `json:"name"`
	Kind           int            `json:"kind"`
	Range          map[string]any `json:"range"`
	SelectionRange map[string]any `json:"selectionRange"`
	Children       []*symbol      `json:"children,omitempty"`
}

func pos(line, character int) map[string]int {
	return map[string]int{"line": line, "character": character}
}

// symbols turns the declaration lines of text into nested document symbols (kind 12, function).
func symbols(text string) []*symbol {
	lines := strings.Split(text, "\n")
	indent := func(l string) int { return len(l) - len(strings.TrimLeft(l, " \t")) }
	top := []*symbol{}
	type open struct {
		sym    *symbol
		indent int
	}
	var stack []open
	closeTo := func(end, level int) {
		for len(stack) > 0 && stack[len(stack)-1].indent >= level {
			o := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			last := end
			for last > 0 && strings.TrimSpace(lines[last]) == "" {
				last--
			}
			o.sym.Range["end"] = pos(last, len(utf16(lines[last])))
		}
	}
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		// A closing brace belongs to the symbol it closes.
		level := indent(l)
		if strings.HasPrefix(strings.TrimSpace(l), "}") {
			level++
		}
		closeTo(i-1, level)
		m := symbolLine.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		col := strings.Index(l, m[2])
		sym := &symbol{
			Name:           m[2],
			Kind:           12,
			Range:          map[string]any{"start": pos(i, len(m[1])), "end": pos(i, len(utf16(l)))},
			SelectionRange: map[string]any{"start": pos(i, col), "end": pos(i, col+len(m[2]))},
		}
		if len(stack) > 0 {
			parent := stack[len(stack)-1].sym
			parent.Children = append(parent.Children, sym)
		} else {
			top = append(top, sym)
		}
		stack = append(stack, open{sym, indent(l)})
	}
	closeTo(len(lines)-1, 0)
	return top
}

// initDelay is how long initialize waits: see the package comment.
func initDelay(params json.RawMessage) time.Duration {
	var p struct {
		RootURI string `json:"rootUri"`
	}
	if json.Unmarshal(params, &p) != nil {
		return 0
	}
	u, err := url.Parse(p.RootURI)
	if err != nil || u.Path == "" {
		return 0
	}
	raw, err := os.ReadFile(filepath.Join(u.Path, ".fakelsp-slow-init"))
	if err != nil {
		return 0
	}
	if d, err := time.ParseDuration(strings.TrimSpace(string(raw))); err == nil && d > 0 {
		return d
	}
	return 4 * time.Second
}
