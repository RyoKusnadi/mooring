package lsp

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	installTimeout = 10 * time.Minute
	maxLineBytes   = 500
	tailBytes      = 2048
)

// Install runs the one-click install of a server into <data>/lsp.
func (s *Service) Install(ctx context.Context, id string) (Server, error) {
	d, ok, err := s.def(id)
	if err != nil {
		return Server{}, err
	}
	if !ok {
		return Server{}, apiErr("NOT_FOUND", "There is no language server %q.", id)
	}
	if d.Install == nil || s.cfg.DataDir == "" {
		return Server{}, apiErr("BAD_REQUEST", "%s has no one-click install. %s", d.Name, s.view(s.resolve(d)).InstallHint)
	}
	argv, env, err := s.installCommand(d.Install)
	if err != nil {
		return Server{}, err
	}
	s.mu.Lock()
	if s.closed || s.installing[id] {
		s.mu.Unlock()
		return Server{}, apiErr("BUSY", "%s is being installed already.", d.Name)
	}
	s.installing[id] = true
	s.mu.Unlock()
	s.publish("lsp.changed", map[string]any{"server": s.view(s.resolve(d))})
	tail, runErr := s.runInstaller(ctx, id, argv, env)
	s.mu.Lock()
	delete(s.installing, id)
	s.mu.Unlock()
	r := s.resolve(d)
	v := s.view(r)
	s.publish("lsp.changed", map[string]any{"server": v})
	if runErr != nil {
		s.log.Warn("lsp: install failed", "server", id, "err", runErr)
		return Server{}, apiErr("INSTALL_FAILED", "Installing %s failed: %s", d.Name, tailOr(tail, runErr))
	}
	if r.path == "" {
		return Server{}, apiErr("INSTALL_FAILED", "The installer finished, but %s was not found afterwards. %s", d.Command[0], tail)
	}
	return v, nil
}

func tailOr(tail string, err error) string {
	if strings.TrimSpace(tail) != "" {
		return tail
	}
	return err.Error()
}

// installCommand builds the installer's command line and environment. The
// installer must be on the PATH.
func (s *Service) installCommand(in *Install) ([]string, []string, error) {
	prog := map[string]string{"go": "go", "npm": "npm", "rustup": "rustup"}[in.Kind]
	path, ok := find(prog, s.pathDirs())
	if !ok {
		return nil, nil, apiErr("INSTALLER_MISSING", "%s is needed for this install and was not found on the PATH.", prog)
	}
	root := filepath.Join(s.cfg.DataDir, "lsp")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o700); err != nil {
		s.log.Error("lsp: create install folder", "err", err)
		return nil, nil, apiErr("INSTALL_FAILED", "The folder %s could not be created.", root)
	}
	env := []string{"PATH=" + strings.Join(s.pathDirs(), string(os.PathListSeparator))}
	switch in.Kind {
	case "go":
		env = append(env, "GOBIN="+filepath.Join(root, "bin"))
		return append([]string{path, "install"}, in.Args...), env, nil
	case "npm":
		return append([]string{path, "install", "--prefix", root, "--no-save", "--no-fund", "--no-audit"}, in.Args...), env, nil
	}
	return append([]string{path, "component", "add"}, in.Args...), env, nil
}

// runInstaller runs the installer, publishes each output line and returns the
// last 2 KiB of output.
func (s *Service) runInstaller(ctx context.Context, id string, argv, env []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	pr, pw, err := os.Pipe()
	if err != nil {
		return "", err
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		return "", err
	}
	_ = pw.Close()
	var tail []byte
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := scrub(strings.TrimRight(sc.Text(), "\r"))
		if len(line) > maxLineBytes {
			line = strings.ToValidUTF8(line[:maxLineBytes], "")
		}
		tail = append(tail, line...)
		tail = append(tail, '\n')
		if len(tail) > tailBytes {
			tail = tail[len(tail)-tailBytes:]
		}
		s.publish("lsp.install.progress", map[string]any{"id": id, "line": line})
	}
	_ = pr.Close()
	err = cmd.Wait()
	return strings.TrimSpace(strings.ToValidUTF8(string(tail), "")), err
}
