package mcp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Check starts a server as a chat would, lists its tools and stops it. A list that is
// still fresh (same config, within its ttl) is reused without starting anything.
func (s *Service) Check(ctx context.Context, id string) (ToolServer, error) {
	return s.check(ctx, id, false)
}

// CheckFresh is Check that ignores the saved tool list.
func (s *Service) CheckFresh(ctx context.Context, id string) (ToolServer, error) {
	return s.check(ctx, id, true)
}

func (s *Service) check(ctx context.Context, id string, fresh bool) (ToolServer, error) {
	r, gen, release, err := s.beginCheck(id)
	if err != nil {
		return ToolServer{}, err
	}
	defer release()
	key := s.cacheKey(r)
	tools, listedAt, ttl, cached := []Tool(nil), s.Now(), time.Duration(0), false
	if !fresh && key != "" {
		tools, listedAt, cached = s.cache.get(id, key, s.Now())
	}
	if !cached {
		if tools, ttl, err = s.listTools(ctx, r); err != nil {
			s.cache.drop(id)
			return ToolServer{}, s.checkFailed(id, gen, err)
		}
		listedAt = s.Now()
		if key != "" {
			s.cache.put(id, key, tools, listedAt, ttl)
		}
	}
	stale := false
	out, err := s.update(id, func(r *rec) {
		if s.gen[id] != gen {
			stale = true
			return
		}
		r.checkOK, r.CheckedAt, r.Tools = true, listedAt.UnixMilli(), tools
	})
	if err != nil {
		return ToolServer{}, err
	}
	if stale {
		return ToolServer{}, apiErr("CHECK_FAILED", "The server was changed while it was being checked. Run the check again.")
	}
	out.Cached = cached
	return out.ToolServer, nil
}

// listTools runs one real check and returns the tools with how long they stay fresh.
func (s *Service) listTools(ctx context.Context, r rec) ([]Tool, time.Duration, error) {
	timeout := s.Timeout
	if r.CheckedAt == 0 && slices.Contains([]string{"npx", "uvx", "docker"}, filepath.Base(r.Command)) {
		timeout = s.FirstTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if r.remote() {
		return s.checkRemote(ctx, r)
	}
	env, missing, err := s.values(r.ID, r.Env, envKey)
	if err == nil && len(missing) > 0 {
		err = apiErr("BAD_REQUEST", "These values are needed first: %s.", strings.Join(missing, ", "))
	}
	if err != nil {
		return nil, 0, err
	}
	return s.checkStdio(ctx, r, env)
}

// checkFailed records a failed check and turns err into what the browser gets.
func (s *Service) checkFailed(id string, gen uint64, err error) error {
	s.markFailed(id, gen)
	var le *ledger.Error
	if errors.As(err, &le) {
		return err
	}
	s.log.Error("tool server: check failed", "id", id, "err", err)
	return apiErr("CHECK_FAILED", "The check could not run.")
}

// beginCheck reads the server, notes its generation and takes a check slot: one
// check per server and four in all, else BUSY. The caller calls release.
func (s *Service) beginCheck(id string) (r rec, gen uint64, release func(), err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, err = s.get(id); err != nil {
		return r, 0, nil, err
	}
	if !r.Allowed {
		return r, 0, nil, apiErr("NOT_ALLOWED", "Allow this command before checking it.")
	}
	if s.checking[id] {
		return r, 0, nil, apiErr("BUSY", "This server is already being checked.")
	}
	if len(s.checking) >= maxChecks {
		return r, 0, nil, apiErr("BUSY", "%d checks are running already. Try again when one is done.", maxChecks)
	}
	s.checking[id] = true
	return r, s.gen[id], func() {
		s.mu.Lock()
		delete(s.checking, id)
		s.mu.Unlock()
	}, nil
}

// markFailed records a failed check, unless the server was edited since it began.
func (s *Service) markFailed(id string, gen uint64) {
	if _, err := s.update(id, func(r *rec) {
		if s.gen[id] == gen {
			r.checkOK, r.Enabled = false, false
		}
	}); err != nil {
		s.log.Warn("tool server: could not record a failed check", "err", err)
	}
}

// headerRT adds the saved headers and the sign-in token to every request and
// remembers a 401 with its challenge.
type headerRT struct {
	next    http.RoundTripper
	headers map[string]string
	token   string
	origin  string // scheme://host of the endpoint; headers and token go only there
	spy     ttlSpy

	mu        sync.Mutex
	unauth    bool
	challenge []string
}

func (h *headerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if r.URL.Scheme+"://"+strings.ToLower(r.URL.Host) == h.origin {
		for k, v := range h.headers {
			r.Header.Set(k, v)
		}
		if h.token != "" {
			r.Header.Set("Authorization", "Bearer "+h.token)
		}
	}
	resp, err := h.next.RoundTrip(r)
	if err == nil && resp.StatusCode == http.StatusUnauthorized {
		h.mu.Lock()
		h.unauth, h.challenge = true, resp.Header.Values("WWW-Authenticate")
		h.mu.Unlock()
	}
	if err == nil && resp.Body != nil {
		resp.Body = struct {
			io.Reader
			io.Closer
		}{h.spy.wrap(resp.Body), resp.Body}
	}
	return resp, err
}

func (h *headerRT) saw401() ([]string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.challenge, h.unauth
}

func (s *Service) checkRemote(ctx context.Context, r rec) ([]Tool, time.Duration, error) {
	if err := checkAddress(ctx, r.URL, r.AllowPrivate); err != nil {
		return nil, 0, err
	}
	headers, missing, err := s.values(r.ID, r.Headers, headerKey)
	if err != nil {
		return nil, 0, err
	}
	if len(missing) > 0 {
		return nil, 0, apiErr("BAD_REQUEST", "These values are needed first: %s.", strings.Join(missing, ", "))
	}
	token := ""
	if r.SignIn == "signedIn" {
		if token, err = s.accessToken(ctx, r, false); err != nil && !errors.Is(err, errSignIn) {
			return nil, 0, err
		}
	}
	for attempt := 0; ; attempt++ {
		tools, rt, err := s.listRemote(ctx, r, headers, token)
		if err == nil {
			return tools, rt.spy.ttl(), nil
		}
		challenge, unauth := rt.saw401()
		switch {
		case unauth && token != "" && attempt == 0:
			if token, err = s.accessToken(ctx, r, true); err == nil {
				continue
			}
			if !errors.Is(err, errSignIn) {
				return nil, 0, err
			}
			return nil, 0, s.needSignIn(r.ID)
		case unauth && (len(headers) == 0 || hasResourceMetadata(challenge)):
			return nil, 0, s.needSignIn(r.ID)
		case unauth:
			return nil, 0, apiErr("CHECK_FAILED", "The server refused the saved headers (401). Check the values.")
		case isPrivateErr(err):
			return nil, 0, apiErr("PRIVATE_ADDRESS", "That address is on this machine or a private network. Allow private addresses for this server to use it.")
		case ctx.Err() != nil:
			return nil, 0, apiErr("CHECK_FAILED", "The server did not answer in time.")
		}
		return nil, 0, apiErr("CHECK_FAILED", "%s", scrub("The server did not give a tool list: "+err.Error(), append(valuesOf(headers), token)))
	}
}

func valuesOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func (s *Service) listRemote(ctx context.Context, r rec, headers map[string]string, token string) ([]Tool, *headerRT, error) {
	client := newClient(r.AllowPrivate, 0)
	origin := ""
	if u, err := url.Parse(r.URL); err == nil {
		origin = u.Scheme + "://" + strings.ToLower(u.Host)
	}
	rt := &headerRT{next: capTransport{next: client.Transport, limit: maxToolListBody}, headers: headers, token: token, origin: origin}
	client.Transport = rt
	inner := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme+"://"+strings.ToLower(req.URL.Host) != origin {
			return errors.New("the server redirected to another address")
		}
		return inner(req, via)
	}
	var t sdk.Transport = &sdk.StreamableClientTransport{Endpoint: r.URL, HTTPClient: client, DisableStandaloneSSE: true}
	if r.Kind == "sse" {
		t = &sdk.SSEClientTransport{Endpoint: r.URL, HTTPClient: client}
	}
	tools, err := listToolsCtx(ctx, t)
	return tools, rt, err
}

// needSignIn marks a server as wanting sign-in and tells the caller.
func (s *Service) needSignIn(id string) error {
	if _, err := s.update(id, func(r *rec) { r.SignIn, r.Expires, r.checkOK, r.Enabled = "needed", 0, false, false }); err != nil {
		return err
	}
	return apiErr("SIGN_IN_REQUIRED", "This server asks you to sign in.")
}

// hasResourceMetadata says a 401 challenge names protected resource metadata,
// which means the server uses OAuth sign-in.
func hasResourceMetadata(challenges []string) bool {
	for _, c := range challenges {
		if strings.Contains(c, "resource_metadata") {
			return true
		}
	}
	return false
}
