package fakessh

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/ssh"
)

// Run starts a server from environment variables and waits for a signal. The
// e2e starts it through a test binary with MOORING_FAKE_SSH=1:
//
//	FAKE_SSH_ADDR       listen address, default 127.0.0.1:0 (the address is printed)
//	FAKE_SSH_HOSTKEY    host key file, made when missing
//	FAKE_SSH_CLIENTKEY  client key file, made when missing; its public half may sign in
//	FAKE_SSH_USER, FAKE_SSH_PASSWORD, FAKE_SSH_HOME, FAKE_SSH_SHELL ("sh" for a real shell)
//	FAKE_SSH_QUESTION   when set, keyboard-interactive asks "Code:" and wants this answer
func Run() error {
	o := Options{
		Addr: os.Getenv("FAKE_SSH_ADDR"), User: os.Getenv("FAKE_SSH_USER"),
		Password: os.Getenv("FAKE_SSH_PASSWORD"), Home: os.Getenv("FAKE_SSH_HOME"),
		Shell: os.Getenv("FAKE_SSH_SHELL"),
	}
	if o.Home == "" {
		o.Home, _ = os.Getwd()
	}
	if p := os.Getenv("FAKE_SSH_HOSTKEY"); p != "" {
		k, err := LoadOrCreateKey(p)
		if err != nil {
			return err
		}
		o.HostKey = k
	}
	if p := os.Getenv("FAKE_SSH_CLIENTKEY"); p != "" {
		k, err := LoadOrCreateKey(p)
		if err != nil {
			return err
		}
		o.AuthorizedKeys = []ssh.PublicKey{k.PublicKey()}
		pub := ssh.MarshalAuthorizedKey(k.PublicKey())
		if err := os.WriteFile(filepath.Clean(p)+".pub", pub, 0o644); err != nil {
			return fmt.Errorf("fakessh: write the public key: %w", err)
		}
	}
	if q := os.Getenv("FAKE_SSH_QUESTION"); q != "" {
		o.Questions = []Question{{Text: "Code: ", Answer: q}}
	}
	s, err := Start(o)
	if err != nil {
		return err
	}
	defer s.Close()
	fmt.Println("fake ssh listening on", s.Addr())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	return nil
}
