package ssh

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxIncludeDepth = 16
	maxIncludeFiles = 256
)

// includeSet remembers the files read, so repeated or mutual includes are read once.
type includeSet map[string]bool

// block is one Host or Match section of an ssh config file.
type block struct {
	match bool
	args  []string          // Host patterns, or Match criteria words
	opts  map[string]string // first value of each keyword, lower-case
}

// splitWords splits a config line value into words, honouring double quotes.
func splitWords(s string) []string {
	var out []string
	var cur strings.Builder
	inQ, has := false, false
	for _, r := range s {
		switch {
		case r == '"':
			inQ, has = !inQ, true
		case (r == ' ' || r == '\t') && !inQ:
			if has || cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if has || cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// splitLine returns the keyword (lower-case) and the rest of a line.
func splitLine(line string) (key, rest string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || line[0] == '#' {
		return "", "", false
	}
	i := strings.IndexAny(line, " \t=")
	if i < 0 {
		return strings.ToLower(line), "", true
	}
	key = strings.ToLower(line[:i])
	rest = strings.TrimLeft(line[i:], " \t")
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "="))
	return key, rest, true
}

// readConfig reads path and the files it includes into blocks, in order.
// Includes are relative to sshDir; at most maxIncludeDepth deep.
func readConfig(path, sshDir string, depth int, seen includeSet, cur **block, out *[]*block) {
	path = filepath.Clean(path)
	if seen[path] || len(seen) >= maxIncludeFiles {
		return
	}
	seen[path] = true
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, rest, ok := splitLine(sc.Text())
		if !ok {
			continue
		}
		switch key {
		case "include":
			if depth >= maxIncludeDepth {
				continue
			}
			for _, pat := range splitWords(rest) {
				if strings.HasPrefix(pat, "~/") {
					pat = filepath.Join(filepath.Dir(sshDir), pat[2:])
				} else if !filepath.IsAbs(pat) {
					pat = filepath.Join(sshDir, pat)
				}
				matches, _ := filepath.Glob(pat)
				for _, m := range matches {
					readConfig(m, sshDir, depth+1, seen, cur, out)
				}
			}
		case "host", "match":
			*cur = &block{match: key == "match", args: splitWords(rest), opts: map[string]string{}}
			*out = append(*out, *cur)
		default:
			if *cur == nil {
				*cur = &block{args: []string{"*"}, opts: map[string]string{}}
				*out = append(*out, *cur)
			}
			if _, seen := (*cur).opts[key]; !seen {
				if w := splitWords(rest); len(w) > 0 {
					(*cur).opts[key] = w[0]
				}
			}
		}
	}
}

// globMatch matches an ssh pattern with * and ?.
func globMatch(pat, s string) bool {
	if pat == "" {
		return s == ""
	}
	switch pat[0] {
	case '*':
		for i := 0; i <= len(s); i++ {
			if globMatch(pat[1:], s[i:]) {
				return true
			}
		}
		return false
	case '?':
		return s != "" && globMatch(pat[1:], s[1:])
	}
	return s != "" && pat[0] == s[0] && globMatch(pat[1:], s[1:])
}

// patternList matches s against a list with ! negations: a negated hit rejects, else any hit accepts.
func patternList(pats []string, s string) bool {
	hit := false
	for _, p := range pats {
		if strings.HasPrefix(p, "!") {
			if globMatch(p[1:], s) {
				return false
			}
		} else if globMatch(p, s) {
			hit = true
		}
	}
	return hit
}
