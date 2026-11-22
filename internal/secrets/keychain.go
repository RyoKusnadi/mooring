package secrets

import (
	"encoding/base64"
	"fmt"
	"strings"
)

const securityBin = "/usr/bin/security"

// keychain is the macOS login keychain, through /usr/bin/security. A value goes
// in as base64 on the standard input of `security -i`, so it is not in the
// process list and needs no quoting.
type keychain struct{ run runner }

func newKeychain(r runner) *keychain { return &keychain{run: r} }

func (k *keychain) Kind() string { return "keychain" }

func (k *keychain) Get(name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	out, code, err := k.run(securityBin, "", "find-generic-password", "-s", service, "-a", name, "-w")
	if err != nil {
		return "", fmt.Errorf("run security: %w", err)
	}
	if code == 44 { // errSecItemNotFound
		return "", ErrNotFound
	}
	if code != 0 {
		return "", fmt.Errorf("read the keychain: security exited with %d", code)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("read the keychain: the stored value is not one Mooring wrote")
	}
	return string(raw), nil
}

func (k *keychain) Set(name, value string) error {
	if err := checkName(name); err != nil {
		return err
	}
	enc := base64.StdEncoding.EncodeToString([]byte(value))
	line := fmt.Sprintf("add-generic-password -U -s %s -a %s -w %s\n", service, name, enc)
	_, code, err := k.run(securityBin, line, "-i")
	if err != nil {
		return fmt.Errorf("run security: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("write the keychain: security exited with %d", code)
	}
	return nil
}

func (k *keychain) Delete(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	_, code, err := k.run(securityBin, "", "delete-generic-password", "-s", service, "-a", name)
	if err != nil {
		return fmt.Errorf("run security: %w", err)
	}
	if code != 0 && code != 44 {
		return fmt.Errorf("delete from the keychain: security exited with %d", code)
	}
	return nil
}
