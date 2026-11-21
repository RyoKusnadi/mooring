package ssh

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"unicode"

	"github.com/ryokusnadi/mooring/internal/paths"
)

// Host is a saved place to connect to.
type Host struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Address      string `json:"address"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Auth         string `json:"auth"`
	KeyPath      string `json:"keyPath"`
	AgentForward bool   `json:"agentForward"`
	Root         string `json:"root"`
	Remembered   bool   `json:"remembered"`
	// SavedKey says where a host key for this address and port is kept: "", "mooring" or "user".
	SavedKey string `json:"savedKey"`
}

const hostColumns = `id, name, address, port, user, auth, key_path, agent_forward, root, remembered`

type scanner interface{ Scan(...any) error }

func scanHost(r scanner) (Host, error) {
	var h Host
	err := r.Scan(&h.ID, &h.Name, &h.Address, &h.Port, &h.User, &h.Auth, &h.KeyPath, &h.AgentForward, &h.Root, &h.Remembered)
	return h, err
}

// Hosts lists the saved hosts in the order they were saved.
func (m *Manager) Hosts() ([]Host, error) {
	rows, err := m.DB.Query(`SELECT ` + hostColumns + ` FROM ssh_hosts ORDER BY position`)
	if err != nil {
		return nil, fmt.Errorf("list ssh hosts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Host{}
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, fmt.Errorf("read ssh host: %w", err)
		}
		h.SavedKey = m.keys.source(h.Address, h.Port)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (m *Manager) host(id string) (Host, error) {
	h, err := scanHost(m.DB.QueryRow(`SELECT `+hostColumns+` FROM ssh_hosts WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Host{}, apiErr("NOT_FOUND", "That host is not saved.")
	}
	if err != nil {
		return Host{}, fmt.Errorf("read ssh host: %w", err)
	}
	return h, nil
}

func plain(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// validate checks and normalises a host from the browser. Remembered is not the browser's to set.
func (m *Manager) validate(h *Host) error {
	h.Name = strings.TrimSpace(h.Name)
	if n := len([]rune(h.Name)); n < 1 || n > 100 {
		return apiErr("BAD_REQUEST", "The name needs 1 to 100 characters.")
	}
	if h.Address == "" || len(h.Address) > 253 || !plain(h.Address) || strings.HasPrefix(h.Address, "-") {
		return apiErr("BAD_REQUEST", "The address is not valid.")
	}
	if h.Port == 0 {
		h.Port = 22
	}
	if h.Port < 1 || h.Port > 65535 {
		return apiErr("BAD_REQUEST", "The port must be 1 to 65535.")
	}
	if h.User == "" || len(h.User) > 128 || !plain(h.User) {
		return apiErr("BAD_REQUEST", "The user name is not valid.")
	}
	switch h.Auth {
	case "auto", "agent", "password":
		h.KeyPath = ""
	case "key":
		kp, err := m.keyPath(h.KeyPath)
		if err != nil {
			return err
		}
		h.KeyPath = kp
	default:
		return apiErr("BAD_REQUEST", "The sign-in method must be auto, agent, key or password.")
	}
	if h.Root != "" {
		if strings.ContainsRune(h.Root, 0) || !path.IsAbs(h.Root) || len(h.Root) > 4096 {
			return apiErr("BAD_REQUEST", "The remote folder must be an absolute path, or empty for the home folder.")
		}
		h.Root = path.Clean(h.Root)
	}
	return nil
}

// keyPath expands ~ and checks that the key file is under the home folder.
func (m *Manager) keyPath(p string) (string, error) {
	if p == "" {
		return "", apiErr("BAD_REQUEST", "Choose the key file.")
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		p = m.Home + p[1:]
	}
	real, err := paths.Resolve(m.Home, p)
	if err != nil {
		if errors.Is(err, paths.ErrOutsideRoot) {
			return "", apiErr("OUTSIDE_ROOT", "The key file must be inside your home folder.")
		}
		return "", apiErr("BAD_REQUEST", "That key file cannot be used.")
	}
	if fi, err := os.Stat(real); err != nil || !fi.Mode().IsRegular() {
		return "", apiErr("NOT_FOUND", "That key file does not exist.")
	}
	return real, nil
}

// SaveHost adds a host (no id) or changes one.
func (m *Manager) SaveHost(h Host) (Host, error) {
	if err := m.validate(&h); err != nil {
		return Host{}, err
	}
	m.hostsMu.Lock()
	defer m.hostsMu.Unlock()
	if h.ID == "" {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return Host{}, fmt.Errorf("make a host id: %w", err)
		}
		h.ID = hex.EncodeToString(b)
		h.Remembered = false
		_, err := m.DB.Exec(`INSERT INTO ssh_hosts (`+hostColumns+`, position)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, (SELECT COALESCE(MAX(position), 0) + 1 FROM ssh_hosts))`,
			h.ID, h.Name, h.Address, h.Port, h.User, h.Auth, h.KeyPath, h.AgentForward, h.Root)
		if err != nil {
			return Host{}, fmt.Errorf("save ssh host: %w", err)
		}
	} else {
		old, err := m.host(h.ID)
		if err != nil {
			return Host{}, err
		}
		h.Remembered = old.Remembered && old.Address == h.Address && old.User == h.User && old.Port == h.Port
		if _, err := m.DB.Exec(`UPDATE ssh_hosts SET name=?, address=?, port=?, user=?, auth=?, key_path=?,
			agent_forward=?, root=?, remembered=? WHERE id=?`,
			h.Name, h.Address, h.Port, h.User, h.Auth, h.KeyPath, h.AgentForward, h.Root, h.Remembered, h.ID); err != nil {
			return Host{}, fmt.Errorf("save ssh host: %w", err)
		}
		if old.Remembered && !h.Remembered {
			m.forget(h.ID) // the password was for another place
		}
	}
	m.Pub.Publish("ssh.hostsChanged", map[string]any{})
	h.SavedKey = m.keys.source(h.Address, h.Port)
	return h, nil
}

// RemoveHost deletes a host and its remembered password. Open connections stay.
func (m *Manager) RemoveHost(id string) error {
	m.hostsMu.Lock()
	defer m.hostsMu.Unlock()
	if _, err := m.DB.Exec(`DELETE FROM ssh_hosts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("remove ssh host: %w", err)
	}
	m.forget(id)
	m.Pub.Publish("ssh.hostsChanged", map[string]any{})
	return nil
}

// forget deletes the remembered password; a failure is logged and the host's flag cleared anyway.
func (m *Manager) forget(id string) {
	if m.Secrets == nil {
		return
	}
	if err := m.Secrets.Delete(id); err != nil {
		m.log.Warn("ssh: could not delete a remembered password", "host", id, "err", err)
	}
}

// setRemembered updates the flag and tells the browsers.
func (m *Manager) setRemembered(id string, v bool) {
	m.hostsMu.Lock()
	defer m.hostsMu.Unlock()
	res, err := m.DB.Exec(`UPDATE ssh_hosts SET remembered = ? WHERE id = ? AND remembered != ?`, v, id, v)
	if err != nil {
		m.log.Warn("ssh: could not update a host", "host", id, "err", err)
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		m.Pub.Publish("ssh.hostsChanged", map[string]any{})
	}
}
