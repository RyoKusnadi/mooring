package autostart

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fake records every command and answers from a table; it never runs anything.
type fake struct {
	calls   []string
	answers map[string]string
	fail    map[string]bool
}

func (f *fake) run(name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if f.fail[call] {
		return []byte("failed"), errors.New("exit status 1")
	}
	return []byte(f.answers[call]), nil
}

const tricky = `/opt/My Apps/mooring`

func setup(t *testing.T, goos string) (Env, Options, *fake) {
	t.Helper()
	home := t.TempDir()
	data := filepath.Join(home, "My Data & Stuff")
	f := &fake{answers: map[string]string{}, fail: map[string]bool{}}
	e := Env{GOOS: goos, Home: home, UID: 501, Exe: tricky, Run: f.run, PortFree: func(string) bool { return true }}
	o := Options{Addr: "127.0.0.1:7340", Data: data, Shell: "/bin/zsh", SecretsPass: "/keys/pass<1>.txt",
		PathEnv: "/usr/bin:/opt/a b:/x/50%$HOME\"q\"", ShellEnv: "/bin/zsh"}
	return e, o, f
}

func golden(t *testing.T, name, got string, e Env, o Options) {
	t.Helper()
	// The test home is random; the golden file uses a fixed one.
	got = strings.ReplaceAll(got, e.Home, "/home/test")
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("%s differs:\n%s", name, got)
	}
}

func TestPlistMatchesGolden(t *testing.T) {
	e, o, _ := setup(t, "darwin")
	golden(t, "mooring.plist", plist(e, o), e, o)
	if e.GOOS == runtime.GOOS {
		if _, err := exec.LookPath("plutil"); err == nil {
			file := filepath.Join(t.TempDir(), "x.plist")
			if err := os.WriteFile(file, []byte(plist(e, o)), 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command("plutil", "-lint", file).CombinedOutput(); err != nil {
				t.Fatalf("plutil: %v %s", err, out)
			}
		}
	}
}

func TestUnitMatchesGolden(t *testing.T) {
	e, o, _ := setup(t, "linux")
	golden(t, "mooring.service", unit(e, o), e, o)
}

func TestParsingGivesBackTheCommand(t *testing.T) {
	e, o, _ := setup(t, "darwin")
	want := o.args(e.Exe)
	for name, got := range map[string][]string{"plist": parsePlist(plist(e, o)), "unit": parseUnit(unit(e, o))} {
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("%s: %q, want %q", name, got, want)
		}
	}
}

func TestTLSFilesArePassedOn(t *testing.T) {
	e, o, _ := setup(t, "darwin")
	o.TLSCert, o.TLSKey = "/certs/box.pem", "/certs/box key.pem"
	got := strings.Join(o.args(e.Exe), "\x00")
	if !strings.Contains(got, "-tls-cert\x00/certs/box.pem\x00-tls-key\x00/certs/box key.pem") {
		t.Fatalf("args %q", got)
	}
	if back := parsePlist(plist(e, o)); strings.Join(back, "\x00") != got {
		t.Fatalf("plist gives %q", back)
	}
}

func TestMacOSOnOnAgainOffOffAgain(t *testing.T) {
	e, o, f := setup(t, "darwin")
	var out bytes.Buffer
	plistFile := filepath.Join(e.Home, "Library", "LaunchAgents", "local.mooring.plist")
	if err := On(e, o, &out); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(plistFile)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("plist: %v %v", info, err)
	}
	// The port is held by the job itself the second time; that must not refuse.
	e.PortFree = func(string) bool { return false }
	if err := On(e, o, &out); err != nil {
		t.Fatal(err)
	}
	if err := Off(e, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plistFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("plist remains: %v", err)
	}
	out.Reset()
	if err := Off(e, &out); err != nil || !strings.Contains(out.String(), "Autostart is off.") {
		t.Fatalf("second off: %v %q", err, out.String())
	}
	if _, err := os.Stat(o.Data); err != nil {
		t.Fatalf("the data folder must stay: %v", err)
	}
	dom := "gui/501/local.mooring"
	want := []string{
		"launchctl bootout " + dom, "launchctl bootstrap gui/501 " + plistFile,
		"launchctl bootout " + dom, "launchctl bootstrap gui/501 " + plistFile,
		"launchctl bootout " + dom,
	}
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls:\n%s", strings.Join(f.calls, "\n"))
	}
}

func TestLinuxOnOnAgainOffOffAgain(t *testing.T) {
	e, o, f := setup(t, "linux")
	e.XDGConfigHome = filepath.Join(e.Home, "xdg")
	var out bytes.Buffer
	unitFile := filepath.Join(e.XDGConfigHome, "systemd", "user", "mooring.service")
	if err := On(e, o, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "not at boot") {
		t.Fatalf("output: %q", out.String())
	}
	if err := On(e, o, &out); err != nil {
		t.Fatal(err)
	}
	if err := Off(e, &out); err != nil {
		t.Fatal(err)
	}
	if err := Off(e, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unitFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unit remains: %v", err)
	}
	on := []string{"systemctl --user daemon-reload", "systemctl --user enable mooring.service", "systemctl --user restart mooring.service"}
	want := append(append(append([]string{}, on...), on...),
		"systemctl --user disable --now mooring.service", "systemctl --user daemon-reload")
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls:\n%s", strings.Join(f.calls, "\n"))
	}
}

func TestLinuxDefaultsToDotConfig(t *testing.T) {
	e, o, _ := setup(t, "linux")
	if err := On(e, o, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.Home, ".config", "systemd", "user", "mooring.service")); err != nil {
		t.Fatal(err)
	}
}

func TestRefusalsWriteAndRunNothing(t *testing.T) {
	cases := map[string]func(*Env){
		"Run autostart from the installed binary": func(e *Env) {
			e.Avoid = []string{"/var/folders/xx/T"}
			e.Exe = "/var/folders/xx/T/go-build123/b001/exe/mooring"
		},
		"127.0.0.1:7340 is in use": func(e *Env) { e.PortFree = func(string) bool { return false } },
		"home folder is unknown":   func(e *Env) { e.Home = "" },
		"path of this program":     func(e *Env) { e.Exe = "" },
		"not available on windows": func(e *Env) { e.GOOS = "windows" },
	}
	for want, change := range cases {
		e, o, f := setup(t, "darwin")
		home := e.Home
		change(&e)
		err := On(e, o, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%q: err = %v", want, err)
		}
		if len(f.calls) != 0 {
			t.Fatalf("%q: ran %v", want, f.calls)
		}
		left, _ := os.ReadDir(home)
		if len(left) != 0 {
			t.Fatalf("%q: wrote %v", want, left)
		}
	}
}

func TestFailedStartIsReported(t *testing.T) {
	e, o, f := setup(t, "darwin")
	f.fail["launchctl bootstrap gui/501 "+e.path()] = true
	err := On(e, o, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "launchctl") {
		t.Fatalf("err = %v", err)
	}
}

func TestStatus(t *testing.T) {
	e, o, f := setup(t, "darwin")
	var out bytes.Buffer
	if err := Status(e, &out); err != nil || out.String() != "Autostart is off.\n" {
		t.Fatalf("off: %v %q", err, out.String())
	}
	if err := On(e, o, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	f.answers["launchctl print gui/501/local.mooring"] = "state = running\n"
	out.Reset()
	if err := Status(e, &out); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"Autostart is on.", "File: " + e.path(), "-addr 127.0.0.1:7340", "Running: yes"} {
		if !strings.Contains(out.String(), s) {
			t.Fatalf("missing %q in %q", s, out.String())
		}
	}
	if strings.Contains(out.String(), "Warning") {
		t.Fatalf("warning for the same binary: %q", out.String())
	}
	e.Exe = "/elsewhere/mooring"
	f.answers["launchctl print gui/501/local.mooring"] = ""
	out.Reset()
	_ = Status(e, &out)
	if !strings.Contains(out.String(), "Running: no") || !strings.Contains(out.String(), "Warning: the file runs "+tricky) {
		t.Fatalf("status: %q", out.String())
	}
}
