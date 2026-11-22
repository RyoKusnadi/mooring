package ssh

import (
	"crypto/ed25519"
	"errors"
	"net"
	"os"
	"strconv"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// source says which file holds a key for the host: "mooring" (checked first), "user" or "".
func (k *hostKeys) source(address string, port int) string {
	addr := net.JoinHostPort(address, strconv.Itoa(port))
	for _, f := range []struct{ name, path string }{{"mooring", k.own}, {"user", k.user}} {
		if f.path == "" {
			continue
		}
		if _, err := os.Stat(f.path); err != nil {
			continue
		}
		cb, err := knownhosts.New(f.path)
		if err != nil {
			continue
		}
		// A key that cannot match shows whether the host is listed at all.
		var ke *knownhosts.KeyError
		var re *knownhosts.RevokedError
		err = cb(addr, &net.TCPAddr{IP: net.IPv4zero}, probeKey)
		if errors.As(err, &re) || (errors.As(err, &ke) && len(ke.Want) > 0) {
			return f.name
		}
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, s := range k.session {
		if knownhosts.Normalize(s.addr) == knownhosts.Normalize(addr) {
			return "mooring"
		}
	}
	return ""
}

// probeKey is a key no server has.
var probeKey = func() gossh.PublicKey {
	k, err := gossh.NewPublicKey(ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)))
	if err != nil {
		panic(err)
	}
	return k
}()
