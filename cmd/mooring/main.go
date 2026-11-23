// Command mooring runs the Mooring server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ryokusnadi/mooring/internal/chat"
	"github.com/ryokusnadi/mooring/internal/logfile"
	"github.com/ryokusnadi/mooring/internal/secrets"
	"github.com/ryokusnadi/mooring/internal/server"
	"github.com/ryokusnadi/mooring/internal/store"
	"github.com/ryokusnadi/mooring/internal/webui"
)

const usage = `Usage:
  mooring [-addr host:port] [-data folder] [-shell program] [-lsp-idle d] [-lsp-memory-mb n] [-secrets-passphrase-file path] [-tls-cert file -tls-key file] [-quiet]
                                             run the server; a non-local -addr needs -tls-cert, -tls-key and a password
  mooring open [-addr host:port] [-data folder]   open the browser, signed in (link works once, for 5 minutes)
  mooring token [-new] [-data folder]        print the login token; -new makes a new one and signs every browser out
  mooring password [-remove] [-data folder]  set the sign-in username and password in a terminal; -remove deletes it
  mooring version                            print the version
  mooring autostart on|off|status [-addr host:port] [-data folder] [-shell program] [-secrets-passphrase-file path] [-tls-cert file -tls-key file]
                                             start Mooring at login (on takes the same flags as the server)
  mooring bench -agent claude [-account id] [-tasks all] [-modes off,on] [-budget-tokens n] [-dry -stand-in program] [-data folder] [-bench-dir folder]
                                             measure token use on a few tasks (asks before a real run)
`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "mooring:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "bench":
			return runBench(args[1:], os.Stdin, stdout, stderr)
		case "open":
			return runOpen(args[1:], realOpenEnv(), stdout, stderr)
		case "password":
			return runPassword(args[1:], realPasswordInput(stdout), stdout, stderr)
		case "version":
			fmt.Fprintln(stdout, versionLine())
			return nil
		case "autostart":
			return runAutostart(args[1:], realAutostartEnv, stdout, stderr)
		}
	}
	command := "serve"
	if len(args) > 0 && args[0] == "token" {
		command, args = "token", args[1:]
	}

	flags := flag.NewFlagSet("mooring", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usage); flags.PrintDefaults() }
	addr := flags.String("addr", "127.0.0.1:7340", "address to listen on")
	shell := flags.String("shell", "", "program to run in terminals, as a login shell (default: $SHELL)")
	lspIdle := flags.Duration("lsp-idle", 10*time.Minute, "stop a language server after this long without an editor connected")
	lspMemory := flags.Int("lsp-memory-mb", 2048, "restart a language server that uses more than this many MiB")
	dataDir := flags.String("data", "", "folder for the token and saved data (default: the user config folder)")
	secretsPass := flags.String("secrets-passphrase-file", "", "file with the passphrase that protects remembered SSH passwords (readable by you only)")
	tlsCert := flags.String("tls-cert", "", "certificate file (PEM) to serve HTTPS; needed for a non-local -addr")
	tlsKey := flags.String("tls-key", "", "private key file (PEM) for -tls-cert")
	quiet := flags.Bool("quiet", false, "print only warnings and errors to the terminal; the log file is unchanged")
	newToken := flags.Bool("new", false, "with token: make a new login token; every browser is signed out")
	fakeAccounts := addTestFlags(flags)
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
	if *newToken && command != "token" {
		return errors.New("-new works only with `mooring token`")
	}
	var token string
	if command == "token" && *newToken {
		token, err = server.ReplaceToken(dir)
	} else {
		token, err = server.LoadOrCreateToken(dir)
	}
	if err != nil {
		return err
	}
	if command == "token" {
		fmt.Fprintln(stdout, token)
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serve(ctx, serveOpts{addr: *addr, dir: dir, token: token, shell: *shell, lspIdle: *lspIdle, lspMemory: *lspMemory, secretsPass: *secretsPass, tlsCert: *tlsCert, tlsKey: *tlsKey, fakeAccounts: *fakeAccounts, quiet: *quiet, tty: isTerminal(stdout)}, stdout, stderr)
}

func resolveDataDir(flagValue string) (string, error) {
	if flagValue != "" {
		// Absolute, so folders saved under it (account folders) still work from another working directory.
		abs, err := filepath.Abs(flagValue)
		if err != nil {
			return "", fmt.Errorf("resolve the data folder %q: %w", flagValue, err)
		}
		return abs, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find the user config folder (set -data instead): %w", err)
	}
	return filepath.Join(base, "mooring"), nil
}

type serveOpts struct {
	addr, dir, token, shell string
	lspIdle                 time.Duration
	lspMemory               int
	secretsPass             string
	tlsCert, tlsKey         string
	fakeAccounts            string
	quiet                   bool
	tty                     bool // stdout is a terminal, so a sign-in link may be printed
}

// isTerminal reports whether w is the process's own stdout and it is a terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// startMessage tells how to sign in. A link carries a sign-in code, so it goes
// only to a terminal; under autostart stdout is a log file and must not get one.
func startMessage(stdout io.Writer, o serveOpts, base string, now time.Time) error {
	fmt.Fprintf(stdout, "Mooring is running at %s\n", base)
	if !o.tty {
		fmt.Fprintln(stdout, "Run `mooring open` to sign in.")
		return nil
	}
	code, err := server.NewLinkCode(o.token, now)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Sign in: %s/login?code=%s\n", base, code)
	fmt.Fprintln(stdout, "The link works once, for 5 minutes. `mooring open` makes a new one.")
	return nil
}

// serve runs until ctx ends or the server fails.
func serve(ctx context.Context, o serveOpts, stdout, stderr io.Writer) error {
	addr, dir, token, shell := o.addr, o.dir, o.token, o.shell
	tlsSet, err := loadTLS(o.tlsCert, o.tlsKey)
	if err != nil {
		return err
	}
	if err := checkExposure(addr, dir, tlsSet); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	base := baseURL(listener.Addr().String(), tlsSet)
	var hosts []string
	if tlsSet != nil {
		hosts = tlsSet.names
		listener = newHandshakeListener(listener, tlsSet.config, 10*time.Second)
	}
	logFile, err := logfile.Open(filepath.Join(dir, "logs", "mooring.log"), 10<<20, 3)
	if err != nil {
		_ = listener.Close()
		return err
	}
	defer func() { _ = logFile.Close() }()
	terminal := &slog.HandlerOptions{}
	if o.quiet {
		terminal.Level = slog.LevelWarn
	}
	logger := slog.New(slog.NewMultiHandler(slog.NewTextHandler(stderr, terminal), slog.NewJSONHandler(logFile, nil)))
	logger.Info("mooring starting", "version", versionLine(), "data", dir)
	st, err := store.Open(dir, logger)
	if err != nil {
		_ = listener.Close()
		return err
	}
	defer func() { _ = st.Close() }()
	secretStore, err := secrets.Open(secrets.Options{DataDir: dir, PassphraseFile: o.secretsPass, Log: logger})
	if err != nil {
		logger.Warn("remembering SSH passwords is off", "err", err)
		secretStore = nil
	}
	agents, err := catalog(o.fakeAccounts)
	if err != nil {
		_ = listener.Close()
		return err
	}
	app, err := server.New(server.Config{Agents: agents, ListenAddr: listener.Addr().String(), BaseURL: base, Hosts: hosts, Secrets: secretStore, Token: token, Static: webui.FS(), Logger: logger, Store: st, Shell: shell,
		DataDir: dir, LSPIdle: o.lspIdle, LSPMemoryMB: o.lspMemory})
	if err != nil {
		_ = listener.Close()
		return err
	}
	defer app.Close()
	srv := &http.Server{
		Handler:           app,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
	}

	failed := make(chan error, 1)
	go func() { failed <- srv.Serve(listener) }()

	if err := writeAddress(dir, listener.Addr().String(), base, o.tlsCert); err != nil {
		logger.Warn("could not write the address file; `mooring open` will use -addr or the default", "err", err)
	}
	defer removeAddress(dir)
	if err := startMessage(stdout, o, base, time.Now()); err != nil {
		logger.Warn("could not make a sign-in link", "err", err)
	}

	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}
	app.Close()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		return fmt.Errorf("shut down: %w", err)
	}
	return nil
}

// catalog is the default agent list. Browser-test builds (-tags e2e) can give one agent
// an account adapter on the folder variable the stand-in agent reads.
func catalog(fake string) ([]chat.AgentDef, error) {
	return withFakeAccount(chat.DefaultCatalog(), fake)
}
