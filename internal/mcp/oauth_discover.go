package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

var errSignIn = errors.New("sign-in needed")

// The SDK's discovery helpers refuse private addresses even for a server the
// user allowed, so discovery is done here (its metadata types are reused), through
// the same guarded client as every other request.
type authInfo struct {
	resource string
	scope    string
	meta     oauthex.AuthServerMeta
}

func signInFailed(format string, a ...any) error {
	return apiErr("SIGN_IN_FAILED", format, a...)
}

var errNotFound = errors.New("not found")

// getJSON fetches a JSON document of at most 1 MiB.
func getJSON(ctx context.Context, c *http.Client, raw string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("answered %d", resp.StatusCode)
	}
	body, err := readCapped(resp.Body, maxBody)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, into)
}

// probe sends an unauthenticated initialize request and returns the 401 challenges.
func probe(ctx context.Context, c *http.Client, raw string) ([]oauthex.Challenge, error) {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"mooring","version":"0"}}}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, raw, bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		return nil, signInFailed("This server did not ask for sign-in.")
	}
	cs, _ := oauthex.ParseWWWAuthenticate(resp.Header.Values("WWW-Authenticate"))
	return cs, nil
}

func wellKnown(base *url.URL, name string) string {
	u := *base
	u.Path = "/.well-known/" + name + strings.TrimSuffix(base.Path, "/")
	u.RawQuery, u.Fragment = "", ""
	return u.String()
}

func (s *Service) discover(ctx context.Context, r rec) (*authInfo, error) {
	c := newClient(r.AllowPrivate, 30*time.Second)
	cs, err := probe(ctx, c, r.URL)
	if err != nil {
		return nil, s.discoveryErr(err)
	}
	var metaURL, scope string
	for _, ch := range cs {
		if !strings.EqualFold(ch.Scheme, "bearer") {
			continue
		}
		if metaURL == "" {
			metaURL = ch.Params["resource_metadata"]
		}
		if scope == "" {
			scope = ch.Params["scope"]
		}
	}
	base, _ := url.Parse(r.URL)
	info := &authInfo{resource: r.URL, scope: scope}
	issuer := (&url.URL{Scheme: base.Scheme, Host: base.Host}).String()
	var prm oauthex.ProtectedResourceMetadata
	candidates := []string{metaURL, wellKnown(base, "oauth-protected-resource"), issuer + "/.well-known/oauth-protected-resource"}
	for _, u := range candidates {
		if u == "" || !s.addressOK(u, r) {
			continue
		}
		if err := getJSON(ctx, s.clientFor(r, u), u, &prm); err == nil && len(prm.AuthorizationServers) > 0 {
			issuer = prm.AuthorizationServers[0]
			if sameOrigin(prm.Resource, r.URL) {
				info.resource = prm.Resource
			}
			break
		} else if err != nil && isPrivateErr(err) {
			return nil, apiErr("PRIVATE_ADDRESS", "The sign-in address is on this machine or a private network.")
		}
	}
	meta, err := s.fetchASMeta(ctx, issuer, r)
	if err != nil {
		return nil, err
	}
	info.meta = *meta
	return info, nil
}

func (s *Service) discoveryErr(err error) error {
	if isPrivateErr(err) {
		return apiErr("PRIVATE_ADDRESS", "That address is on this machine or a private network. Allow private addresses for this server to use it.")
	}
	var le *ledger.Error
	if errors.As(err, &le) {
		return err
	}
	return signInFailed("Mooring could not reach the server to start the sign-in.")
}

// clientFor is the guarded client for a request to target. Private addresses are
// allowed only for the tool server's own host: a sign-in site elsewhere gets no
// such exception.
func (s *Service) clientFor(r rec, target string) *http.Client {
	return newClient(r.AllowPrivate && sameHost(target, r.URL), 30*time.Second)
}

func sameHost(a, b string) bool {
	ua, err1 := url.Parse(a)
	ub, err2 := url.Parse(b)
	return err1 == nil && err2 == nil && ua.Hostname() != "" && strings.EqualFold(ua.Hostname(), ub.Hostname())
}

func sameOrigin(a, b string) bool {
	ua, err1 := url.Parse(a)
	ub, err2 := url.Parse(b)
	return err1 == nil && err2 == nil && ua.Scheme == ub.Scheme && strings.EqualFold(ua.Host, ub.Host)
}

// addressOK is the cheap literal check for URLs a remote server hands us (the
// dialer decides for names): a web address, https unless the server is allowed private.
func (s *Service) addressOK(raw string, r rec) bool {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Hostname() == "" {
		return false
	}
	return u.Scheme == "https" || (u.Scheme == "http" && r.AllowPrivate && sameHost(raw, r.URL))
}

func (s *Service) fetchASMeta(ctx context.Context, issuer string, r rec) (*oauthex.AuthServerMeta, error) {
	iu, err := url.Parse(issuer)
	if err != nil || !s.addressOK(issuer, r) {
		return nil, signInFailed("The server named a sign-in site that Mooring cannot use.")
	}
	root := (&url.URL{Scheme: iu.Scheme, Host: iu.Host}).String()
	urls := []string{wellKnown(iu, "oauth-authorization-server"), wellKnown(iu, "openid-configuration"), strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"}
	if iu.Path == "" || iu.Path == "/" {
		urls = []string{root + "/.well-known/oauth-authorization-server", root + "/.well-known/openid-configuration"}
	}
	for _, u := range urls {
		var m oauthex.AuthServerMeta
		err := getJSON(ctx, s.clientFor(r, u), u, &m)
		if isPrivateErr(err) {
			return nil, apiErr("PRIVATE_ADDRESS", "The sign-in address is on this machine or a private network.")
		}
		if err != nil {
			continue
		}
		if strings.TrimSuffix(m.Issuer, "/") != strings.TrimSuffix(issuer, "/") {
			continue
		}
		if !contains(m.CodeChallengeMethodsSupported, "S256") {
			return nil, signInFailed("The sign-in site does not support PKCE (S256), so Mooring will not use it.")
		}
		for _, e := range []string{m.AuthorizationEndpoint, m.TokenEndpoint, m.RegistrationEndpoint} {
			if e != "" && !s.addressOK(e, r) {
				return nil, signInFailed("The sign-in site gave an address that Mooring cannot use.")
			}
		}
		if m.AuthorizationEndpoint == "" || m.TokenEndpoint == "" {
			continue
		}
		return &m, nil
	}
	return nil, signInFailed("Mooring could not find the sign-in site's settings.")
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
