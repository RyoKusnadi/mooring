package autostart

import (
	"fmt"
	"strings"
)

// quote wraps s for a systemd unit line: backslash and quote are escaped, and
// % and $ doubled so neither is expanded.
func quote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%", "$", "$$")
	return `"` + r.Replace(s) + `"`
}

// quoteEnv is quote for Environment= lines, where $ is not special.
func quoteEnv(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%").Replace(s) + `"`
}

func unquote(s string) string {
	r := strings.NewReplacer(`\\`, `\`, `\"`, `"`, "%%", "%", "$$", "$")
	return r.Replace(s)
}

func unit(e Env, o Options) string {
	var parts []string
	for _, a := range o.args(e.Exe) {
		parts = append(parts, quote(a))
	}
	var b strings.Builder
	b.WriteString("[Unit]\nDescription=Mooring\n\n[Service]\n")
	fmt.Fprintf(&b, "ExecStart=%s\n", strings.Join(parts, " "))
	if o.PathEnv != "" {
		fmt.Fprintf(&b, "Environment=%s\n", quoteEnv("PATH="+o.PathEnv))
	}
	if o.ShellEnv != "" {
		fmt.Fprintf(&b, "Environment=%s\n", quoteEnv("SHELL="+o.ShellEnv))
	}
	b.WriteString("Restart=on-failure\nRestartSec=5\n\n[Install]\nWantedBy=default.target\n")
	return b.String()
}

// parseUnit returns the ExecStart words.
func parseUnit(text string) []string {
	for _, line := range strings.Split(text, "\n") {
		rest, ok := strings.CutPrefix(line, "ExecStart=")
		if !ok || len(rest) < 2 {
			continue
		}
		var out []string
		for _, w := range strings.Split(rest[1:len(rest)-1], `" "`) {
			out = append(out, unquote(w))
		}
		return out
	}
	return nil
}

func (e Env) startSystemd() error {
	for _, args := range [][]string{{"daemon-reload"}, {"enable", serviceName}, {"restart", serviceName}} {
		if out, err := e.Run("systemctl", append([]string{"--user"}, args...)...); err != nil {
			return fmt.Errorf("systemctl --user %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func (e Env) stop(_ string) error {
	if e.GOOS == "darwin" {
		_, _ = e.Run("launchctl", "bootout", e.domain()+"/"+label)
		return nil
	}
	// A unit that is not running or enabled is fine; the file goes either way.
	_, _ = e.Run("systemctl", "--user", "disable", "--now", serviceName)
	return nil
}

func (e Env) running() bool {
	if e.GOOS == "darwin" {
		out, err := e.Run("launchctl", "print", e.domain()+"/"+label)
		return err == nil && strings.Contains(string(out), "state = running")
	}
	out, _ := e.Run("systemctl", "--user", "is-active", serviceName)
	return strings.TrimSpace(string(out)) == "active"
}
