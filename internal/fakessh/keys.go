package fakessh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

// GenerateKey makes an ed25519 signer.
func GenerateKey() (ssh.Signer, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("fakessh: make a key: %w", err)
	}
	return ssh.NewSignerFromKey(priv)
}

// PEM encodes a private key in OpenSSH format, with a passphrase when given.
func PEM(priv ed25519.PrivateKey, passphrase string) ([]byte, error) {
	var blk *pem.Block
	var err error
	if passphrase == "" {
		blk, err = ssh.MarshalPrivateKey(priv, "")
	} else {
		blk, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	}
	if err != nil {
		return nil, fmt.Errorf("fakessh: encode a key: %w", err)
	}
	return pem.EncodeToMemory(blk), nil
}

// NewKeyFile writes a new ed25519 private key to path (mode 0600) and returns its signer.
func NewKeyFile(path, passphrase string) (ssh.Signer, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("fakessh: make a key: %w", err)
	}
	raw, err := PEM(priv, passphrase)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, fmt.Errorf("fakessh: write %s: %w", path, err)
	}
	return ssh.NewSignerFromKey(priv)
}

// LoadOrCreateKey reads the unencrypted private key at path, or makes one there.
func LoadOrCreateKey(path string) (ssh.Signer, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NewKeyFile(path, "")
	}
	if err != nil {
		return nil, fmt.Errorf("fakessh: read %s: %w", path, err)
	}
	return ssh.ParsePrivateKey(raw)
}
