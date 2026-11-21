package ssh

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	passwordTries   = 4
	passphraseTries = 3
)

var defaultKeys = []string{"id_ed25519", "id_ecdsa", "id_rsa"}

// authMethods lists the sign-in methods for the host, in the order they are tried.
// The returned func releases what the methods hold open.
func (m *Manager) authMethods(ctx context.Context, c *Conn) ([]gossh.AuthMethod, func()) {
	var out []gossh.AuthMethod
	var agentConn net.Conn
	closeFn := func() {
		if agentConn != nil {
			_ = agentConn.Close()
		}
	}
	agentSigners := func() ([]gossh.Signer, error) {
		sock := m.agentSocket()
		if sock == "" {
			return nil, nil
		}
		nc, err := net.Dial("unix", sock)
		if err != nil {
			m.log.Debug("ssh: agent not reachable", "err", err)
			return nil, nil
		}
		agentConn = nc
		return agent.NewClient(nc).Signers()
	}
	// The client sends one "publickey" method, so agent keys and key files share a callback.
	keyMethod := func(useAgent bool, files []string, strict bool) gossh.AuthMethod {
		return gossh.PublicKeysCallback(func() ([]gossh.Signer, error) {
			var signers []gossh.Signer
			if useAgent {
				signers, _ = agentSigners()
			}
			for _, f := range files {
				s, err := m.loadKey(ctx, c, f, strict)
				if err != nil {
					return nil, err
				}
				if s != nil {
					signers = append(signers, s)
				}
			}
			return signers, nil
		})
	}
	switch c.host.Auth {
	case "agent":
		out = append(out, keyMethod(true, nil, false))
	case "key":
		out = append(out, keyMethod(false, []string{c.host.KeyPath}, true))
	case "password":
		out = append(out, m.passwordMethod(ctx, c))
	default:
		out = append(out, keyMethod(true, m.defaultKeyFiles(), false), m.questionMethod(ctx, c), m.passwordMethod(ctx, c))
	}
	return out, closeFn
}

func (m *Manager) defaultKeyFiles() []string {
	var out []string
	for _, n := range defaultKeys {
		p := filepath.Join(m.Home, ".ssh", n)
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			out = append(out, p)
		}
	}
	return out
}

// loadKey reads a private key file. A key with a passphrase asks for it.
// A file that is not a usable key is skipped, or an error when strict (the host names it).
func (m *Manager) loadKey(ctx context.Context, c *Conn, file string, strict bool) (gossh.Signer, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		m.log.Warn("ssh: cannot read key file", "file", file, "err", err)
		if strict {
			err := apiErr("NOT_FOUND", "The key file %s cannot be read.", file)
			c.setFatal(err)
			return nil, err
		}
		return nil, nil
	}
	s, err := gossh.ParsePrivateKey(raw)
	var missing *gossh.PassphraseMissingError
	if !errors.As(err, &missing) {
		if err != nil {
			m.log.Warn("ssh: not a usable key file", "file", file)
			if strict {
				err := apiErr("BAD_REQUEST", "The key file %s is not a private key Mooring can use.", file)
				c.setFatal(err)
				return nil, err
			}
			return nil, nil
		}
		return s, nil
	}
	for try := 0; try < passphraseTries; try++ {
		ans, err := m.ask(ctx, c, &prompt{Kind: "passphrase", Text: fmt.Sprintf("Passphrase for the key %s", file), Retry: try > 0})
		if err != nil {
			c.setFatal(err)
			return nil, err
		}
		s, err := gossh.ParseRawPrivateKeyWithPassphrase(raw, []byte(ans.value))
		if err == nil {
			return gossh.NewSignerFromKey(s)
		}
		if !errors.Is(err, x509.IncorrectPasswordError) {
			break
		}
	}
	m.log.Warn("ssh: key passphrase not accepted", "file", file)
	return nil, nil
}

func (m *Manager) questionMethod(ctx context.Context, c *Conn) gossh.AuthMethod {
	return gossh.KeyboardInteractive(func(_, _ string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i, q := range questions {
			ans, err := m.ask(ctx, c, &prompt{Kind: "question", Text: q, Echo: echos[i]})
			if err != nil {
				c.setFatal(err)
				return nil, err
			}
			answers[i] = ans.value
		}
		return answers, nil
	})
}

// passwordMethod tries the remembered password first, then asks. A remembered
// one that is refused is deleted, and asking goes on.
func (m *Manager) passwordMethod(ctx context.Context, c *Conn) gossh.AuthMethod {
	triedStored, asked := false, false
	return gossh.RetryableAuthMethod(gossh.PasswordCallback(func() (string, error) {
		switch {
		case !triedStored && m.Secrets != nil && c.host.Remembered:
			triedStored = true
			pw, err := m.Secrets.Get(c.host.ID)
			if err == nil {
				c.mu.Lock()
				c.usedMem = true
				c.mu.Unlock()
				return pw, nil
			}
			m.log.Info("ssh: no remembered password", "host", c.host.ID)
			m.setRemembered(c.host.ID, false)
		case c.takeUsedMem():
			m.forget(c.host.ID)
			m.setRemembered(c.host.ID, false)
		}
		ans, err := m.ask(ctx, c, &prompt{Kind: "password", Text: fmt.Sprintf("Password for %s", c.hostLabel()), Retry: asked})
		if err != nil {
			c.setFatal(err)
			return "", err
		}
		asked = true
		c.mu.Lock()
		c.pending = "" // only the password of the latest answer may be remembered
		if ans.remember && m.Secrets != nil {
			c.pending = ans.value
		}
		c.mu.Unlock()
		return ans.value, nil
	}), passwordTries)
}

func (c *Conn) takeUsedMem() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.usedMem
	c.usedMem = false
	return v
}
