package ssh

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ryokusnadi/mooring/internal/events"
)

// Handles reports whether name is an ssh command.
func Handles(name string) bool { return strings.HasPrefix(name, "ssh.") }

// Secret is true for ssh.answer: its args hold a password.
func Secret(name string) bool { return name == "ssh.answer" }

type cmdArgs struct {
	ID         string `json:"id"`
	HostID     string `json:"hostId"`
	Host       Host   `json:"host"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Conn       string `json:"conn"`
	Path       string `json:"path"`
	From       string `json:"from"`
	To         string `json:"to"`
	Recursive  bool   `json:"recursive"`
	LocalPort  int    `json:"localPort"`
	RemoteHost string `json:"remoteHost"`
	RemotePort int    `json:"remotePort"`
}

// Run runs an ssh command. Errors are *ledger.Error.
func (m *Manager) Run(ctx context.Context, cmd events.Command) (any, error) {
	if cmd.Name == "ssh.answer" {
		if err := m.Answer(cmd.Args); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	}
	var a cmdArgs
	if err := json.Unmarshal(cmd.Args, &a); err != nil {
		return nil, apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	empty := map[string]any{}
	switch cmd.Name {
	case "ssh.hostSave":
		return m.SaveHost(a.Host)
	case "ssh.hostRemove":
		return empty, m.RemoveHost(a.ID)
	case "ssh.connect":
		return m.Connect(ctx, a.ID, a.HostID)
	case "ssh.disconnect":
		return empty, m.Disconnect(a.ID)
	case "ssh.forgetHostKey":
		return empty, m.ForgetHostKey(a.Address, a.Port)
	case "ssh.fileRename":
		return m.Rename(a.Conn, a.From, a.To)
	case "ssh.fileDelete":
		return empty, m.Delete(a.Conn, a.Path, a.Recursive)
	case "ssh.mkdir":
		return empty, m.Mkdir(a.Conn, a.Path)
	case "ssh.forward":
		return m.Forward(a.Conn, "127.0.0.1", a.LocalPort, a.RemoteHost, a.RemotePort)
	case "ssh.forwardStop":
		m.StopForward(a.ID)
		return empty, nil
	}
	return nil, apiErr("BAD_REQUEST", "Unknown command %s.", cmd.Name)
}
