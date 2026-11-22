package password

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/ledger"
)

// Status is what the Settings page shows. It never holds the password.
type Status struct {
	Set      bool   `json:"set"`
	Username string `json:"username"`
}

// Config is what a Service needs.
type Config struct {
	Dir string // the data folder
	// Reserve counts one attempt before it is checked and returns how long
	// attempts are paused when there were too many (then it counts nothing).
	// release takes the attempt back after a success. It is shared with the
	// login page, so guessing here costs the same.
	Reserve func() (release func(), wait time.Duration)
	Log     *slog.Logger
}

// Service sets, changes and removes the password for the Settings commands.
type Service struct {
	Config
	mu sync.Mutex // one change at a time
}

// NewService returns the service.
func NewService(cfg Config) *Service {
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{Config: cfg}
}

// Handles reports whether name is a password command.
func Handles(name string) bool { return name == "password.set" || name == "password.remove" }

// Secret is true for both commands: their args hold passwords.
func Secret(name string) bool { return Handles(name) }

func apiErr(code, msg string) *ledger.Error { return &ledger.Error{Code: code, Message: msg} }

// Status reads the current state.
func (s *Service) Status() (Status, error) {
	if s.Dir == "" {
		return Status{}, nil
	}
	r, err := Load(s.Dir)
	if err != nil {
		return Status{}, err
	}
	if r == nil {
		return Status{}, nil
	}
	return Status{Set: true, Username: r.Username}, nil
}

type args struct {
	Current  string `json:"current"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Run runs password.set or password.remove. Errors are *ledger.Error.
func (s *Service) Run(cmd events.Command) (any, error) {
	var a args
	if err := json.Unmarshal(cmd.Args, &a); err != nil {
		return nil, apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	if s.Dir == "" {
		return nil, apiErr("BAD_REQUEST", "This server has no data folder, so it cannot keep a password.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := Load(s.Dir)
	if err != nil {
		s.Log.Error("password: read file", "err", err)
		return nil, apiErr("INTERNAL", "Could not read the saved password. Run mooring password -remove in a terminal.")
	}
	var next *Record
	if cmd.Name == "password.set" {
		// Check the new values first, so a typo does not use up a guess.
		if next, err = NewRecord(a.Username, a.Password); err != nil {
			return nil, apiErr("BAD_REQUEST", capital(err.Error())+".")
		}
	}
	if cur != nil {
		if e := s.checkCurrent(cur, a.Current); e != nil {
			return nil, e
		}
	}
	if next == nil {
		if err := Remove(s.Dir); err != nil {
			s.Log.Error("password: remove", "err", err)
			return nil, apiErr("INTERNAL", "Could not remove the password.")
		}
		s.Log.Info("password: removed")
		return Status{}, nil
	}
	if err := Save(s.Dir, next); err != nil {
		s.Log.Error("password: save", "err", err)
		return nil, apiErr("INTERNAL", "Could not save the password.")
	}
	s.Log.Info("password: saved")
	return Status{Set: true, Username: next.Username}, nil
}

func (s *Service) checkCurrent(cur *Record, pw string) *ledger.Error {
	release, wait := s.Reserve()
	if wait > 0 {
		return apiErr("RATE_LIMITED", "Too many wrong attempts. Try again in "+strconv.Itoa(int(wait.Seconds())+1)+" seconds.")
	}
	if len(pw) > MaxPasswordBytes || !cur.Verify(cur.Username, pw) {
		return apiErr("WRONG_PASSWORD", "The current password is not right.")
	}
	release()
	return nil
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-('a'-'A')) + s[1:]
}
