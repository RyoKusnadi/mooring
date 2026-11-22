package lsp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// dataBinDirs are the two folders of Mooring's own installs.
func (s *Service) dataBinDirs() []string {
	if s.cfg.DataDir == "" {
		return nil
	}
	root := filepath.Join(s.cfg.DataDir, "lsp")
	return []string{filepath.Join(root, "bin"), filepath.Join(root, "node_modules", ".bin")}
}

// pathDirs is the PATH the servers and installers see: Mooring's folders first.
func (s *Service) pathDirs() []string {
	dirs := s.cfg.PathDirs
	if dirs == nil {
		dirs = filepath.SplitList(os.Getenv("PATH"))
	}
	return append(s.dataBinDirs(), dirs...)
}

// searchDirs is where a server program is looked for, in order: Mooring's own
// folder, the PATH, then $(go env GOPATH)/bin and ~/.cargo/bin.
func (s *Service) searchDirs() []string {
	dirs := s.pathDirs()
	if s.cfg.ExtraDirs != nil {
		return append(dirs, s.cfg.ExtraDirs...)
	}
	return append(dirs, s.defaultExtra()...)
}

func (s *Service) defaultExtra() []string {
	s.extraOnce.Do(func() {
		if gp := goPath(); gp != "" {
			s.extra = append(s.extra, filepath.Join(gp, "bin"))
		}
		if home, err := os.UserHomeDir(); err == nil {
			s.extra = append(s.extra, filepath.Join(home, ".cargo", "bin"))
		}
	})
	return s.extra
}

func goPath() string {
	if v := os.Getenv("GOPATH"); v != "" {
		return filepath.SplitList(v)[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "go", "env", "GOPATH").Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return filepath.SplitList(v)[0]
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "go")
	}
	return ""
}

func isExecutable(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// find returns the full path of a program: a path with a slash is used as it is,
// a bare name is looked for in dirs.
func find(name string, dirs []string) (string, bool) {
	if name == "" {
		return "", false
	}
	if strings.ContainsRune(name, filepath.Separator) {
		abs, err := filepath.Abs(name)
		return abs, err == nil && isExecutable(abs)
	}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if p := filepath.Join(d, name); isExecutable(p) {
			return p, true
		}
	}
	return "", false
}

// tsserverPath finds lib/tsserver.js of the typescript package next to the
// server: in the node_modules that holds it (the .bin link is resolved first).
func tsserverPath(server string) string {
	var starts []string
	starts = append(starts, filepath.Dir(server))
	if real, err := filepath.EvalSymlinks(server); err == nil {
		starts = append(starts, filepath.Dir(real))
	}
	for _, dir := range starts {
		for d := dir; ; d = filepath.Dir(d) {
			var cand string
			if filepath.Base(d) == "node_modules" {
				cand = filepath.Join(d, "typescript", "lib", "tsserver.js")
			} else {
				cand = filepath.Join(d, "node_modules", "typescript", "lib", "tsserver.js")
			}
			if _, err := os.Stat(cand); err == nil {
				return cand
			}
			if filepath.Dir(d) == d {
				break
			}
		}
	}
	return ""
}
