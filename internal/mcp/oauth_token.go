package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// clientRecord is the registered sign-in client, kept in the secret store.
type clientRecord struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret,omitempty"`
	Redirect     string `json:"redirect"`
	Issuer       string `json:"issuer"`
}

// tokenRecord is what a sign-in leaves, kept in the secret store.
type tokenRecord struct {
	Access       string `json:"access"`
	Refresh      string `json:"refresh,omitempty"`
	Expires      int64  `json:"expires"` // unix milliseconds, 0 when the server gave none
	TokenURL     string `json:"tokenUrl"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret,omitempty"`
	Resource     string `json:"resource"`
	Issuer       string `json:"issuer,omitempty"` // the sign-in site at sign-in time; not a secret
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Error        string `json:"error"`
}

// tokenRequest posts a form to the token endpoint through the guarded client.
func (s *Service) tokenRequest(ctx context.Context, r rec, tokenURL string, form url.Values) (tokenResponse, error) {
	var tr tokenResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tr, signInFailed("The sign-in site gave an address that Mooring cannot use.")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := s.clientFor(r, tokenURL).Do(req)
	if err != nil {
		if isPrivateErr(err) {
			return tr, apiErr("PRIVATE_ADDRESS", "The sign-in address is on this machine or a private network.")
		}
		return tr, errUnreachable
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := readCapped(resp.Body, maxBody)
	if err != nil {
		return tr, errUnreachable
	}
	_ = json.Unmarshal(body, &tr)
	if resp.StatusCode == http.StatusOK && tr.AccessToken != "" {
		return tr, nil
	}
	// Only a clear "no" counts as refused. A 5xx, a 429 or a proxy page is treated like
	// an unreachable site, so saved tokens stay.
	code := oauthCode(tr.Error)
	if (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized) &&
		(code == "invalid_grant" || code == "invalid_client") {
		return tr, &refused{code: code}
	}
	return tr, errUnreachable
}

var errUnreachable = errors.New("sign-in site unreachable")

// refused is an answer from the token endpoint that says no.
type refused struct{ code string }

func (r *refused) Error() string { return "token endpoint refused: " + r.code }

// oauthCode keeps only a short error code from the sign-in site, never its text.
func oauthCode(s string) string {
	if len(s) > 40 {
		return ""
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && c != '_' {
			return ""
		}
	}
	return s
}

func (s *Service) saveToken(id string, t tokenRecord) error {
	raw, _ := json.Marshal(t)
	if err := s.Secrets.Set(tokenKey(id), string(raw)); err != nil {
		s.log.Error("tool server: could not save the sign-in token", "id", id, "err", err)
		return apiErr("SECRETS_UNAVAILABLE", "The sign-in worked but the secret store did not take the token.")
	}
	return nil
}

func (s *Service) loadToken(id string) (tokenRecord, bool) {
	raw, ok, err := s.secretGet(tokenKey(id))
	var t tokenRecord
	if err != nil || !ok || json.Unmarshal([]byte(raw), &t) != nil || t.Access == "" {
		return tokenRecord{}, false
	}
	return t, true
}

func (s *Service) lockFor(id string) *sync.Mutex {
	m, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// accessToken returns the token of a signed-in server. It refreshes it when it
// ends within 60 s, or when force is set (a request got 401). A refused refresh
// sets signIn "needed" and returns errSignIn.
func (s *Service) accessToken(ctx context.Context, r rec, force bool) (string, error) {
	l := s.lockFor(r.ID)
	l.Lock()
	defer l.Unlock()
	t, ok := s.loadToken(r.ID)
	if !ok {
		s.lostSignIn(r.ID)
		return "", errSignIn
	}
	soon := t.Expires != 0 && time.UnixMilli(t.Expires).Sub(s.Now()) < refreshAhead
	if !force && !soon {
		return t.Access, nil
	}
	if t.Refresh == "" {
		if force || soon {
			s.lostSignIn(r.ID)
			return "", errSignIn
		}
		return t.Access, nil
	}
	if err := s.sameIssuer(ctx, r, t); err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {t.Refresh}, "client_id": {t.ClientID}, "resource": {t.Resource}}
	if t.ClientSecret != "" {
		form.Set("client_secret", t.ClientSecret)
	}
	tr, err := s.tokenRequest(ctx, r, t.TokenURL, form)
	var ref *refused
	if errors.As(err, &ref) {
		s.secretDelete(tokenKey(r.ID))
		s.lostSignIn(r.ID)
		return "", errSignIn
	}
	if err != nil {
		return "", err
	}
	t.Access = tr.AccessToken
	if tr.RefreshToken != "" {
		t.Refresh = tr.RefreshToken
	}
	t.Expires = 0
	if tr.ExpiresIn > 0 {
		t.Expires = s.Now().Add(time.Duration(tr.ExpiresIn) * time.Second).UnixMilli()
	}
	if err := s.saveToken(r.ID, t); err != nil {
		return "", err
	}
	s.setExpires(r.ID, t.Expires)
	return t.Access, nil
}

// lostSignIn records that a signed-in server needs signing in again.
func (s *Service) lostSignIn(id string) {
	s.cache.drop(id)
	if _, err := s.update(id, func(r *rec) {
		r.SignIn, r.Expires, r.checkOK, r.Enabled = "needed", 0, false, false
	}); err != nil {
		s.log.Warn("tool server: could not record a lost sign-in", "id", id, "err", err)
	}
}

func (s *Service) setExpires(id string, ms int64) {
	if _, err := s.update(id, func(r *rec) { r.Expires = ms }); err != nil {
		s.log.Warn("tool server: could not record the token expiry", "id", id, "err", err)
	}
}

// SignOut forgets the tokens of a server (the registered client stays).
func (s *Service) SignOut(id string) (ToolServer, error) {
	s.pending.purge(id)
	s.cache.drop(id)
	s.notes.Delete(id)
	s.secretDelete(tokenKey(id))
	r, err := s.update(id, func(r *rec) {
		if r.remote() {
			r.SignIn = "needed"
		}
		r.Expires, r.checkOK, r.Enabled = 0, false, false
	})
	return r.ToolServer, err
}

// sameIssuer looks the sign-in site up again before a refresh token goes to it. A site
// that now names another issuer gets nothing: the saved tokens go and the user signs in
// again. A site that cannot be reached leaves the tokens alone.
func (s *Service) sameIssuer(ctx context.Context, r rec, t tokenRecord) error {
	if t.Issuer == "" {
		return nil
	}
	info, err := s.discover(ctx, r)
	if err != nil {
		var le *ledger.Error
		if errors.As(err, &le) && le.Code == "PRIVATE_ADDRESS" {
			return err
		}
		s.log.Warn("tool server: could not look the sign-in site up before a refresh", "id", r.ID, "err", err)
		return apiErr("CHECK_FAILED", "Mooring could not reach the sign-in site. The saved sign-in is kept. Check again later.")
	}
	if info.meta.Issuer == t.Issuer {
		return nil
	}
	s.log.Info("tool server: the sign-in site's issuer changed", "id", r.ID)
	s.secretDelete(tokenKey(r.ID))
	e := errIssuerChanged()
	s.setNote(r.ID, e)
	s.lostSignIn(r.ID)
	return e
}
