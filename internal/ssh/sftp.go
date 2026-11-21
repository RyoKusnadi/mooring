package ssh

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/pkg/sftp"

	"github.com/ryokusnadi/mooring/internal/paths"
)

// files returns the connection's SFTP client, opening it on first use.
func (c *Conn) files() (*sftp.Client, error) {
	c.sftpMu.Lock()
	defer c.sftpMu.Unlock()
	if c.sftp != nil {
		return c.sftp, nil
	}
	if c.sftpErr != nil {
		return nil, c.sftpErr
	}
	cl, err := sftp.NewClient(c.client)
	if err != nil {
		select {
		case <-c.closed:
			return nil, apiErr("NOT_CONNECTED", "That connection is closed. Connect again.")
		default:
		}
		c.m.log.Warn("ssh: sftp", "conn", c.id, "err", err)
		c.sftpErr = apiErr("SFTP_UNAVAILABLE", "The server does not offer file transfer (SFTP).")
		return nil, c.sftpErr
	}
	c.sftp = cl
	return cl, nil
}

// remoteFS is one connection's files with the host's root: every path goes
// through paths.ResolveIn.
type remoteFS struct {
	c    *Conn
	cl   *sftp.Client
	root string // real absolute root
}

// fsys returns the files of a connection with the root resolved.
func (m *Manager) fsys(connID string) (*remoteFS, error) {
	c, err := m.conn(connID)
	if err != nil {
		return nil, err
	}
	cl, err := c.files()
	if err != nil {
		return nil, err
	}
	root := c.host.Root
	if root == "" {
		if root, err = cl.RealPath("."); err != nil {
			return nil, mapErr(err)
		}
	}
	real, err := paths.RootIn(cl, root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, apiErr("NOT_FOUND", "The remote folder %s does not exist.", root)
		}
		return nil, mapErr(err)
	}
	return &remoteFS{c: c, cl: cl, root: real}, nil
}

const maxPath = 4096

// clean checks a path from the browser and returns it without "." parts.
func clean(rel string) (string, error) {
	if len(rel) > maxPath || strings.ContainsRune(rel, 0) || strings.HasPrefix(rel, "/") {
		return "", apiErr("BAD_REQUEST", "That path is not valid. Paths are relative to the remote folder.")
	}
	c := path.Clean(rel)
	if c == "." {
		return "", nil
	}
	return c, nil
}

// follow resolves a browser path through links to a real path inside the root.
func (r *remoteFS) follow(rel string) (string, error) {
	c, err := clean(rel)
	if err != nil {
		return "", err
	}
	real, err := paths.ResolveIn(r.cl, r.root, c)
	if err != nil {
		return "", mapErr(err)
	}
	return real, nil
}

// entry resolves the folder that holds rel and keeps the last name, so a link is acted on and not its target.
func (r *remoteFS) entry(rel string) (string, error) {
	c, err := clean(rel)
	if err != nil {
		return "", err
	}
	if c == "" || path.Base(c) == ".." {
		return "", apiErr("BAD_REQUEST", "That path is not valid.")
	}
	parent, err := paths.ResolveIn(r.cl, r.root, path.Dir(c))
	if err != nil {
		return "", mapErr(err)
	}
	fi, err := r.cl.Stat(parent)
	if err != nil {
		return "", mapErr(err)
	}
	if !fi.IsDir() {
		return "", apiErr("NOT_A_FOLDER", "A part of that path is a file, not a folder.")
	}
	return path.Join(parent, path.Base(c)), nil
}

// mapErr turns SFTP and resolver errors into contract errors.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, paths.ErrOutsideRoot):
		return apiErr("OUTSIDE_ROOT", "That path leaves the remote folder.")
	case errors.Is(err, fs.ErrNotExist):
		return apiErr("NOT_FOUND", "That file or folder does not exist.")
	case errors.Is(err, fs.ErrPermission):
		return apiErr("PERMISSION_DENIED", "The remote server refused: permission denied.")
	case errors.Is(err, fs.ErrExist):
		return apiErr("ALREADY_EXISTS", "Something with that name already exists.")
	case errors.Is(err, io.EOF), errors.Is(err, sftp.ErrSSHFxConnectionLost), errors.Is(err, os.ErrClosed),
		strings.Contains(err.Error(), "connection lost"), strings.Contains(err.Error(), "use of closed"):
		return apiErr("NOT_CONNECTED", "The connection was lost. Connect again.")
	}
	return fmt.Errorf("remote file operation: %w", err)
}
