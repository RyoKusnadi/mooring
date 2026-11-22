package secrets

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Rounds is the PBKDF2-SHA256 round count of a new file.
const Rounds = 600_000

const fileName = "secrets.enc"

// fileStore is an AES-256-GCM encrypted JSON file. The key comes from PBKDF2
// over the passphrase and a random salt kept in the file.
type fileStore struct {
	path string
	mu   sync.Mutex
	key  []byte
	hdr  header
	vals map[string]string
}

type header struct {
	V      int    `json:"v"`
	Rounds int    `json:"rounds"`
	Salt   []byte `json:"salt"`
	Nonce  []byte `json:"nonce"`
	Data   []byte `json:"data"`
}

// OpenFile opens (or prepares) <dir>/secrets.enc with the passphrase read from
// passphraseFile, which only its owner may read. rounds 0 means Rounds; tests lower it.
func OpenFile(dir, passphraseFile string, rounds int) (Store, error) {
	pass, err := readPassphrase(passphraseFile)
	if err != nil {
		return nil, err
	}
	if rounds <= 0 {
		rounds = Rounds
	}
	s := &fileStore{path: filepath.Join(dir, fileName), vals: map[string]string{}}
	raw, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.hdr = header{V: 1, Rounds: rounds, Salt: make([]byte, 16)}
		if _, err := rand.Read(s.hdr.Salt); err != nil {
			return nil, fmt.Errorf("make a salt: %w", err)
		}
		if s.key, err = derive(pass, s.hdr); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, fmt.Errorf("read %s: %w", fileName, err)
	default:
		if err := json.Unmarshal(raw, &s.hdr); err != nil || s.hdr.V != 1 || s.hdr.Rounds < 1 {
			return nil, fmt.Errorf("%s is not a secrets file Mooring can read", fileName)
		}
		if s.key, err = derive(pass, s.hdr); err != nil {
			return nil, err
		}
		if err := s.decrypt(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func readPassphrase(path string) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read the passphrase file: %w", err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("the passphrase file %s must be readable only by you (chmod 600)", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read the passphrase file: %w", err)
	}
	pass := bytes.TrimRight(raw, "\r\n")
	if len(pass) == 0 {
		return nil, errors.New("the passphrase file is empty")
	}
	return pass, nil
}

func derive(pass []byte, h header) ([]byte, error) {
	key, err := pbkdf2.Key(sha256.New, string(pass), h.Salt, h.Rounds, 32)
	if err != nil {
		return nil, fmt.Errorf("derive the key: %w", err)
	}
	return key, nil
}

func (s *fileStore) aead() (cipher.AEAD, error) {
	blk, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(blk)
}

func (s *fileStore) decrypt() error {
	g, err := s.aead()
	if err != nil {
		return fmt.Errorf("set up the cipher: %w", err)
	}
	if len(s.hdr.Nonce) != g.NonceSize() {
		return fmt.Errorf("%s is damaged", fileName)
	}
	plain, err := g.Open(nil, s.hdr.Nonce, s.hdr.Data, nil)
	if err != nil {
		return ErrWrongPassphrase
	}
	if err := json.Unmarshal(plain, &s.vals); err != nil {
		return fmt.Errorf("%s is damaged", fileName)
	}
	return nil
}

func (s *fileStore) save() error {
	g, err := s.aead()
	if err != nil {
		return fmt.Errorf("set up the cipher: %w", err)
	}
	plain, err := json.Marshal(s.vals)
	if err != nil {
		return err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("make a nonce: %w", err)
	}
	h := s.hdr
	h.Nonce, h.Data = nonce, g.Seal(nil, nonce, plain, nil)
	raw, err := json.Marshal(h)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", fileName, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", fileName, err)
	}
	s.hdr = h
	return nil
}

func (s *fileStore) Kind() string { return "file" }

func (s *fileStore) Get(name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.vals[name]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (s *fileStore) Set(name, value string) error {
	if err := checkName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, had := s.vals[name]
	s.vals[name] = value
	if err := s.save(); err != nil {
		if had {
			s.vals[name] = old
		} else {
			delete(s.vals, name)
		}
		return err
	}
	return nil
}

func (s *fileStore) Delete(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.vals[name]
	if !ok {
		return nil
	}
	delete(s.vals, name)
	if err := s.save(); err != nil {
		s.vals[name] = old
		return err
	}
	return nil
}
