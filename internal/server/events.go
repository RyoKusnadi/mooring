package server

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/ryokusnadi/mooring/internal/accounts"
	"github.com/ryokusnadi/mooring/internal/chat"
	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/files"
	"github.com/ryokusnadi/mooring/internal/git"
	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/lsp"
	"github.com/ryokusnadi/mooring/internal/mcp"
	"github.com/ryokusnadi/mooring/internal/password"
	"github.com/ryokusnadi/mooring/internal/projects"
	"github.com/ryokusnadi/mooring/internal/settings"
	"github.com/ryokusnadi/mooring/internal/ssh"
	"github.com/ryokusnadi/mooring/internal/terminal"
)

// Server is the HTTP handler plus the background work behind the events socket.
type Server struct {
	http.Handler
	events http.Handler
	// terminals serves the terminal sockets.
	terminals http.Handler
	terms     *terminal.Manager
	// chatSocket serves the chat message streams.
	chatSocket http.Handler
	// chatOutputs serves the raw command output saved by a chat.
	chatOutputs http.Handler
	// chatSearch finds chats in every project by their words.
	chatSearch http.Handler
	chats      *chat.Manager
	// lspSocket serves the language server relay sockets.
	lspSocket http.Handler
	lsp       *lsp.Service
	files     *files.Service
	ssh       *ssh.Manager
	git       *git.Service
	tools     *mcp.Service
	accounts  *accounts.Service
	settings  *settings.Service
	password  *password.Service
	cancel    context.CancelFunc
	ledger    *ledger.Ledger
}

// Close ends every socket and stops running commands. Commands that had not
// finished stay recorded as running, so after a restart they read as unknown.
func (s *Server) Close() {
	s.cancel()
	s.ledger.Wait()
	s.terms.Shutdown()
	s.ssh.Shutdown()
	s.chats.Shutdown()
	s.tools.Shutdown()
	s.lsp.Shutdown()
	s.files.Close()
}

func (s *Server) mountEvents(srv *server, cfg Config) error {
	if cfg.Store == nil {
		return fmt.Errorf("server needs a store")
	}
	home := cfg.Home
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find the home folder: %w", err)
		}
		home = h
	}
	hub, err := events.NewHub(cfg.Logger)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	led, err := ledger.New(ctx, cfg.Store.DB, cfg.Now, cfg.Logger)
	if err != nil {
		cancel()
		return err
	}
	svc, err := projects.New(cfg.Store.DB, hub, home, cfg.Logger)
	if err != nil {
		cancel()
		return err
	}
	s.cancel, s.ledger = cancel, led
	s.ssh, err = ssh.New(ssh.Config{
		DB: cfg.Store.DB, Pub: hub, Ledger: led, Secrets: cfg.Secrets, Home: home,
		DataDir: cfg.DataDir, Log: cfg.Logger, Context: ctx, Now: cfg.Now,
	})
	if err != nil {
		cancel()
		return err
	}
	s.terms = terminal.New(terminal.Config{
		Pub: hub, Dir: svc.Dir, Log: cfg.Logger, Context: ctx, Now: cfg.Now,
		Shell: cfg.Shell, Args: cfg.ShellArgs,
		Remote: func(connID string, cols, rows uint16) (terminal.Backend, error) {
			sh, err := s.ssh.OpenShell(connID, cols, rows)
			if err != nil {
				return nil, err
			}
			return sh, nil
		},
	})
	s.terminals = s.terms.Handler()
	s.git = git.New(git.Config{Pub: hub, Dir: svc.Dir, Log: cfg.Logger, Home: home})
	s.tools, err = mcp.New(mcp.Config{
		DB: cfg.Store.DB, Pub: hub, Secrets: cfg.Secrets, Home: home, DataDir: cfg.DataDir,
		RedirectBase: "http://" + cfg.ListenAddr, Log: cfg.Logger, Context: ctx, Now: cfg.Now,
	})
	if err != nil {
		cancel()
		return err
	}
	defs := cfg.Agents
	if defs == nil {
		defs = chat.DefaultCatalog()
	}
	adapters, names := chat.Adapters(defs)
	s.accounts, err = accounts.New(accounts.Config{
		DB: cfg.Store.DB, Pub: hub, DataDir: cfg.DataDir, Home: home, Adapters: adapters, Agents: names,
		Log: cfg.Logger, Now: cfg.Now,
	})
	if err != nil {
		cancel()
		return err
	}
	known := map[string]bool{}
	for _, d := range defs {
		known[d.ID] = true
	}
	s.settings = settings.New(settings.Config{DB: cfg.Store.DB, Pub: hub, KnownAgent: func(id string) bool { return known[id] }, Log: cfg.Logger})
	s.chats, err = chat.New(chat.Config{
		DB: cfg.Store.DB, Pub: hub, Dir: svc.Dir, Catalog: defs, Worktrees: s.git, Tools: s.tools, Accounts: s.accounts,
		DataDir: cfg.DataDir, Settings: s.settings, Staged: s.git.Staged, Uncommitted: s.git.Uncommitted,
		Log: cfg.Logger, Context: ctx, Now: cfg.Now,
	})
	if err != nil {
		cancel()
		return err
	}
	s.accounts.SetChats(s.chats)
	s.accounts.SetTerminals(s.terms)
	s.git.SetChats(chatFolders{s.chats})
	s.git.SetTerminals(s.terms)
	s.chatSocket = s.chats.Handler()
	s.chatOutputs = s.chats.OutputHandler()
	s.chatSearch = s.chats.SearchHandler()
	s.files = files.New(files.Config{
		Pub: hub, Dir: svc.Dir, Ledger: led, Log: cfg.Logger, Context: ctx, Home: home,
	})
	s.lsp, err = lsp.New(lsp.Config{
		DB: cfg.Store.DB, Pub: hub, Dir: svc.Dir, Files: s.files, DataDir: cfg.DataDir, Log: cfg.Logger,
		Context: ctx, Now: cfg.Now, Idle: cfg.LSPIdle, MemoryMB: cfg.LSPMemoryMB,
	})
	if err != nil {
		cancel()
		return err
	}
	s.lspSocket = s.lsp.Handler()
	s.password = password.NewService(password.Config{Dir: cfg.DataDir, Reserve: srv.auth.reserve, Log: cfg.Logger})
	cmds := commands{password: s.password, projects: svc, terms: s.terms, chats: s.chats, files: s.files, git: s.git, lsp: s.lsp, ssh: s.ssh, tools: s.tools, accounts: s.accounts, settings: s.settings}
	s.events = events.Handler(events.Config{
		Hub: hub, Ledger: led, Commands: cmds, Snapshot: cmds.Snapshot,
		Log: cfg.Logger, Context: ctx,
	})
	return nil
}

// requireSession refuses a request without a valid session, with 401, before
// anything else (in particular before a WebSocket upgrade).
func (s *server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorize(w, r) {
			writeError(w, http.StatusUnauthorized, "login_required")
			return
		}
		next.ServeHTTP(s.socks.wrap(w, r), r)
	})
}
