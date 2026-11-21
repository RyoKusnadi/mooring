// Package accounts keeps the logins of each agent. An account is a settings
// folder for the agent's own program; Mooring starts the agent with the folder
// variable pointed at it and never reads or stores a login token. What it
// learns about an account (signed in or not, usage, limits) comes from the chats
// that ran on it.
package accounts

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Adapter is the per-agent part: how to point the agent at a folder, what to tell
// the user to sign in, which session folder is shared, and which usage reader
// understands the agent.
type Adapter struct {
	Env    string // variable that names the agent's settings folder
	Login  string // hint shown in the sign-in terminal
	Shared string // session folder inside the settings folder; "" shares nothing
	Usage  string // usage reader: "claude", or "" for none
	Home   string // the built-in login's folder under the home folder, such as ".claude"
}

// Login states.
const (
	LoginUnknown  = "unknown"
	LoginSignedIn = "signedIn"
	LoginNeeded   = "needed"
)

// Limits.
const (
	MaxPerAgent = 20
	maxName     = 40
)

// Account is what the browser sees.
type Account struct {
	ID        string `json:"id"`
	Agent     string `json:"agent"`
	Name      string `json:"name"`
	Dir       string `json:"dir"`
	Managed   bool   `json:"managed"`
	IsDefault bool   `json:"isDefault"`
	Login     string `json:"login"`
	Usage     Usage  `json:"usage"`
}

// Publisher receives the events accounts produce.
type Publisher interface {
	Publish(kind string, data any)
}

// Chats is the part of the chat manager accounts need.
type Chats interface {
	// LiveOn counts the chats whose agent is running on the account.
	LiveOn(accountID string) int
	// MoveAccount points the chats of a project and agent at another account and
	// restarts the open ones. It fails with BUSY while one is answering.
	MoveAccount(projectID, agent, accountID string) (restarted []string, err error)
}

// Terminals opens the sign-in terminal.
type Terminals interface {
	OpenWith(projectID, id string, env []string, banner string) error
}

// Config is what a Service needs.
type Config struct {
	DB       *sql.DB
	Pub      Publisher
	DataDir  string             // managed folders live in <DataDir>/accounts
	Home     string             // the user's home folder: the root for folders from the browser
	Adapters map[string]Adapter // by agent id; an agent without one has only its own login
	Agents   map[string]string  // agent id to display name
	Log      *slog.Logger
	Now      func() time.Time
}

// Service owns accounts.
type Service struct {
	Config
	log *slog.Logger
	now func() time.Time

	mu        sync.Mutex      // guards the account tables and their events
	rmMu      sync.Mutex      // guards removing
	removing  map[string]bool // accounts being removed: no chat may start on them
	chats     Chats
	terminals Terminals
}

// New returns a service. SetChats and SetTerminals connect the rest.
func New(cfg Config) (*Service, error) {
	if cfg.DB == nil || cfg.Pub == nil {
		return nil, fmt.Errorf("accounts need a database and a publisher")
	}
	s := &Service{Config: cfg, log: cfg.Log, now: cfg.Now}
	if s.log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s, nil
}

// SetChats connects the chat manager.
func (s *Service) SetChats(c Chats) { s.chats = c }

// SetTerminals connects the terminal manager.
func (s *Service) SetTerminals(t Terminals) { s.terminals = t }

func apiErr(code, format string, a ...any) *ledger.Error {
	return &ledger.Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

// Has reports whether the agent has an adapter, so more than one login.
func (s *Service) Has(agent string) bool {
	_, ok := s.Adapters[agent]
	return ok
}

// BuiltinID is the id of an agent's own login.
func BuiltinID(agent string) string { return "default:" + agent }

func (s *Service) agentName(agent string) string {
	if n := s.Agents[agent]; n != "" {
		return n
	}
	return agent
}

// builtinDir is the folder the agent uses when Mooring sets nothing: the folder
// variable if Mooring itself has it, else the usual folder under home.
func (s *Service) builtinDir(a Adapter) string {
	if v := os.Getenv(a.Env); v != "" && filepath.IsAbs(v) {
		return filepath.Clean(v)
	}
	if a.Home == "" {
		return ""
	}
	return filepath.Join(s.Home, a.Home)
}

func (s *Service) setRemoving(id string, on bool) {
	s.rmMu.Lock()
	defer s.rmMu.Unlock()
	if s.removing == nil {
		s.removing = map[string]bool{}
	}
	if on {
		s.removing[id] = true
	} else {
		delete(s.removing, id)
	}
}

func (s *Service) isRemoving(id string) bool {
	s.rmMu.Lock()
	defer s.rmMu.Unlock()
	return s.removing[id]
}
