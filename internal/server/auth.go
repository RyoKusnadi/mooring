package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie  = "mooring_session"
	sessionTTL     = 30 * 24 * time.Hour
	tokenRecheck   = time.Second
	maxFailures    = 5
	failureWindow  = time.Minute
	tokenFileName  = "token"
	randomIDLength = 32
)

// LoadOrCreateToken returns the login token stored in dir, creating the
// directory and a new token on first run. The file is readable by its owner only.
func LoadOrCreateToken(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create data folder: %w", err)
	}
	path := filepath.Join(dir, tokenFileName)
	data, err := os.ReadFile(path)
	if err == nil {
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", fmt.Errorf("token file %s is empty; delete it to create a new token", path)
		}
		return token, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read token file: %w", err)
	}
	token, err := randomID()
	if err != nil {
		return "", err
	}
	// O_EXCL: two servers starting at once must not each write a different token.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create token file: %w", err)
	}
	if _, err := f.WriteString(token + "\n"); err != nil {
		_ = f.Close() // the write error is the one worth reporting
		return "", fmt.Errorf("write token file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("write token file: %w", err)
	}
	return token, nil
}

// ReplaceToken writes a new login token to dir and returns it. Every session
// cookie and sign-in link made from the old token stops working, because
// their keys are derived from the token. The file is replaced in one step, so
// a running server never reads half a token.
func ReplaceToken(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create data folder: %w", err)
	}
	token, err := randomID()
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "token-new-*")
	if err != nil {
		return "", fmt.Errorf("create token file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // gone already after a successful rename
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("protect token file: %w", err)
	}
	if _, err := tmp.WriteString(token + "\n"); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write token file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("write token file: %w", err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, tokenFileName)); err != nil {
		return "", fmt.Errorf("replace token file: %w", err)
	}
	return token, nil
}

func randomID() (string, error) {
	buf := make([]byte, randomIDLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// keys are everything derived from the login token.
type keys struct {
	token   string
	hash    [sha256.Size]byte // for comparing a typed token
	session []byte            // signs session cookies
	link    []byte            // signs sign-in codes
	hello   []byte            // proves to `mooring open` that this is the server
}

func deriveKeys(token string) keys {
	return keys{
		token:   token,
		hash:    sha256.Sum256([]byte(token)),
		session: deriveKey(token, sessionLabel),
		link:    deriveKey(token, linkLabel),
		hello:   deriveKey(token, helloLabel),
	}
}

// auth holds the keys and the used sign-in codes.
// Sessions are not stored: each cookie carries a signed expiry.
type auth struct {
	now   func() time.Time
	start time.Time // codes made before this are refused
	// tokenPath, when set, is re-read about once a second so `mooring token -new`
	// signs everyone out without a restart.
	tokenPath string
	// onRotate runs after the keys change, so open sockets can be closed. Set once before use.
	onRotate func()

	mu        sync.Mutex
	keys      keys
	lastCheck time.Time
	stamp     os.FileInfo          // the token file as last read: identity, modification time, size
	used      map[string]time.Time // sign-in codes already redeemed, until they expire
}

func newAuth(token, tokenPath string, now func() time.Time) *auth {
	return &auth{now: now, start: now(), tokenPath: tokenPath, keys: deriveKeys(token),
		lastCheck: now(), used: make(map[string]time.Time)}
}

// refresh re-reads the token file when its size or modification time changed,
// so `mooring token -new` signs everyone out without a restart. It checks at
// most once a second and never holds a.mu during file access.
func (a *auth) refresh() {
	if a.tokenPath == "" {
		return
	}
	a.mu.Lock()
	now := a.now()
	due := now.Sub(a.lastCheck) >= tokenRecheck || now.Before(a.lastCheck)
	if due {
		a.lastCheck = now
	}
	a.mu.Unlock()
	if !due {
		return
	}
	info, err := os.Stat(a.tokenPath)
	if err != nil {
		return
	}
	a.mu.Lock()
	same := a.stamp != nil && os.SameFile(info, a.stamp) && info.ModTime().Equal(a.stamp.ModTime()) && info.Size() == a.stamp.Size()
	a.mu.Unlock()
	if same {
		return
	}
	data, err := os.ReadFile(a.tokenPath)
	if err != nil {
		return
	}
	t := strings.TrimSpace(string(data))
	a.mu.Lock()
	a.stamp = info
	changed := t != "" && subtle.ConstantTimeCompare([]byte(t), []byte(a.keys.token)) != 1
	if changed {
		a.keys = deriveKeys(t)
		clear(a.used)
	}
	a.mu.Unlock()
	if changed && a.onRotate != nil {
		a.onRotate()
	}
}

// rotate switches to a new token at once, so every cookie and link made from
// the old one stops working. The caller has already written the token file.
func (a *auth) rotate(token string) {
	a.mu.Lock()
	a.keys = deriveKeys(token)
	clear(a.used)
	a.mu.Unlock()
	if a.onRotate != nil {
		a.onRotate()
	}
}

// login checks the token and, when it matches, returns a signed session cookie value.
func (a *auth) login(token string) (string, bool, error) {
	got := sha256.Sum256([]byte(token))
	a.refresh()
	a.mu.Lock()
	k := a.keys
	a.mu.Unlock()
	// Comparing digests keeps the comparison constant-time for any input length.
	if subtle.ConstantTimeCompare(got[:], k.hash[:]) != 1 {
		return "", false, nil
	}
	value, err := a.newSession()
	return value, err == nil, err
}
