// Package secrets keeps the few secrets Mooring remembers (SSH passwords) in
// the operating system's store, or in an encrypted file. Values are never
// logged and never put in an error.
package secrets

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"runtime"
)

// ErrNotFound is returned by Get for a name that has no value.
var ErrNotFound = errors.New("secret not found")

// ErrWrongPassphrase is returned when the encrypted file cannot be opened with the passphrase.
var ErrWrongPassphrase = errors.New("wrong passphrase for the secrets file")

// Store holds named secrets. Names are 1 to 128 characters of letters, digits and ._:-.
type Store interface {
	Get(name string) (string, error)
	Set(name, value string) error
	Delete(name string) error
	// Kind names the backend: "keychain", "secret-tool" or "file".
	Kind() string
}

const service = "mooring"

var namePattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func checkName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("secret name %q is not valid", name)
	}
	return nil
}

// Options says where to look for a store.
type Options struct {
	DataDir string
	// PassphraseFile selects the encrypted file <DataDir>/secrets.enc. It wins over the system stores.
	PassphraseFile string
	Log            *slog.Logger
}

// Open picks a store: the encrypted file when a passphrase file is given, else
// the macOS keychain, else secret-tool on Linux. It returns nil and no error
// when there is none: nothing is remembered then.
func Open(o Options) (Store, error) {
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	if o.PassphraseFile != "" {
		return OpenFile(o.DataDir, o.PassphraseFile, 0)
	}
	switch runtime.GOOS {
	case "darwin":
		if _, err := os.Stat(securityBin); err == nil {
			return newKeychain(execRunner), nil
		}
	case "linux":
		if p, err := exec.LookPath("secret-tool"); err == nil {
			if s := newSecretTool(p, execRunner); s.usable() {
				return s, nil
			}
		}
	}
	o.Log.Info("secrets: no secret store, passwords will not be remembered")
	return nil, nil
}
