package terminal

import "io"

// Backend is what a terminal runs on: a local PTY, or a shell on an SSH
// connection. Read gives the output (stdout and stderr together), Write takes
// the input. Reads and writes end with an error once the shell is gone.
type Backend interface {
	io.ReadWriter
	// Wait blocks until the shell has ended and returns its exit code
	// (128+signal when killed, 255 when the connection ended; an SSH shell then
	// also has Lost() true).
	Wait() int
	// Resize sets the size of the terminal the shell sees.
	Resize(cols, rows uint16) error
	// Winch asks a full-screen program to redraw (SIGWINCH to the foreground group).
	// A backend that cannot do it does nothing.
	Winch()
	// Hangup asks the shell to end (SIGHUP, or closing the channel).
	Hangup()
	// Kill ends the shell and whatever it started, for good.
	Kill()
	// Close releases the descriptors after the output is read.
	Close() error
}

// RemoteOpener starts a shell on the SSH connection connID. Errors should be *ledger.Error.
type RemoteOpener func(connID string, cols, rows uint16) (Backend, error)
