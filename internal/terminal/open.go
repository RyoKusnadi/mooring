package terminal

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ryokusnadi/mooring/internal/paths"
)

type openReq struct {
	projectID, folder, ssh, id string
	env                        []string // extra KEY=VALUE for a local shell
	banner                     string   // written to the terminal before the shell's own output
	cols, rows                 int
}

func (m *Manager) open(r openReq) (Info, error) {
	id, projectID, cols, rows := r.id, r.projectID, r.cols, r.rows
	if !idPattern.MatchString(id) {
		return Info{}, apiErr("BAD_REQUEST", "A terminal id needs 1 to 64 letters, digits, - or _.")
	}
	if r.ssh != "" && (len(r.env) > 0 || r.banner != "") {
		return Info{}, apiErr("BAD_REQUEST", "A remote shell cannot take extra environment or a banner.")
	}
	if cols == 0 && rows == 0 {
		cols, rows = defaultCols, defaultRows
	}
	if cols < minCols || cols > maxCols || rows < 1 || rows > maxRows {
		return Info{}, apiErr("BAD_REQUEST", "The terminal size is not valid.")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	dir, err := m.Dir(projectID) // under the lock, so CloseProject cannot miss this shell
	if err != nil {
		return Info{}, err
	}
	if r.ssh == "" && r.folder != "" {
		if dir, err = projectFolder(dir, r.folder); err != nil {
			return Info{}, err
		}
	}
	if err := m.admitLocked(id, projectID); err != nil {
		return Info{}, err
	}
	var b Backend
	if r.ssh != "" {
		// A remote shell can take seconds to open: do not hold the lock meanwhile.
		m.mu.Unlock()
		b, err = m.Remote(r.ssh, uint16(cols), uint16(rows))
		m.mu.Lock()
		if err != nil {
			return Info{}, err
		}
		if err := m.admitLocked(id, projectID); err != nil {
			_ = b.Close()
			return Info{}, err
		}
	} else if b, err = m.startShell(dir, r.env, uint16(cols), uint16(rows)); err != nil {
		m.log.Error("terminal: start failed", "project", projectID, "err", err)
		return Info{}, apiErr("INTERNAL", "The shell could not be started.")
	}
	t, err := newTerminal(m, id, projectID, r.ssh, b, uint16(cols), uint16(rows))
	if err != nil {
		m.log.Error("terminal: start failed", "project", projectID, "err", err)
		return Info{}, apiErr("INTERNAL", "The shell could not be started.")
	}
	if r.ssh == "" {
		t.startDir = dir
	}
	if r.banner != "" {
		t.publish([]byte(r.banner))
	}
	m.log.Info("terminal: started", "id", id, "project", projectID, "ssh", r.ssh)
	m.terms[id] = t
	info := t.info()
	m.pub.Publish("terminal.opened", info)
	return info, nil
}

func projectFolder(dir, folder string) (string, error) {
	if strings.ContainsRune(folder, 0) || filepath.IsAbs(folder) {
		return "", apiErr("BAD_REQUEST", "The folder must be a path inside the project.")
	}
	real, err := paths.Resolve(dir, folder)
	if err != nil {
		return "", apiErr("OUTSIDE_PROJECT", "That folder is not inside the project.")
	}
	if fi, err := os.Stat(real); err != nil || !fi.IsDir() {
		return "", apiErr("NOT_FOUND", "That folder does not exist.")
	}
	return real, nil
}

func (m *Manager) startShell(dir string, env []string, cols, rows uint16) (Backend, error) {
	shell, args := m.shell()
	b, err := startLocal(shell, args, dir, env, cols, rows)
	if err != nil {
		return nil, err
	}
	m.log.Info("terminal: shell", "pid", b.pid(), "shell", shell)
	return b, nil
}

// admitLocked checks that the manager is open, the id is free and the limits allow another terminal.
func (m *Manager) admitLocked(id, projectID string) error {
	if m.closed {
		return apiErr("INTERNAL", "The server is shutting down.")
	}
	if _, ok := m.terms[id]; ok {
		return apiErr("ALREADY_EXISTS", "That terminal already exists.")
	}
	total, project := 0, 0
	for _, t := range m.terms {
		if info := t.info(); !info.Exited {
			total++
			if info.ProjectID == projectID {
				project++
			}
		}
	}
	if total >= MaxTerminals {
		return apiErr("LIMIT_REACHED", "%d terminals are running. Close one to open another.", MaxTerminals)
	}
	if project >= MaxPerProject {
		return apiErr("LIMIT_REACHED", "This project has %d terminals open. Close one to open another.", MaxPerProject)
	}
	return nil
}
