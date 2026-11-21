package ssh

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// hostKeys reads the user's ~/.ssh/known_hosts (never written) and Mooring's
// own file in the data folder (written when the user accepts a key).
type hostKeys struct {
	user, own string
	mu        sync.Mutex
	fileMu    sync.Mutex   // one change of Mooring's own file at a time
	session   []sessionKey // accepted keys when there is no data folder
}

type sessionKey struct {
	addr string
	key  gossh.PublicKey
}

type verdict int

const (
	trusted verdict = iota
	unknownKey
	changedKey
	revokedKey
)

// keyCheck is the outcome of looking a key up.
type keyCheck struct {
	verdict verdict
	// For a changed key: the trusted key of the same type and the file it is in.
	oldKey  gossh.PublicKey
	oldFile string
	// For an unknown key: the key types the files do hold for this host.
	otherTypes []string
}

func (k *hostKeys) check(addr string, remote net.Addr, key gossh.PublicKey) (keyCheck, error) {
	k.mu.Lock()
	for _, s := range k.session {
		if s.addr == addr && string(s.key.Marshal()) == string(key.Marshal()) {
			k.mu.Unlock()
			return keyCheck{verdict: trusted}, nil
		}
	}
	k.mu.Unlock()
	res := keyCheck{verdict: unknownKey}
	for _, file := range []string{k.user, k.own} {
		if file == "" {
			continue
		}
		if _, err := os.Stat(file); err != nil {
			continue
		}
		cb, err := knownhosts.New(file)
		if err != nil {
			return res, fmt.Errorf("read %s: %w", file, err)
		}
		err = cb(addr, remote, key)
		var ke *knownhosts.KeyError
		var re *knownhosts.RevokedError
		switch {
		case err == nil:
			return keyCheck{verdict: trusted}, nil
		case errors.As(err, &re):
			return keyCheck{verdict: revokedKey, oldFile: file}, nil
		case errors.As(err, &ke):
			for _, w := range ke.Want {
				if w.Key.Type() == key.Type() && res.verdict != changedKey {
					res = keyCheck{verdict: changedKey, oldKey: w.Key, oldFile: w.Filename}
				} else if res.verdict == unknownKey {
					res.otherTypes = append(res.otherTypes, w.Key.Type())
				}
			}
		default:
			return res, fmt.Errorf("check %s: %w", file, err)
		}
	}
	return res, nil
}

// save records an accepted key in Mooring's own file.
func (k *hostKeys) save(addr string, key gossh.PublicKey) error {
	if k.own == "" {
		k.mu.Lock()
		k.session = append(k.session, sessionKey{addr, key})
		k.mu.Unlock()
		return nil
	}
	k.fileMu.Lock()
	defer k.fileMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(k.own), 0o700); err != nil {
		return fmt.Errorf("create the data folder: %w", err)
	}
	f, err := os.OpenFile(k.own, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open known_hosts: %w", err)
	}
	line := knownhosts.Line([]string{knownhosts.Normalize(addr)}, key)
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		return fmt.Errorf("write known_hosts: %w", err)
	}
	return f.Close()
}

// forget removes a host from Mooring's own file. Other files are never touched.
func (k *hostKeys) forget(addr string) error {
	k.mu.Lock()
	keep := k.session[:0]
	norm := knownhosts.Normalize(addr)
	for _, s := range k.session {
		if knownhosts.Normalize(s.addr) != norm {
			keep = append(keep, s)
		}
	}
	k.session = keep
	k.mu.Unlock()
	if k.own == "" {
		return nil
	}
	k.fileMu.Lock()
	defer k.fileMu.Unlock()
	f, err := os.Open(k.own)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read known_hosts: %w", err)
	}
	var out strings.Builder
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if fields := strings.Fields(line); len(fields) >= 3 && !strings.HasPrefix(line, "#") {
			hosts := strings.Split(fields[0], ",")
			if contains(hosts, norm) {
				continue
			}
		}
		out.WriteString(line + "\n")
	}
	_ = f.Close()
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read known_hosts: %w", err)
	}
	tf, err := os.CreateTemp(filepath.Dir(k.own), "known_hosts-*.tmp")
	if err != nil {
		return fmt.Errorf("write known_hosts: %w", err)
	}
	tmp := tf.Name()
	_, werr := tf.WriteString(out.String())
	if cerr := tf.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write known_hosts: %w", werr)
	}
	if err := os.Rename(tmp, k.own); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write known_hosts: %w", err)
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// hostKeyCallback checks the key during the handshake. It asks the user about
// an unknown key. Errors from it are kept in c.fatal, since the handshake wraps them.
// errRetry ends a handshake so dial can offer the host key types we know.
var errRetry = errors.New("retry with the known host key types")

func (m *Manager) hostKeyCallback(ctx context.Context, c *Conn) gossh.HostKeyCallback {
	return func(addr string, remote net.Addr, key gossh.PublicKey) error {
		err := m.verifyHostKey(ctx, c, addr, remote, key)
		if err != nil {
			c.setFatal(err)
		}
		return err
	}
}

func (m *Manager) verifyHostKey(ctx context.Context, c *Conn, addr string, remote net.Addr, key gossh.PublicKey) error {
	chk, err := m.keys.check(addr, remote, key)
	if err != nil {
		m.log.Error("ssh: known_hosts", "err", err)
		return apiErr("INTERNAL", "A known_hosts file could not be read: %v", err)
	}
	fp := gossh.FingerprintSHA256(key)
	switch chk.verdict {
	case trusted:
		return nil
	case revokedKey:
		return apiErr("HOST_KEY_REJECTED", "The key of %s is marked revoked in %s.", addr, chk.oldFile)
	case changedKey:
		return apiErr("HOST_KEY_CHANGED", "The host key of %s changed. It was %s and is now %s. Someone may be in the middle, or the server was reinstalled. %s",
			addr, gossh.FingerprintSHA256(chk.oldKey), fp, m.changedHint(chk.oldFile, addr))
	}
	if len(chk.otherTypes) > 0 && c.tryKnownTypes(chk.otherTypes) {
		return errRetry // the host is known by another key type: offer that one
	}
	ans, err := m.ask(ctx, c, &prompt{Kind: "hostkey", KeyType: key.Type(), Fingerprint: fp})
	if err != nil {
		return err
	}
	if !ans.accept {
		return apiErr("HOST_KEY_REJECTED", "You did not accept the host key of %s.", addr)
	}
	if err := m.keys.save(addr, key); err != nil {
		m.log.Error("ssh: save host key", "err", err)
		return apiErr("INTERNAL", "The host key could not be saved.")
	}
	return nil
}

func (m *Manager) changedHint(file, addr string) string {
	if file == m.keys.user {
		return fmt.Sprintf("The old key is in %s; if the change is expected, remove it there with ssh-keygen -R %s.", file, hostOf(addr))
	}
	return fmt.Sprintf("The old key is in %s; if the change is expected, forget the host key first.", file)
}

func hostOf(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
}

// ForgetHostKey removes a host from Mooring's own file.
func (m *Manager) ForgetHostKey(address string, port int) error {
	if address == "" || port < 1 || port > 65535 {
		return apiErr("BAD_REQUEST", "Give the address and the port.")
	}
	if err := m.keys.forget(net.JoinHostPort(address, fmt.Sprint(port))); err != nil {
		m.log.Error("ssh: forget host key", "err", err)
		return apiErr("INTERNAL", "The host key could not be removed.")
	}
	return nil
}
