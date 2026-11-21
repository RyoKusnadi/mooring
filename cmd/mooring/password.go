package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/ryokusnadi/mooring/internal/password"
)

// passwordInput is what `mooring password` needs from the terminal. Tests replace it.
type passwordInput interface {
	isTerminal() bool
	line(prompt string) (string, error)
	secret(prompt string) (string, error) // reads without echo
}

type ttyInput struct {
	in  *os.File
	out io.Writer
	rd  *bufio.Reader
}

func realPasswordInput(stdout io.Writer) passwordInput {
	return &ttyInput{in: os.Stdin, out: stdout, rd: bufio.NewReader(os.Stdin)}
}

func (t *ttyInput) isTerminal() bool {
	_, err := unix.IoctlGetTermios(int(t.in.Fd()), ioctlReadTermios)
	return err == nil
}

func (t *ttyInput) line(prompt string) (string, error) {
	fmt.Fprint(t.out, prompt)
	s, err := t.rd.ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || s == "") {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimRight(s, "\r\n"), nil
}

// echoControl reads and sets the terminal's modes. Tests replace it.
type echoControl struct {
	get func() (*unix.Termios, error)
	set func(*unix.Termios) error
}

// silence turns echo off and returns a function that puts the terminal back.
// The function is safe to call more than once and from a signal handler.
func silence(c echoControl) (restore func(), err error) {
	old, err := c.get()
	if err != nil {
		return nil, fmt.Errorf("turn off echo: %w", err)
	}
	quiet := *old
	quiet.Lflag &^= unix.ECHO
	if err := c.set(&quiet); err != nil {
		return nil, fmt.Errorf("turn off echo: %w", err)
	}
	var once sync.Once
	return func() { once.Do(func() { _ = c.set(old) }) }, nil
}

func (t *ttyInput) secret(prompt string) (string, error) {
	fd := int(t.in.Fd())
	restore, err := silence(echoControl{
		get: func() (*unix.Termios, error) { return unix.IoctlGetTermios(fd, ioctlReadTermios) },
		set: func(m *unix.Termios) error { return unix.IoctlSetTermios(fd, ioctlWriteTermios, m) },
	})
	if err != nil {
		return "", err
	}
	// Ctrl-C or a kill while typing must not leave the terminal without echo.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-sig:
			restore()
			fmt.Fprintln(t.out)
			os.Exit(130)
		case <-done:
		}
	}()
	defer func() {
		close(done)
		signal.Stop(sig)
		restore()
		fmt.Fprintln(t.out)
	}()
	return t.line(prompt)
}

const usagePassword = "Usage: mooring password [-remove] [-data folder]"

// runPassword sets or removes the sign-in password. It is the way back in when
// the password is forgotten, so it asks nothing about the old one: it runs as
// you, on this computer, where the token file is already readable.
func runPassword(args []string, in passwordInput, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("mooring password", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprintln(stderr, usagePassword); flags.PrintDefaults() }
	dataDir := flags.String("data", "", "folder for the token and saved data (default: the user config folder)")
	remove := flags.Bool("remove", false, "remove the password; sign in with the token or `mooring open` again")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unknown argument %q", flags.Arg(0))
	}
	dir, err := resolveDataDir(*dataDir)
	if err != nil {
		return err
	}
	if *remove {
		if err := password.Remove(dir); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Password removed. The login page asks for the token or a sign-in link again.")
		return nil
	}
	if !in.isTerminal() {
		return errors.New("`mooring password` needs a terminal, so the password is not echoed or saved in your shell history. Run it in a terminal on this computer")
	}
	return setPassword(dir, in, stdout)
}

func setPassword(dir string, in passwordInput, stdout io.Writer) error {
	current := ""
	if rec, err := password.Load(dir); err == nil && rec != nil {
		current = rec.Username
	}
	prompt := "Username: "
	if current != "" {
		prompt = fmt.Sprintf("Username [%s]: ", current)
	}
	name, err := in.line(prompt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		name = current
	}
	pw, err := in.secret("New password: ")
	if err != nil {
		return err
	}
	again, err := in.secret("New password again: ")
	if err != nil {
		return err
	}
	if pw != again {
		return errors.New("the two passwords are not the same; nothing was saved")
	}
	rec, err := password.NewRecord(name, pw)
	if err != nil {
		return fmt.Errorf("%w; nothing was saved", err)
	}
	if err := password.Save(dir, rec); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Password saved for %s. A running Mooring uses it now.\n", rec.Username)
	fmt.Fprintln(stdout, "Browsers that are signed in stay signed in. `mooring token -new` signs them all out.")
	return nil
}
