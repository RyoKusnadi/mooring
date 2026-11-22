package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Caps from the contract.
const (
	maxBody          = 1 << 20
	maxToolListBody  = 4 << 20
	maxRedirects     = 5
	maxRegistryBytes = 20 << 20
)

// errPrivate is what the dialer returns for an address Mooring may not contact.
var errPrivate = errors.New("address is on this machine or a private network")

var blockedNets = mustPrefixes(
	"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "198.18.0.0/15", "240.0.0.0/4", "64:ff9b::/96", "fc00::/7", "fe80::/10",
	"::/96", "2002::/16", "2001::/32", "64:ff9b:1::/48",
)

func mustPrefixes(list ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(list))
	for i, s := range list {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

// blocked reports whether ip is loopback, private, link-local, CGNAT,
// unspecified, multicast, broadcast or otherwise not a public address.
func blocked(ip netip.Addr) bool {
	ip = ip.Unmap().WithZone("")
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, p := range blockedNets {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// guardControl runs inside the dialer, after the name was resolved and before
// connecting, so a name that changes its answer between a check and the connect
// cannot get around it.
func guardControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("read dial address: %w", err)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("dial address is not an IP: %w", err)
	}
	if blocked(ip) {
		return errPrivate
	}
	return nil
}

// newClient is the one HTTP client for tool-server addresses. Unless
// allowPrivate, every connection (including redirects) goes through guardControl.
// It never uses a proxy: the dialer must see the real address.
func newClient(allowPrivate bool, timeout time.Duration) *http.Client {
	d := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	if !allowPrivate {
		d.Control = guardControl
	}
	tr := &http.Transport{
		Proxy: nil, DialContext: d.DialContext, TLSHandshakeTimeout: 15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second, MaxResponseHeaderBytes: 64 << 10, DisableCompression: false,
	}
	return &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: redirectCheck(allowPrivate)}
}

// redirectCheck allows at most maxRedirects, never from https to http, and
// refuses a redirect to a private address literal at once (the dialer catches names).
func redirectCheck(allowPrivate bool) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return fmt.Errorf("more than %d redirects", maxRedirects)
		}
		if req.URL.Scheme != "https" && req.URL.Scheme != "http" {
			return fmt.Errorf("redirect to %q is not allowed", req.URL.Scheme)
		}
		if prev := via[len(via)-1]; prev.URL.Scheme == "https" && req.URL.Scheme == "http" {
			return errors.New("redirect from https to http")
		}
		if !allowPrivate {
			if ip, err := netip.ParseAddr(req.URL.Hostname()); err == nil && blocked(ip) {
				return errPrivate
			}
		}
		return nil
	}
}

// isPrivateErr tells a refusal by the dialer, even when an error was flattened to text.
func isPrivateErr(err error) bool {
	return err != nil && (errors.Is(err, errPrivate) || strings.Contains(err.Error(), errPrivate.Error()))
}

// checkAddress resolves the host of raw and refuses it with PRIVATE_ADDRESS when
// any address is not public, unless allowPrivate. A plain http URL is allowed
// only for an allowed private address. A name that does not resolve is refused,
// so the dialer is never the only guard for a name that failed here.
func checkAddress(ctx context.Context, raw string, allowPrivate bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return apiErr("BAD_REQUEST", "The address must be a web address that starts with https://.")
	}
	ips, err := lookup(ctx, u.Hostname())
	if err != nil || len(ips) == 0 {
		return apiErr("BAD_REQUEST", "Mooring could not look up %q. Check the name and the network connection, then try again.", u.Hostname())
	}
	private := true
	for _, ip := range ips {
		if blocked(ip) {
			if !allowPrivate {
				return apiErr("PRIVATE_ADDRESS", "That address is on this machine or a private network. Allow private addresses for this server to use it.")
			}
		} else {
			private = false
		}
	}
	if u.Scheme == "http" && (!allowPrivate || !private) {
		return apiErr("BAD_REQUEST", "A plain http:// address works only for a private address that you allow.")
	}
	return nil
}

func lookup(ctx context.Context, host string) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		return []netip.Addr{ip}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

// capReader fails with errTooBig instead of silently cutting a body.
type capReader struct {
	r    io.Reader
	left int64
}

var errTooBig = errors.New("response is too large")

func (c *capReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		return 0, errTooBig
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}

// readCapped reads a whole body of at most limit bytes.
func readCapped(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(&capReader{r: r, left: limit + 1})
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errTooBig
	}
	return b, nil
}

// capTransport limits what a response body can deliver.
type capTransport struct {
	next  http.RoundTripper
	limit int64
}

func (t capTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	resp.Body = struct {
		io.Reader
		io.Closer
	}{&capReader{r: resp.Body, left: t.limit}, resp.Body}
	return resp, nil
}
