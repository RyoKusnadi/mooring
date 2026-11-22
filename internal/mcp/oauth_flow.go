package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const callbackPath = "/api/toolservers/callback"

// pendingSignIn is a sign-in that was started and waits for the browser to come
// back. It lives in memory only; its state is single use and lasts 10 minutes.
type pendingSignIn struct {
	id, url, verifier, redirect, tokenURL, clientID, clientSecret, resource string
	// issuer is the metadata issuer recorded before the browser left; issAdvertised is the
	// metadata's authorization_response_iss_parameter_supported.
	issuer        string
	issAdvertised bool
	created       time.Time
}

type pendingSignIns struct {
	mu sync.Mutex
	m  map[string]*pendingSignIn
}

func (p *pendingSignIns) add(state string, v *pendingSignIn, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, o := range p.m {
		if now.Sub(o.created) > signInLife {
			delete(p.m, k)
		}
	}
	for len(p.m) >= maxPending {
		oldest := ""
		for k, o := range p.m {
			if oldest == "" || o.created.Before(p.m[oldest].created) {
				oldest = k
			}
		}
		delete(p.m, oldest)
	}
	v.created = now
	p.m[state] = v
}

// purge forgets the waiting sign-ins of one server.
func (p *pendingSignIns) purge(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, v := range p.m {
		if v.id == id {
			delete(p.m, k)
		}
	}
}

// take removes and returns the sign-in of a state: a state works once.
func (p *pendingSignIns) take(state string, now time.Time) *pendingSignIn {
	p.mu.Lock()
	defer p.mu.Unlock()
	v := p.m[state]
	delete(p.m, state)
	if v == nil || now.Sub(v.created) > signInLife {
		return nil
	}
	return v
}

func randomB64(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// SignIn starts a sign-in and returns the page to open and the address the site sends the browser back to.
func (s *Service) SignIn(ctx context.Context, id string) (authURL, redirect string, err error) {
	if s.Secrets == nil {
		return "", "", apiErr("SECRETS_UNAVAILABLE", "There is no secret store on this machine, so a sign-in cannot be kept.")
	}
	if s.RedirectBase == "" {
		return "", "", apiErr("INTERNAL", "Mooring does not know its own address.")
	}
	r, err := s.get(id)
	if err != nil {
		return "", "", err
	}
	s.notes.Delete(id)
	if r.SignIn == "failed" {
		s.setSignIn(id, "needed")
	}
	if !r.remote() {
		return "", "", apiErr("BAD_REQUEST", "Only a remote server can be signed in to.")
	}
	if !r.Allowed {
		return "", "", apiErr("NOT_ALLOWED", "Allow this address first.")
	}
	if err := checkAddress(ctx, r.URL, r.AllowPrivate); err != nil {
		return "", "", err
	}
	redirect = strings.TrimSuffix(s.RedirectBase, "/") + callbackPath
	info, err := s.discover(ctx, r)
	if err != nil {
		return "", "", err
	}
	cl, err := s.registeredClient(ctx, r, info, redirect)
	if err != nil {
		return "", "", err
	}
	verifier, state := randomB64(32), randomB64(32)
	sum := sha256.Sum256([]byte(verifier))
	u, err := url.Parse(info.meta.AuthorizationEndpoint)
	if err != nil {
		return "", "", signInFailed("The sign-in site gave an address that Mooring cannot use.")
	}
	q := u.Query()
	for k, v := range map[string]string{
		"response_type": "code", "client_id": cl.ClientID, "redirect_uri": redirect, "state": state,
		"code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]), "code_challenge_method": "S256", "resource": info.resource,
	} {
		q.Set(k, v)
	}
	if info.scope != "" {
		q.Set("scope", info.scope)
	}
	u.RawQuery = q.Encode()
	s.pending.add(state, &pendingSignIn{id: id, url: r.URL, verifier: verifier, redirect: redirect, tokenURL: info.meta.TokenEndpoint,
		clientID: cl.ClientID, clientSecret: cl.ClientSecret, resource: info.resource,
		issuer: info.meta.Issuer, issAdvertised: info.meta.AuthorizationResponseIssParameterSupported}, s.Now())
	return u.String(), redirect, nil
}

// registeredClient returns the saved client for this site and redirect, or registers a new one.
func (s *Service) registeredClient(ctx context.Context, r rec, info *authInfo, redirect string) (clientRecord, error) {
	if raw, ok, err := s.secretGet(clientKey(r.ID)); err == nil && ok {
		var c clientRecord
		if json.Unmarshal([]byte(raw), &c) == nil && c.ClientID != "" && c.Redirect == redirect && c.Issuer == info.meta.Issuer {
			return c, nil
		}
	}
	if info.meta.RegistrationEndpoint == "" {
		return clientRecord{}, signInFailed("The sign-in site does not take new apps, and Mooring has no client id for it.")
	}
	reg, err := oauthex.RegisterClient(ctx, info.meta.RegistrationEndpoint, &oauthex.ClientRegistrationMetadata{
		RedirectURIs: []string{redirect}, TokenEndpointAuthMethod: "none", GrantTypes: []string{"authorization_code", "refresh_token"},
		ResponseTypes: []string{"code"}, ClientName: "Mooring",
	}, s.clientFor(r, info.meta.RegistrationEndpoint))
	if err != nil {
		if isPrivateErr(err) {
			return clientRecord{}, apiErr("PRIVATE_ADDRESS", "The sign-in address is on this machine or a private network.")
		}
		s.log.Info("tool server: client registration failed", "id", r.ID)
		return clientRecord{}, signInFailed("The sign-in site did not accept Mooring as an app.")
	}
	c := clientRecord{ClientID: reg.ClientID, ClientSecret: reg.ClientSecret, Redirect: redirect, Issuer: info.meta.Issuer}
	raw, _ := json.Marshal(c)
	if err := s.Secrets.Set(clientKey(r.ID), string(raw)); err != nil {
		return clientRecord{}, apiErr("SECRETS_UNAVAILABLE", "The secret store did not take the sign-in client.")
	}
	return c, nil
}

// finish swaps the code of a sign-in for tokens and records the result. wantID, when
// set, must be the server the state was made for.
func (s *Service) finish(ctx context.Context, q url.Values, wantID string) (ToolServer, error) {
	state, code, authErr := q.Get("state"), q.Get("code"), q.Get("error")
	p := s.pending.take(state, s.Now())
	if p == nil || (wantID != "" && p.id != wantID) {
		return ToolServer{}, signInFailed("This sign-in link is not valid any more. Start the sign-in again.")
	}
	if err := checkIss(q, p.issuer, p.issAdvertised); err != nil {
		s.log.Info("tool server: sign-in reply had a wrong or missing issuer", "id", p.id)
		s.setNote(p.id, err)
		s.setSignIn(p.id, "failed")
		return ToolServer{}, err
	}
	r, err := s.get(p.id)
	if err != nil {
		return ToolServer{}, signInFailed("The tool server was removed.")
	}
	if r.URL != p.url {
		return ToolServer{}, signInFailed("The server's address changed. Start the sign-in again.")
	}
	if authErr != "" || code == "" {
		s.setSignIn(p.id, "failed")
		return ToolServer{}, signInFailed("The sign-in was not completed.")
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {p.redirect},
		"client_id": {p.clientID}, "code_verifier": {p.verifier}, "resource": {p.resource}}
	if p.clientSecret != "" {
		form.Set("client_secret", p.clientSecret)
	}
	tr, err := s.tokenRequest(ctx, r, p.tokenURL, form)
	if err != nil {
		s.setSignIn(p.id, "failed")
		var ref *refused
		switch {
		case errors.As(err, &ref):
			return ToolServer{}, signInFailed("The sign-in site did not accept the code.")
		case errors.Is(err, errUnreachable):
			return ToolServer{}, signInFailed("Mooring could not reach the sign-in site.")
		}
		return ToolServer{}, err
	}
	t := tokenRecord{Access: tr.AccessToken, Refresh: tr.RefreshToken, TokenURL: p.tokenURL, ClientID: p.clientID,
		ClientSecret: p.clientSecret, Resource: p.resource, Issuer: p.issuer}
	if tr.ExpiresIn > 0 {
		t.Expires = s.Now().Add(time.Duration(tr.ExpiresIn) * time.Second).UnixMilli()
	}
	if err := s.saveToken(p.id, t); err != nil {
		s.setSignIn(p.id, "failed")
		return ToolServer{}, err
	}
	s.cache.drop(p.id)
	s.notes.Delete(p.id)
	out, err := s.update(p.id, func(r *rec) { r.SignIn, r.Expires = "signedIn", t.Expires })
	return out.ToolServer, err
}

func (s *Service) setSignIn(id, state string) {
	if _, err := s.update(id, func(r *rec) { r.SignIn = state }); err != nil {
		s.log.Warn("tool server: could not record the sign-in state", "id", id, "err", err)
	}
}

// SignInFinish finishes a sign-in from the address the user pasted.
func (s *Service) SignInFinish(ctx context.Context, id, pasted string) (ToolServer, error) {
	if len(pasted) > 8<<10 {
		return ToolServer{}, apiErr("BAD_REQUEST", "That address is too long.")
	}
	u, err := url.Parse(strings.TrimSpace(pasted))
	if err != nil || u.RawQuery == "" {
		return ToolServer{}, apiErr("BAD_REQUEST", "Paste the whole address from the browser's address bar.")
	}
	return s.finish(ctx, u.Query(), id)
}

// Callback is the browser's return from the sign-in site. It needs no session
// (the cookie is not sent on this cross-site redirect): the state decides.
func (s *Service) Callback(w http.ResponseWriter, r *http.Request) {
	_, err := s.finish(r.Context(), r.URL.Query(), "")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	msg, status := "Sign-in is done. You can close this tab and go back to Mooring.", http.StatusOK
	if err != nil {
		msg, status = "This sign-in could not be finished. Go back to Mooring and start it again.", http.StatusBadRequest
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>Mooring</title><body style="font:16px system-ui;margin:3rem"><p>` + msg + `</p>`))
}
