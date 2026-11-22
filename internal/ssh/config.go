package ssh

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// ConfigHost is a host from ~/.ssh/config, ready to be saved.
type ConfigHost struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	KeyPath      string `json:"keyPath"`
	AgentForward bool   `json:"agentForward"`
	Skipped      string `json:"skipped,omitempty"`
}

const skipJump = "jump hosts come later"

// matchApplies evaluates a Match block for a host; unknown is true when it needs exec, canonical or final.
func matchApplies(b *block, name, usr, local string) (applies, unknown bool) {
	if len(b.args) == 0 {
		return false, false
	}
	applies = true
	for i := 0; i < len(b.args); i++ {
		crit := strings.ToLower(b.args[i])
		neg := strings.HasPrefix(crit, "!")
		crit = strings.TrimPrefix(crit, "!")
		var ok bool
		switch crit {
		case "all":
			ok = true
		case "exec", "canonical", "final":
			unknown = true
			if crit == "exec" {
				i++
			}
			continue
		case "host", "originalhost", "user", "localuser":
			if i+1 >= len(b.args) {
				return false, unknown
			}
			i++
			val := map[string]string{"host": name, "originalhost": name, "user": usr, "localuser": local}[crit]
			ok = patternList(strings.Split(b.args[i], ","), val)
		default:
			return false, unknown
		}
		if ok == neg {
			applies = false
		}
	}
	return applies, unknown
}

var relevant = []string{"hostname", "port", "user", "identityfile", "forwardagent", "proxyjump", "proxycommand"}

// ReadConfig lists the concrete Host entries of ~/.ssh/config. The file is only read.
func (m *Manager) ReadConfig() []ConfigHost {
	sshDir := filepath.Join(m.Home, ".ssh")
	var blocks []*block
	var cur *block
	readConfig(filepath.Join(sshDir, "config"), sshDir, 0, includeSet{}, &cur, &blocks)
	local := os.Getenv("USER")
	if u, err := user.Current(); err == nil {
		local = u.Username
	}
	out := []ConfigHost{}
	seen := map[string]bool{}
	for _, b := range blocks {
		if b.match {
			continue
		}
		for _, name := range b.args {
			if strings.ContainsAny(name, "*?!") || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, m.resolveConfigHost(blocks, name, local))
		}
	}
	return out
}

func (m *Manager) resolveConfigHost(blocks []*block, name, local string) ConfigHost {
	opts := map[string]string{}
	skipped := ""
	for _, b := range blocks {
		if b.match {
			usr := opts["user"]
			if usr == "" {
				usr = local
			}
			ok, unknown := matchApplies(b, name, usr, local)
			if unknown {
				if ok {
					for _, k := range relevant {
						if _, has := b.opts[k]; has {
							skipped = "uses Match exec, canonical or final, which Mooring does not run"
						}
					}
				}
				continue
			}
			if !ok {
				continue
			}
		} else if !patternList(b.args, name) {
			continue
		}
		for k, v := range b.opts {
			if _, has := opts[k]; !has {
				opts[k] = v
			}
		}
	}
	h := ConfigHost{Name: name, Address: name, Port: 22, User: local, Skipped: skipped}
	if v := opts["hostname"]; v != "" {
		h.Address = strings.ReplaceAll(v, "%h", name)
	}
	if p, err := strconv.Atoi(opts["port"]); err == nil && p > 0 && p < 65536 {
		h.Port = p
	}
	if v := opts["user"]; v != "" {
		h.User = v
	}
	h.AgentForward = strings.EqualFold(opts["forwardagent"], "yes")
	if v := opts["identityfile"]; v != "" {
		if v == "~" || strings.HasPrefix(v, "~/") {
			v = m.Home + v[1:]
		}
		if rel, err := filepath.Rel(m.Home, v); err == nil && !strings.HasPrefix(rel, "..") && filepath.IsAbs(v) {
			h.KeyPath = v
		}
	}
	if opts["proxyjump"] != "" || opts["proxycommand"] != "" {
		h.Skipped = skipJump
	}
	return h
}
