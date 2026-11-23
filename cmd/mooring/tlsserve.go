package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/ryokusnadi/mooring/internal/password"
	"github.com/ryokusnadi/mooring/internal/server"
)

// tlsSetup is a loaded certificate and the names it is valid for.
type tlsSetup struct {
	config *tls.Config
	names  []string // DNS names, then IP addresses, as written in the certificate
}

// loadTLS reads the certificate and key files given to -tls-cert and -tls-key.
func loadTLS(certPath, keyPath string) (*tlsSetup, error) {
	if (certPath == "") != (keyPath == "") {
		return nil, errors.New("-tls-cert and -tls-key go together; give both or neither")
	}
	if certPath == "" {
		return nil, nil
	}
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load the TLS certificate and key: %w", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("read the TLS certificate: %w", err)
	}
	names := append([]string{}, leaf.DNSNames...)
	for _, ip := range leaf.IPAddresses {
		names = append(names, ip.String())
	}
	if len(names) == 0 {
		return nil, errors.New("the TLS certificate names no host (no subject alternative names)")
	}
	return &tlsSetup{config: &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}, names: names}, nil
}

// checkExposure enforces the rule for listening beyond this machine: only over
// TLS, so the cookie never crosses the network in clear text, and only with a
// password, the sign-in meant for another device.
func checkExposure(addr, dataDir string, t *tlsSetup) error {
	if server.IsLoopbackHost(addr) {
		return nil
	}
	if t == nil {
		return fmt.Errorf("-addr %s is not a local address; to open Mooring from other devices give -tls-cert and -tls-key", addr)
	}
	rec, err := password.Load(dataDir)
	if err != nil {
		return fmt.Errorf("read the password file: %w", err)
	}
	if rec == nil {
		return fmt.Errorf("-addr %s is not a local address; set a password first with `mooring password`", addr)
	}
	return nil
}

// baseURL is the address a browser should use. Under TLS the host is the
// listening host when the certificate names it, else the certificate's first
// name that is not a wildcard.
func baseURL(listenAddr string, t *tlsSetup) string {
	if t == nil {
		return "http://" + listenAddr
	}
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "https://" + listenAddr
	}
	name := ""
	for _, n := range t.names {
		if strings.HasPrefix(n, "*") {
			continue // a pattern, not an address a browser can open
		}
		if name == "" || strings.EqualFold(n, host) {
			name = n
		}
	}
	if name == "" {
		name = host
		if isUnspecified(host) {
			name = "localhost"
		}
	}
	return "https://" + net.JoinHostPort(name, port)
}

// isUnspecified reports whether host is 0.0.0.0, ::, or empty (every address).
func isUnspecified(host string) bool {
	if host == "" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsUnspecified()
}

// dialAddr is where this machine reaches a server listening on listenAddr.
func dialAddr(listenAddr string) string {
	host, port, err := net.SplitHostPort(listenAddr)
	if err == nil && isUnspecified(host) {
		return net.JoinHostPort("127.0.0.1", port)
	}
	return listenAddr
}

// pinnedClient talks HTTPS to this machine's server: it dials dial whatever the
// name in base resolves to, and trusts only the certificate in certPath, so a
// self-made certificate works and no other one does.
func pinnedClient(c *http.Client, certPath, base, dial string) (*http.Client, error) {
	pem, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read the TLS certificate %s: %w", certPath, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no certificate in %s", certPath)
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("read the address file: %w", err)
	}
	var d net.Dialer
	out := *c
	out.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: u.Hostname(), MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return d.DialContext(ctx, network, dial)
		},
	}
	return &out, nil
}
