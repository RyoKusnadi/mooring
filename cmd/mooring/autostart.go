package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ryokusnadi/mooring/internal/autostart"
)

// realAutostartEnv reads the machine. Tests pass their own.
func realAutostartEnv() (autostart.Env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return autostart.Env{}, fmt.Errorf("find the home folder: %w", err)
	}
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return autostart.Env{}, fmt.Errorf("find this program: %w", err)
	}
	avoid := []string{os.TempDir()}
	if real, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		avoid = append(avoid, real)
	}
	if cache, err := os.UserCacheDir(); err == nil {
		avoid = append(avoid, filepath.Join(cache, "go-build"))
	}
	return autostart.Env{
		GOOS: runtime.GOOS, Home: home, XDGConfigHome: os.Getenv("XDG_CONFIG_HOME"), UID: os.Getuid(), Exe: exe, Avoid: avoid,
		PortFree: func(addr string) bool {
			l, err := net.Listen("tcp", addr)
			if err != nil {
				return false
			}
			_ = l.Close()
			return true
		},
		Run: func(name string, args ...string) ([]byte, error) {
			cmd := exec.Command(name, args...)
			done := time.AfterFunc(30*time.Second, func() { _ = cmd.Process.Kill() })
			defer done.Stop()
			return cmd.CombinedOutput()
		},
	}, nil
}

func runAutostart(args []string, envFn func() (autostart.Env, error), stdout, stderr io.Writer) error {
	if len(args) == 0 || (args[0] != "on" && args[0] != "off" && args[0] != "status") {
		return errors.New("usage: mooring autostart on|off|status")
	}
	action := args[0]
	flags := flag.NewFlagSet("mooring autostart", flag.ContinueOnError)
	flags.SetOutput(stderr)
	addr := flags.String("addr", "127.0.0.1:7340", "address the server listens on")
	dataDir := flags.String("data", "", "data folder (default: the user config folder)")
	shell := flags.String("shell", "", "program to run in terminals")
	secretsPass := flags.String("secrets-passphrase-file", "", "file with the passphrase that protects remembered SSH passwords")
	tlsCert := flags.String("tls-cert", "", "certificate file (PEM) to serve HTTPS")
	tlsKey := flags.String("tls-key", "", "private key file (PEM) for -tls-cert")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unknown argument %q", flags.Arg(0))
	}
	if action == "on" {
		// Fail here, not at the next login where nobody reads the error.
		t, err := loadTLS(*tlsCert, *tlsKey)
		if err != nil {
			return err
		}
		dir, err := resolveDataDir(*dataDir)
		if err != nil {
			return err
		}
		if err := checkExposure(*addr, dir, t); err != nil {
			return err
		}
	}
	env, err := envFn()
	if err != nil {
		return err
	}
	switch action {
	case "off":
		return autostart.Off(env, stdout)
	case "status":
		return autostart.Status(env, stdout)
	}
	dir, err := resolveDataDir(*dataDir)
	if err != nil {
		return err
	}
	opts := autostart.Options{Addr: *addr, Data: dir, PathEnv: os.Getenv("PATH"), ShellEnv: os.Getenv("SHELL")}
	if opts.Shell, err = absOrEmpty(*shell); err != nil {
		return err
	}
	if opts.SecretsPass, err = absOrEmpty(*secretsPass); err != nil {
		return err
	}
	if *tlsCert != "" {
		// Files, not programs: a bare name is in this folder, not on PATH.
		if opts.TLSCert, err = filepath.Abs(*tlsCert); err != nil {
			return err
		}
		if opts.TLSKey, err = filepath.Abs(*tlsKey); err != nil {
			return err
		}
	}
	return autostart.On(env, opts, stdout)
}

// absOrEmpty makes a flag path absolute so the job does not depend on a working folder.
// A bare program name such as "zsh" is left to the shell search.
func absOrEmpty(p string) (string, error) {
	if p == "" || !filepath.IsAbs(p) && filepath.Base(p) == p {
		return p, nil
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", p, err)
	}
	return abs, nil
}
