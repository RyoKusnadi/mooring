// Package autostart installs and removes the login job that starts Mooring: a
// launchd agent on macOS, a systemd user unit on Linux. It never reads the real
// home folder, executable or environment; the caller passes them in Env.
package autostart

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Runner runs a program and returns what it printed. A non-zero exit is an error.
type Runner func(name string, args ...string) ([]byte, error)

// Env is everything the package needs from the machine.
type Env struct {
	GOOS          string
	Home          string
	XDGConfigHome string // empty: <Home>/.config
	UID           int
	Exe           string   // the installed binary, symlinks resolved
	Avoid         []string // folders the binary must not be in (temp folder, Go build cache)
	PortFree      func(addr string) bool
	Run           Runner
}

// Options are the flags the job starts Mooring with. Paths are absolute.
type Options struct {
	Addr        string
	Data        string
	Shell       string // program for terminals; empty: not passed
	SecretsPass string // passphrase file; empty: not passed
	TLSCert     string // certificate and key files for HTTPS; empty: plain HTTP
	TLSKey      string
	PathEnv     string // PATH of the current shell, so agents and tools are found
	ShellEnv    string // SHELL of the current shell
}

const (
	label       = "local.mooring"
	serviceName = "mooring.service"
)

// args is the command line the job runs.
func (o Options) args(exe string) []string {
	a := []string{exe, "-addr", o.Addr, "-data", o.Data}
	if o.Shell != "" {
		a = append(a, "-shell", o.Shell)
	}
	if o.SecretsPass != "" {
		a = append(a, "-secrets-passphrase-file", o.SecretsPass)
	}
	if o.TLSCert != "" {
		a = append(a, "-tls-cert", o.TLSCert, "-tls-key", o.TLSKey)
	}
	return append(a, "-quiet")
}

func (e Env) check() error {
	if e.Home == "" {
		return errors.New("autostart: the home folder is unknown")
	}
	if e.Exe == "" {
		return errors.New("autostart: the path of this program is unknown")
	}
	if e.GOOS != "darwin" && e.GOOS != "linux" {
		return fmt.Errorf("autostart is not available on %s", e.GOOS)
	}
	return nil
}

// path is where the job file lives.
func (e Env) path() string {
	if e.GOOS == "darwin" {
		return filepath.Join(e.Home, "Library", "LaunchAgents", label+".plist")
	}
	base := e.XDGConfigHome
	if base == "" {
		base = filepath.Join(e.Home, ".config")
	}
	return filepath.Join(base, "systemd", "user", serviceName)
}

// On writes the job file and starts the job. Running it again rewrites the file
// and restarts the job, which is how an upgraded binary takes over.
func On(e Env, o Options, w io.Writer) error {
	if err := e.check(); err != nil {
		return err
	}
	for _, dir := range e.Avoid {
		if dir != "" && within(dir, e.Exe) {
			return errors.New("Run autostart from the installed binary, not go run.") //nolint:staticcheck // shown to the user as written
		}
	}
	file := e.path()
	_, statErr := os.Stat(file)
	reinstall := statErr == nil
	if !reinstall && e.PortFree != nil && !e.PortFree(o.Addr) {
		return fmt.Errorf("%s is in use. Stop the Mooring that is running, then try again.", o.Addr) //nolint:staticcheck // shown to the user as written
	}
	for _, v := range append(o.args(e.Exe), o.PathEnv, o.ShellEnv) {
		if strings.ContainsAny(v, "\n\r\x00") {
			return errors.New("autostart: a path or setting contains a line break")
		}
	}
	var body string
	if e.GOOS == "darwin" {
		body = plist(e, o)
	} else {
		body = unit(e, o)
	}
	if err := os.MkdirAll(filepath.Join(o.Data, "logs"), 0o700); err != nil {
		return fmt.Errorf("create the log folder: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(file), err)
	}
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", file, err)
	}
	if err := os.Chmod(file, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", file, err)
	}
	if err := e.start(file); err != nil {
		return err
	}
	scheme := "http"
	if o.TLSCert != "" {
		scheme = "https"
	}
	fmt.Fprintf(w, "Autostart is on. Mooring starts at login and listens on %s://%s.\nFile: %s\n", scheme, o.Addr, file)
	if e.GOOS == "linux" {
		fmt.Fprintln(w, "A user unit starts when you log in, not at boot. Turning on lingering (loginctl enable-linger) changes that; Mooring does not do it.")
	}
	return nil
}

// Off stops the job and deletes its file. The data folder is never touched.
func Off(e Env, w io.Writer) error {
	if err := e.check(); err != nil {
		return err
	}
	file := e.path()
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(w, "Autostart is off.")
		return nil
	} else if err != nil {
		return fmt.Errorf("check %s: %w", file, err)
	}
	if err := e.stop(file); err != nil {
		return err
	}
	if err := os.Remove(file); err != nil {
		return fmt.Errorf("delete %s: %w", file, err)
	}
	if e.GOOS == "linux" {
		_, _ = e.Run("systemctl", "--user", "daemon-reload")
	}
	fmt.Fprintln(w, "Autostart is off. Your data folder was not touched.")
	return nil
}

// Status prints whether the job is installed, its command line and whether it runs.
func Status(e Env, w io.Writer) error {
	if err := e.check(); err != nil {
		return err
	}
	file := e.path()
	raw, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(w, "Autostart is off.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}
	var cmd []string
	if e.GOOS == "darwin" {
		cmd = parsePlist(string(raw))
	} else {
		cmd = parseUnit(string(raw))
	}
	fmt.Fprintf(w, "Autostart is on.\nFile: %s\nCommand: %s\n", file, strings.Join(cmd, " "))
	if e.running() {
		fmt.Fprintln(w, "Running: yes")
	} else {
		fmt.Fprintln(w, "Running: no")
	}
	if len(cmd) == 0 {
		fmt.Fprintln(w, "Warning: the file has no command Mooring can read. Run autostart on again.")
	} else if cmd[0] != e.Exe {
		fmt.Fprintf(w, "Warning: the file runs %s, not this binary (%s). Run autostart on to switch.\n", cmd[0], e.Exe)
	}
	return nil
}

// within reports whether path is dir or inside it.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (e Env) start(file string) error {
	if e.GOOS == "darwin" {
		return e.startLaunchd(file)
	}
	return e.startSystemd()
}
