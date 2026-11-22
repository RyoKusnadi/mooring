package files

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ryokusnadi/mooring/internal/paths"
)

// alwaysIgnored are folders marked ignored in every project.
var alwaysIgnored = map[string]bool{
	"node_modules": true, ".git": true, ".next": true, ".cache": true, ".turbo": true, "dist": true,
	"build": true, ".output": true, ".nuxt": true, ".svelte-kit": true, "__pycache__": true,
	".pytest_cache": true, "target": true,
}

// rule is one line of a .gitignore (or a search glob).
type rule struct {
	segs    []string
	neg     bool
	dirOnly bool
	anchor  bool
}

func parseRule(line string) (rule, bool) {
	line = strings.TrimRight(line, " \t\r")
	if line == "" || strings.HasPrefix(line, "#") {
		return rule{}, false
	}
	var r rule
	if strings.HasPrefix(line, "!") {
		r.neg, line = true, line[1:]
	} else if strings.HasPrefix(line, `\`) {
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		r.dirOnly, line = true, strings.TrimRight(line, "/")
	}
	if line == "" {
		return rule{}, false
	}
	r.anchor = strings.Contains(line, "/")
	line = strings.TrimPrefix(line, "/")
	r.segs = strings.Split(line, "/")
	return r, true
}

// match reports whether rel (slash form, relative to the rule's folder) matches.
func (r rule) match(rel string, isDir bool) bool {
	if r.dirOnly && !isDir {
		return false
	}
	parts := strings.Split(rel, "/")
	if !r.anchor {
		return segMatch(r.segs, parts[len(parts)-1:])
	}
	return segMatch(r.segs, parts)
}

// segMatch matches pattern segments against path segments; "**" spans any number.
func segMatch(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for len(pat) > 1 && pat[1] == "**" {
				pat = pat[1:]
			}
			if len(pat) == 1 {
				return len(name) > 0
			}
			for i := 0; i <= len(name); i++ {
				if segMatch(pat[1:], name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if ok, err := path.Match(pat[0], name[0]); err != nil || !ok {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}

type ruleFile struct {
	base  string // folder of the .gitignore, relative to the project, "" for the root
	rules []rule
}

// ignoreSet is the chain of .gitignore files from the project folder down.
// It is never changed after it is built, so a copy per folder is cheap.
type ignoreSet struct {
	files []ruleFile
}

// child adds the .gitignore of folder rel, read through the path check.
func (s ignoreSet) child(root, rel string) ignoreSet {
	real, err := paths.Resolve(root, filepath.FromSlash(path.Join(rel, ".gitignore")))
	if err != nil {
		return s
	}
	f, err := os.Open(real)
	if err != nil {
		return s
	}
	defer func() { _ = f.Close() }()
	var rules []rule
	sc := bufio.NewScanner(limitReader(f))
	for sc.Scan() && len(rules) < 5000 {
		if r, ok := parseRule(sc.Text()); ok {
			rules = append(rules, r)
		}
	}
	if len(rules) == 0 {
		return s
	}
	out := ignoreSet{files: make([]ruleFile, len(s.files), len(s.files)+1)}
	copy(out.files, s.files)
	out.files = append(out.files, ruleFile{base: rel, rules: rules})
	return out
}

// ignoreFor builds the chain for folder rel: the root's file down to rel's own.
func ignoreFor(root, rel string) ignoreSet {
	set := ignoreSet{}.child(root, "")
	cur := ""
	if rel == "" {
		return set
	}
	for _, p := range strings.Split(rel, "/") {
		cur = path.Join(cur, p)
		set = set.child(root, cur)
	}
	return set
}

// ignored reports whether the entry rel (not its parents) is ignored.
func (s ignoreSet) ignored(rel string, isDir bool) bool {
	if alwaysIgnored[path.Base(rel)] && isDir {
		return true
	}
	out := false
	for _, f := range s.files {
		sub := rel
		if f.base != "" {
			if !strings.HasPrefix(rel, f.base+"/") {
				continue
			}
			sub = rel[len(f.base)+1:]
		}
		for _, r := range f.rules {
			if r.match(sub, isDir) {
				out = !r.neg
			}
		}
	}
	return out
}

// anyIgnored reports whether rel or one of its parent folders is ignored.
func (s ignoreSet) anyIgnored(rel string, isDir bool) bool {
	parts := strings.Split(rel, "/")
	for i := range parts {
		p := strings.Join(parts[:i+1], "/")
		if s.ignored(p, i < len(parts)-1 || isDir) {
			return true
		}
	}
	return false
}
