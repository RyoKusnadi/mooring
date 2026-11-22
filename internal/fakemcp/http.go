package fakemcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Stats is what the tests look at.
type Stats struct {
	Registrations int
	Refreshes     int
	Exchanges     int
	Authorizes    int
	LastState     string
	LastResource  string
	LastMethod    string // code_challenge_method of the last authorize request
	PKCEFailures  int
}

// HTTP is a streamable HTTP tool server at /mcp that needs a Bearer token, with an
// authorization server at the same address (metadata, registration, authorize
// that approves at once, token, refresh). Set Base before serving.
type HTTP struct {
	Base      string
	ExpiresIn int // seconds an access token lives; 0 means 3600
	// Iss is what the authorization server does about RFC 9207: "ok" sends the right iss,
	// "wrong" sends another one, "missing" sends none; all three advertise
	// authorization_response_iss_parameter_supported. "" sends none and advertises nothing.
	// New sets it from FAKE_MCP_ISS.
	Iss string
	// TTLms is the ttlMs of tools/list results; 0 leaves it out. New sets it from FAKE_MCP_TTL.
	TTLms int
	// MovedIssuer makes the metadata name another issuer, as if the site changed hands.
	MovedIssuer bool

	mu      sync.Mutex
	n       int
	clients map[string][]string  // client id to redirect URIs
	codes   map[string]codeGrant // authorization codes
	access  map[string]time.Time // access token to expiry
	refresh map[string]bool      // refresh tokens
	stats   Stats
	srv     *sdk.Server
}

type codeGrant struct{ challenge, redirect, client string }

// New returns an HTTP stand-in.
func New() *HTTP {
	return &HTTP{clients: map[string][]string{}, codes: map[string]codeGrant{}, access: map[string]time.Time{},
		refresh: map[string]bool{}, srv: NewServer(), Iss: os.Getenv("FAKE_MCP_ISS"), TTLms: envInt("FAKE_MCP_TTL")}
}

func (h *HTTP) issuer() string {
	if h.MovedIssuer {
		return h.Base + "/moved"
	}
	return h.Base
}

// Stats returns a copy of the counters.
func (h *HTTP) Stats() Stats {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stats
}

// RevokeAccess makes every access token fail with 401; refresh tokens still work.
func (h *HTTP) RevokeAccess() {
	h.mu.Lock()
	h.access = map[string]time.Time{}
	h.mu.Unlock()
}

// RevokeAll also forgets the refresh tokens.
func (h *HTTP) RevokeAll() {
	h.RevokeAccess()
	h.mu.Lock()
	h.refresh = map[string]bool{}
	h.mu.Unlock()
}

func rnd() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Handler serves everything.
func (h *HTTP) Handler() http.Handler {
	mux := http.NewServeMux()
	mcp := withTTL(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return h.srv },
		&sdk.StreamableHTTPOptions{JSONResponse: true}), func() int { return h.TTLms })
	verify := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		h.mu.Lock()
		exp, ok := h.access[token]
		h.mu.Unlock()
		if !ok || time.Now().After(exp) {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{Expiration: exp}, nil
	}
	mux.Handle("/mcp", auth.RequireBearerToken(verify, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: h.Base + "/.well-known/oauth-protected-resource/mcp"})(mcp))
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"resource": h.Base + "/mcp", "authorization_servers": []string{h.issuer()}})
	})
	asMeta := func(w http.ResponseWriter, _ *http.Request) {
		meta := map[string]any{
			"issuer": h.issuer(), "authorization_endpoint": h.Base + "/authorize", "token_endpoint": h.Base + "/token",
			"registration_endpoint": h.Base + "/register", "response_types_supported": []string{"code"},
			"grant_types_supported": []string{"authorization_code", "refresh_token"}, "code_challenge_methods_supported": []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{"none"},
		}
		if h.Iss == "ok" || h.Iss == "wrong" || h.Iss == "missing" {
			meta["authorization_response_iss_parameter_supported"] = true
		}
		writeJSON(w, 200, meta)
	}
	mux.HandleFunc("/.well-known/oauth-authorization-server", asMeta)
	mux.HandleFunc("/.well-known/oauth-authorization-server/moved", asMeta)
	mux.HandleFunc("POST /register", h.register)
	mux.HandleFunc("GET /authorize", h.authorize)
	mux.HandleFunc("POST /token", h.token)
	return mux
}

func (h *HTTP) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RedirectURIs []string `json:"redirect_uris"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.RedirectURIs) == 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid_client_metadata"})
		return
	}
	h.mu.Lock()
	h.n++
	id := fmt.Sprintf("client-%d", h.n)
	h.clients[id] = in.RedirectURIs
	h.stats.Registrations++
	h.mu.Unlock()
	writeJSON(w, 201, map[string]any{"client_id": id, "redirect_uris": in.RedirectURIs, "token_endpoint_auth_method": "none"})
}

func (h *HTTP) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	h.mu.Lock()
	defer h.mu.Unlock()
	uris, ok := h.clients[q.Get("client_id")]
	good := false
	for _, u := range uris {
		good = good || u == q.Get("redirect_uri")
	}
	if !ok || !good || q.Get("response_type") != "code" {
		http.Error(w, "bad request", 400)
		return
	}
	h.stats.Authorizes++
	h.stats.LastState, h.stats.LastResource, h.stats.LastMethod = q.Get("state"), q.Get("resource"), q.Get("code_challenge_method")
	code := rnd()
	h.codes[code] = codeGrant{challenge: q.Get("code_challenge"), redirect: q.Get("redirect_uri"), client: q.Get("client_id")}
	back, _ := url.Parse(q.Get("redirect_uri"))
	bq := back.Query()
	bq.Set("code", code)
	bq.Set("state", q.Get("state"))
	switch h.Iss {
	case "ok":
		bq.Set("iss", h.Base)
	case "wrong":
		bq.Set("iss", "https://other-site.example")
	}
	back.RawQuery = bq.Encode()
	http.Redirect(w, r, back.String(), http.StatusFound)
}

func (h *HTTP) token(w http.ResponseWriter, r *http.Request) {
	if r.ParseForm() != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_request"})
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		g, ok := h.codes[r.Form.Get("code")]
		delete(h.codes, r.Form.Get("code"))
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || g.redirect != r.Form.Get("redirect_uri") || g.client != r.Form.Get("client_id") ||
			base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge {
			if ok {
				h.stats.PKCEFailures++
			}
			writeJSON(w, 400, map[string]string{"error": "invalid_grant"})
			return
		}
		h.stats.Exchanges++
	case "refresh_token":
		if !h.refresh[r.Form.Get("refresh_token")] {
			writeJSON(w, 400, map[string]string{"error": "invalid_grant"})
			return
		}
		delete(h.refresh, r.Form.Get("refresh_token"))
		h.stats.Refreshes++
	default:
		writeJSON(w, 400, map[string]string{"error": "unsupported_grant_type"})
		return
	}
	life := h.ExpiresIn
	if life == 0 {
		life = 3600
	}
	at, rt := "at-"+rnd(), "rt-"+rnd()
	h.access[at] = time.Now().Add(time.Duration(life) * time.Second)
	h.refresh[rt] = true
	writeJSON(w, 200, map[string]any{"access_token": at, "token_type": "Bearer", "expires_in": life, "refresh_token": rt})
}

// ListenAndServe serves a new HTTP stand-in at addr (a host:port) until the process ends.
func ListenAndServe(addr string) error {
	h := New()
	h.Base = "http://" + addr
	return (&http.Server{Addr: addr, Handler: h.Handler(), ReadHeaderTimeout: 10 * time.Second}).ListenAndServe()
}

// Approve plays the browser at the authorize page: it fetches authURL without
// following the redirect and returns where the site sends the browser back to.
func Approve(authURL string) (string, error) {
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(authURL)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("authorize answered %d", resp.StatusCode)
	}
	return resp.Header.Get("Location"), nil
}
