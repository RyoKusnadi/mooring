package main

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ryokusnadi/mooring/internal/server"
)

const (
	defaultAddr     = "127.0.0.1:7340"
	addressFileName = "address"
)

// openEnv is what `mooring open` needs from the machine. Tests replace it.
type openEnv struct {
	open   func(url string) error // opens the default browser
	now    func() time.Time
	client *http.Client
}

func realOpenEnv() openEnv {
	return openEnv{open: openBrowser, now: time.Now, client: &http.Client{Timeout: 2 * time.Second}}
}

func openBrowser(url string) error {
	var name string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "linux":
		name = "xdg-open"
	default:
		return fmt.Errorf("no way to open a browser on %s", runtime.GOOS)
	}
	cmd := exec.Command(name, url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// writeAddress records where this server listens, for `mooring open`.
func writeAddress(dir, addr string) error {
	tmp, err := os.CreateTemp(dir, "address-new-*") // created 0600
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // gone already after a successful rename
	if _, err := tmp.WriteString(addr + "\n"); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, addressFileName))
}

func removeAddress(dir string) { _ = os.Remove(filepath.Join(dir, addressFileName)) }

// runOpen makes a single-use sign-in link and opens it in the browser.
func runOpen(args []string, env openEnv, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("mooring open", flag.ContinueOnError)
	flags.SetOutput(stderr)
	addrFlag := flags.String("addr", "", "address of the running server (default: read from the data folder, else "+defaultAddr+")")
	dataFlag := flags.String("data", "", "folder with the token (default: the user config folder)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unknown argument %q", flags.Arg(0))
	}
	dir, err := resolveDataDir(*dataFlag)
	if err != nil {
		return err
	}
	addr := *addrFlag
	if addr == "" {
		addr = defaultAddr
		if data, err := os.ReadFile(filepath.Join(dir, addressFileName)); err == nil && strings.TrimSpace(string(data)) != "" {
			addr = strings.TrimSpace(string(data))
		}
	}
	// The link carries a sign-in code, so it only ever goes to this machine.
	if !server.IsLoopbackHost(addr) {
		return fmt.Errorf("%s is not a local address", addr)
	}
	token, err := server.LoadOrCreateToken(dir)
	if err != nil {
		return err
	}
	if err := verifyServer(env.client, addr, token); err != nil {
		return err
	}
	code, err := server.NewLinkCode(token, env.now())
	if err != nil {
		return err
	}
	link := "http://" + addr + "/login?code=" + code
	if err := env.open(link); err != nil {
		fmt.Fprintln(stdout, "Could not open a browser. Open this link within 5 minutes; it works once:")
		fmt.Fprintln(stdout, link)
		return nil
	}
	fmt.Fprintln(stdout, "Opened Mooring in your browser.")
	return nil
}

// verifyServer proves that addr is a Mooring server for this token, by
// checking its answer to a random nonce. A stale address file, or another
// program on that port, fails here before any sign-in code exists.
func verifyServer(client *http.Client, addr, token string) error {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("read random bytes: %w", err)
	}
	nonce := hex.EncodeToString(raw)
	// Never follow a redirect: the answer must come from addr itself.
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := c.Get("http://" + addr + "/api/hello?nonce=" + nonce)
	if err != nil {
		return fmt.Errorf("Mooring is not running at %s. Start it with `mooring`, or turn on autostart with `mooring autostart on`.", addr) //nolint:staticcheck // a sentence shown to the user
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 256))
	want := server.HelloProof(token, nonce, addr)
	if res.StatusCode != http.StatusOK || !hmac.Equal(body, []byte(want)) {
		return fmt.Errorf("Something else is answering at %s; is Mooring running?", addr) //nolint:staticcheck // a sentence shown to the user
	}
	return nil
}
