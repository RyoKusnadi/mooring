package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ryokusnadi/mooring/internal/password"
	"github.com/ryokusnadi/mooring/internal/server"
	"github.com/ryokusnadi/mooring/internal/store"
)

// writeCert makes a self-signed certificate for names and returns the cert and key paths.
func writeCert(t *testing.T, names ...string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, n)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	kb, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cert, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0o600); err != nil {
		t.Fatal(err)
	}
	return cert, keyPath
}

func TestLoadTLSNeedsBothFilesAndReadsTheNames(t *testing.T) {
	cert, key := writeCert(t, "box.example", "100.64.0.5")
	if _, err := loadTLS(cert, ""); err == nil {
		t.Fatal("a certificate without a key must be refused")
	}
	if s, err := loadTLS("", ""); s != nil || err != nil {
		t.Fatalf("no files means plain HTTP, got %v %v", s, err)
	}
	s, err := loadTLS(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.names, ",") != "box.example,100.64.0.5" || s.config.MinVersion != tls.VersionTLS12 {
		t.Fatalf("names %v, min version %x", s.names, s.config.MinVersion)
	}
}

func TestOnlyLoopbackWithoutTLSAndPassword(t *testing.T) {
	defer password.UseLowCost()()
	dir := t.TempDir()
	cert, key := writeCert(t, "box.example")
	s, err := loadTLS(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkExposure("127.0.0.1:7340", dir, nil); err != nil {
		t.Fatalf("loopback needs nothing: %v", err)
	}
	if err := checkExposure("0.0.0.0:7340", dir, nil); err == nil || !strings.Contains(err.Error(), "-tls-cert") {
		t.Fatalf("no TLS: %v", err)
	}
	if err := checkExposure("0.0.0.0:7340", dir, s); err == nil || !strings.Contains(err.Error(), "mooring password") {
		t.Fatalf("no password: %v", err)
	}
	rec, err := password.NewRecord("me", "a long enough password")
	if err != nil {
		t.Fatal(err)
	}
	if err := password.Save(dir, rec); err != nil {
		t.Fatal(err)
	}
	if err := checkExposure("0.0.0.0:7340", dir, s); err != nil {
		t.Fatalf("TLS and a password: %v", err)
	}
}

func TestBaseURLUsesACertificateName(t *testing.T) {
	s := &tlsSetup{names: []string{"box.example", "127.0.0.1"}}
	cases := map[string]string{
		"0.0.0.0:7443":   "https://box.example:7443",
		"127.0.0.1:7443": "https://127.0.0.1:7443",
	}
	for addr, want := range cases {
		if got := baseURL(addr, s); got != want {
			t.Errorf("%s: got %s, want %s", addr, got, want)
		}
	}
	if got := baseURL("0.0.0.0:7443", &tlsSetup{names: []string{"*.lan.example", "pc.lan.example"}}); got != "https://pc.lan.example:7443" {
		t.Errorf("wildcard first: got %s", got)
	}
	if got := baseURL("127.0.0.1:7340", nil); got != "http://127.0.0.1:7340" {
		t.Errorf("plain: got %s", got)
	}
	if got := dialAddr("[::]:7443"); got != "127.0.0.1:7443" {
		t.Errorf("dial: got %s", got)
	}
}

// tlsServer serves Mooring over HTTPS under the name "mooring.test" and returns its address.
func tlsServer(t *testing.T, dir, token, cert, key string) (addr, base string) {
	t.Helper()
	s, err := loadTLS(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr = l.Addr().String()
	base = baseURL(addr, s)
	app, err := server.New(server.Config{ListenAddr: addr, BaseURL: base, Hosts: s.names, Token: token,
		Static: fstest.MapFS{"index.html": {Data: []byte("x")}}, Store: st, Home: t.TempDir(), DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	ts := &httptest.Server{Listener: newHandshakeListener(l, s.config, 5*time.Second), Config: &http.Server{Handler: app, ReadHeaderTimeout: time.Second}}
	go func() { _ = ts.Config.Serve(ts.Listener) }()
	t.Cleanup(func() { _ = ts.Config.Close(); app.Close(); _ = st.Close() })
	return addr, base
}

func TestOpenSignsInOverHTTPS(t *testing.T) {
	dir := t.TempDir()
	token, err := server.LoadOrCreateToken(dir)
	if err != nil {
		t.Fatal(err)
	}
	cert, key := writeCert(t, "mooring.test")
	addr, base := tlsServer(t, dir, token, cert, key)
	if base != "https://mooring.test:"+strings.Split(addr, ":")[1] {
		t.Fatalf("base %s", base)
	}
	if err := writeAddress(dir, addr, base, cert); err != nil {
		t.Fatal(err)
	}
	f := &fakeOpen{}
	var out bytes.Buffer
	if err := runOpen([]string{"-data", dir}, f.env(time.Now().Add(time.Second)), &out, &out); err != nil {
		t.Fatal(err)
	}
	if len(f.urls) != 1 || !strings.HasPrefix(f.urls[0], base+"/login?code=v1.") {
		t.Fatalf("opened %v, want a link on %s", f.urls, base)
	}
	// Naming the recorded address keeps its https address.
	if err := runOpen([]string{"-data", dir, "-addr", addr}, f.env(time.Now().Add(time.Second)), &out, &out); err != nil {
		t.Fatal(err)
	}
	if len(f.urls) != 2 || !strings.HasPrefix(f.urls[1], base+"/login?code=v1.") {
		t.Fatalf("with -addr opened %v", f.urls)
	}
	client, err := pinnedClient(&http.Client{Timeout: 2 * time.Second}, cert, base, addr)
	if err != nil {
		t.Fatal(err)
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Get(f.urls[0])
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusSeeOther || len(res.Cookies()) != 1 || !res.Cookies()[0].Secure {
		t.Fatalf("link: %d with cookies %v", res.StatusCode, res.Cookies())
	}
	if res.Header.Get("Strict-Transport-Security") == "" {
		t.Fatal("HTTPS to a name must send HSTS")
	}
}

func TestOpenRefusesAServerWithAnotherCertificate(t *testing.T) {
	dir := t.TempDir()
	token, err := server.LoadOrCreateToken(dir)
	if err != nil {
		t.Fatal(err)
	}
	cert, key := writeCert(t, "mooring.test")
	addr, base := tlsServer(t, dir, token, cert, key)
	other, _ := writeCert(t, "mooring.test")
	if err := writeAddress(dir, addr, base, other); err != nil {
		t.Fatal(err)
	}
	f := &fakeOpen{}
	var out bytes.Buffer
	if err := runOpen([]string{"-data", dir}, f.env(time.Now().Add(time.Second)), &out, &out); err == nil {
		t.Fatal("a server with another certificate must not get the sign-in code")
	}
	if len(f.urls) != 0 {
		t.Fatalf("opened %v", f.urls)
	}
}
