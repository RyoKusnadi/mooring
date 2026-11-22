// Package server is Mooring's HTTP server: login, request checks and the web app.
package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ryokusnadi/mooring/internal/chat"
	"github.com/ryokusnadi/mooring/internal/secrets"
	"github.com/ryokusnadi/mooring/internal/store"
)

// Config is everything the server needs from its caller.
type Config struct {
	Token  string           // login token
	Static fs.FS            // built web app; index.html at the root
	Logger *slog.Logger     // nil discards logs
	Now    func() time.Time // nil uses time.Now; tests set it
	Store  *store.Store     // open database; the caller closes it after Close
	Home   string           // root for every path from the browser; "" uses the home folder

	Shell     string   // program for terminals; "" uses $SHELL
	ShellArgs []string // its arguments; nil means "-l"

	Agents []chat.AgentDef // chat agent catalog; nil uses chat.DefaultCatalog

	DataDir     string        // Mooring's data folder; language servers are installed into <DataDir>/lsp
	LSPIdle     time.Duration // a language server without sockets stops after this; 0 means 10 minutes
	LSPMemoryMB int           // a language server over this many MiB is restarted; 0 means 2048

	ListenAddr string // host:port Mooring listens on; tool server sign-in sends the browser back here
	// BaseURL is how a browser reaches Mooring ("https://name:port" under TLS);
	// "" means http://ListenAddr.
	BaseURL string
	// Hosts are the extra host names a request may be addressed to (the names in
	// the TLS certificate). Loopback names are always allowed.
	Hosts []string

	Secrets secrets.Store // remembered SSH passwords; nil remembers nothing
}

type server struct {
	auth   *auth
	limit  *limiter // failed logins per client address
	static fs.FS
	log    *slog.Logger
	// listenAddr is where this server really listens (from Config, never from a request).
	listenAddr string
	// dataDir holds the password file; "" means passwords are off.
	dataDir string
	// slots limits password checks that run at the same time.
	slots chan struct{}
	// socks holds the open WebSockets, closed when the session keys change.
	socks *sockets
}

// New returns the server. Call Close before closing the store.
func New(cfg Config) (*Server, error) {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	tokenPath := ""
	if cfg.DataDir != "" {
		tokenPath = filepath.Join(cfg.DataDir, tokenFileName)
	}
	s := &server{auth: newAuth(cfg.Token, tokenPath, cfg.Now), limit: newLimiter(cfg.Now), static: cfg.Static, log: cfg.Logger, listenAddr: cfg.ListenAddr,
		dataDir: cfg.DataDir, slots: make(chan struct{}, hashSlots), socks: newSockets()}
	s.auth.onRotate = s.socks.closeAll

	srv := &Server{}
	if err := srv.mountEvents(s, cfg); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("GET /api/events", s.requireSession(srv.events))
	mux.Handle("GET /api/terminals/{id}", s.requireSession(srv.terminals))
	mux.Handle("GET /api/chats/{id}", s.requireSession(srv.chatSocket))
	mux.Handle("GET /api/chats/{id}/outputs/{outputId}", s.requireSession(srv.chatOutputs))
	mux.Handle("GET /api/chats/search", s.requireSession(srv.chatSearch))
	mux.Handle("GET /api/lsp/{id}", s.requireSession(srv.lspSocket))
	srv.files.Mount(mux, s.requireSession)
	srv.git.Mount(mux, s.requireSession)
	srv.ssh.Mount(mux, s.requireSession)
	srv.tools.Mount(mux, s.requireSession)
	srv.accounts.Mount(mux, s.requireSession)
	srv.settings.Mount(mux, s.requireSession)
	mux.Handle("GET /api/password", s.requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handlePasswordStatus(w, r, srv.password)
	})))
	mux.HandleFunc("POST /api/password/sign-out-others", s.handleSignOutOthers)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("GET /api/login/state", s.handleLoginState)
	mux.HandleFunc("GET /login", s.handleLink)
	mux.HandleFunc("GET /api/hello", s.handleHello)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found")
	})
	mux.HandleFunc("/", s.handleStatic)
	srv.Handler = guard(cfg.Hosts, mux)
	return srv, nil
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func (s *server) sessionID(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	key := clientKey(r)
	if wait := s.limit.retryAfter(key); wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "too_many_attempts")
		return
	}
	var body struct {
		Token    string `json:"token"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	var id string
	var ok bool
	var err error
	byPassword := body.Token == ""
	if byPassword {
		id, ok, err = s.passwordLogin(r.Context(), key, body.Username, body.Password)
	} else {
		id, ok, err = s.tokenLogin(key, strings.TrimSpace(body.Token))
	}
	var blocked blockedError
	if errors.As(err, &blocked) {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(blocked.wait.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "too_many_attempts")
		return
	}
	if errors.Is(err, errPasswordFile) {
		s.log.Warn("login: password file unusable", "remote", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "password_file_unreadable")
		return
	}
	if err != nil {
		s.log.Error("login: create session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if !ok && byPassword {
		s.log.Warn("login: wrong username or password", "remote", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "wrong_credentials")
		return
	}
	if !ok {
		s.log.Warn("login: wrong token", "remote", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "wrong_token")
		return
	}
	s.setSessionCookie(w, r, id)
	s.log.Info("login: session opened", "remote", r.RemoteAddr)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleSession(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r) {
		writeError(w, http.StatusUnauthorized, "login_required")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// handleStatic serves the built web app. The app shell is public because it
// contains the login screen; everything it loads afterwards needs a session.
func (s *server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name != "" {
		if info, err := fs.Stat(s.static, name); err == nil && !info.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				// Vite puts a content hash in every file name under assets/.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				// Files outside assets/ (theme.js) have no hash in their name, so they are checked on every load.
				w.Header().Set("Cache-Control", "no-cache")
			}
			http.ServeFileFS(w, r, s.static, name)
			return
		}
		if name == "favicon.ico" {
			// Browsers ask for this on their own. Until there is a logo, answer
			// "nothing here" without an error so the console stays clean.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
	}
	if _, err := fs.Stat(s.static, "index.html"); err != nil {
		http.Error(w, "The web app is not built. Run `make build`.", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFileFS(w, r, s.static, "index.html")
}
