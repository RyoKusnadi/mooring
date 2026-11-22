// Package password keeps the optional username and password for signing in.
// The file <data>/password holds one line: a version tag, the username and an
// argon2id hash with its salt and parameters. The password itself is never
// stored, logged or put in an error message.
package password

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// FileName is the file in the data folder.
const FileName = "password"

// Limits on what a person may choose.
const (
	MinPasswordChars = 8
	MaxPasswordBytes = 256
	MaxUsernameChars = 64
)

const (
	version  = "v1"
	saltSize = 16
	keySize  = 32
	// A file with parameters above these is refused, so a damaged file cannot make a login allocate gigabytes.
	maxTime    = 10
	maxMemory  = 256 * 1024
	maxThreads = 8
)

// New hashes use these. A stored hash keeps its own, so they may change later.
var (
	costTime    uint32 = 3
	costMemory  uint32 = 64 * 1024 // KiB
	costThreads uint8  = 1
)

// UseLowCost makes new hashes cheap, so tests of other packages stay fast. Call
// it before anything runs; it returns a function that restores the real cost.
func UseLowCost() (restore func()) {
	if !testing.Testing() {
		return func() {}
	}
	t, m := costTime, costMemory
	costTime, costMemory = 1, 8
	return func() { costTime, costMemory = t, m }
}

// ErrVersion means the file was written by a version this program does not know.
var ErrVersion = errors.New("the password file has an unknown version")

// Record is the stored sign-in: a username and the hash of its password.
type Record struct {
	Username string
	salt     []byte
	key      []byte
	time     uint32
	memory   uint32
	threads  uint8
}

// CleanUsername trims the username and checks its length.
func CleanUsername(name string) (string, error) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n < 1 || n > MaxUsernameChars {
		return "", fmt.Errorf("the username must be 1 to %d characters", MaxUsernameChars)
	}
	return name, nil
}

// CheckPassword checks the length of a new password.
func CheckPassword(pw string) error {
	if utf8.RuneCountInString(pw) < MinPasswordChars {
		return fmt.Errorf("the password must be at least %d characters", MinPasswordChars)
	}
	if len(pw) > MaxPasswordBytes {
		return fmt.Errorf("the password must be at most %d bytes", MaxPasswordBytes)
	}
	return nil
}

// NewRecord hashes pw with a fresh salt.
func NewRecord(username, pw string) (*Record, error) {
	name, err := CleanUsername(username)
	if err != nil {
		return nil, err
	}
	if err := CheckPassword(pw); err != nil {
		return nil, err
	}
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("read random bytes: %w", err)
	}
	r := &Record{Username: name, salt: salt, time: costTime, memory: costMemory, threads: costThreads}
	r.key = argon2.IDKey([]byte(pw), salt, r.time, r.memory, r.threads, keySize)
	return r, nil
}

// Verify reports whether username and pw match. The hash is always computed,
// and the username is compared in constant time, so a wrong username and a
// wrong password take about the same time.
func (r *Record) Verify(username, pw string) bool {
	got := argon2.IDKey([]byte(pw), r.salt, r.time, r.memory, r.threads, uint32(len(r.key)))
	sameKey := subtle.ConstantTimeCompare(got, r.key)
	a, b := sha256.Sum256([]byte(strings.TrimSpace(username))), sha256.Sum256([]byte(r.Username))
	return sameKey&subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

// VerifyNothing spends the time of one check. Used when no password is set, so
// that case cannot be told apart by timing.
func VerifyNothing(pw string) {
	salt := make([]byte, saltSize)
	argon2.IDKey([]byte(pw), salt, costTime, costMemory, costThreads, keySize)
}

func (r *Record) encode() string {
	e := b64
	return strings.Join([]string{version, e(([]byte)(r.Username)), strconv.Itoa(int(r.time)), strconv.Itoa(int(r.memory)),
		strconv.Itoa(int(r.threads)), e(r.salt), e(r.key)}, ":") + "\n"
}

func parse(line string) (*Record, error) {
	parts := strings.Split(strings.TrimSpace(line), ":")
	if len(parts) == 0 || parts[0] != version {
		return nil, ErrVersion
	}
	if len(parts) != 7 {
		return nil, errors.New("the password file is damaged")
	}
	name, err1 := unb64(parts[1])
	t, err2 := strconv.ParseUint(parts[2], 10, 32)
	m, err3 := strconv.ParseUint(parts[3], 10, 32)
	p, err4 := strconv.ParseUint(parts[4], 10, 8)
	salt, err5 := unb64(parts[5])
	key, err6 := unb64(parts[6])
	if err := errors.Join(err1, err2, err3, err4, err5, err6); err != nil {
		return nil, errors.New("the password file is damaged")
	}
	if t < 1 || t > maxTime || m < 8 || m > maxMemory || p < 1 || p > maxThreads || len(salt) < 8 || len(key) < 16 || len(key) > 64 || len(name) == 0 {
		return nil, errors.New("the password file is damaged")
	}
	return &Record{Username: string(name), salt: salt, key: key, time: uint32(t), memory: uint32(m), threads: uint8(p)}, nil
}

// Load reads the record in dir. It returns nil and no error when none is set.
func Load(dir string) (*Record, error) {
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read password file: %w", err)
	}
	return parse(string(data))
}

// Save writes r to dir in one step, readable by its owner only.
func Save(dir string, r *Record) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create data folder: %w", err)
	}
	// A temp file left by a crash holds a hash; do not let it pile up.
	stale, _ := filepath.Glob(filepath.Join(dir, "password-new-*"))
	for _, f := range stale {
		if info, err := os.Stat(f); err == nil && time.Since(info.ModTime()) > time.Minute {
			_ = os.Remove(f)
		}
	}
	tmp, err := os.CreateTemp(dir, "password-new-*")
	if err != nil {
		return fmt.Errorf("create password file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // gone already after a successful rename
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("protect password file: %w", err)
	}
	if _, err := tmp.WriteString(r.encode()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write password file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write password file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write password file: %w", err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, FileName)); err != nil {
		return fmt.Errorf("replace password file: %w", err)
	}
	return nil
}

// Remove deletes the record. Nothing set is not an error.
func Remove(dir string) error {
	err := os.Remove(filepath.Join(dir, FileName))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove password file: %w", err)
	}
	return nil
}
