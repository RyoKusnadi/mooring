package server

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/ryokusnadi/mooring/internal/password"
)

// hashSlots is how many password checks run at once. Each uses 64 MiB, so a
// burst of attempts waits instead of filling memory.
const hashSlots = 2

var errPasswordFile = errors.New("password file cannot be used")

// blockedError says logins are paused; wait is how long.
type blockedError struct{ wait time.Duration }

func (e blockedError) Error() string { return "too many attempts" }

// takeSlot waits for one of the few places where a password may be hashed.
func (s *server) takeSlot(ctx context.Context) (func(), error) {
	select {
	case s.slots <- struct{}{}:
		return func() { <-s.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// passwordLogin checks a typed username and password against the file in the
// data folder, read afresh each time so `mooring password` needs no restart.
// A wrong username, a wrong password and no password set all fail the same way
// and all run the hash. The attempt is counted before the hash runs and given
// back on success, so a burst of parallel requests cannot get more guesses
// than the limit allows.
func (s *server) passwordLogin(ctx context.Context, username, pw string) (string, bool, error) {
	done, err := s.takeSlot(ctx)
	if err != nil {
		return "", false, err
	}
	defer done()
	release, wait := s.auth.reserve()
	if wait > 0 {
		return "", false, blockedError{wait}
	}
	if len(pw) > password.MaxPasswordBytes {
		return "", false, nil
	}
	var rec *password.Record
	if s.dataDir != "" {
		if rec, err = password.Load(s.dataDir); err != nil {
			s.log.Error("login: read password file", "err", err)
			return "", false, errPasswordFile
		}
	}
	ok := false
	if rec != nil {
		ok = rec.Verify(username, pw)
	} else {
		password.VerifyNothing(pw)
	}
	if !ok {
		return "", false, nil
	}
	release()
	value, err := s.auth.newSession()
	return value, err == nil, err
}

// handleLoginState says whether a password is set, so the login page knows
// which form to show. It needs no session and says nothing else.
func (s *server) handleLoginState(w http.ResponseWriter, _ *http.Request) {
	set := false
	if s.dataDir != "" {
		rec, err := password.Load(s.dataDir)
		set = err == nil && rec != nil
		if err != nil {
			set = true // a file that cannot be read still means one was set
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]bool{"passwordSet": set})
}

// handlePasswordStatus tells Settings whether a password is set and for which username.
func (s *server) handlePasswordStatus(w http.ResponseWriter, _ *http.Request, svc *password.Service) {
	st, err := svc.Status()
	if err != nil {
		s.log.Error("password: read status", "err", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(st)
}

// handleSignOutOthers makes a new login token, like `mooring token -new`, which
// ends every other session and sign-in link, and gives this browser a fresh
// cookie so it stays signed in. It needs the current password.
func (s *server) handleSignOutOthers(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r) {
		writeError(w, http.StatusUnauthorized, "login_required")
		return
	}
	var body struct {
		Current string `json:"current"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	done, err := s.takeSlot(r.Context())
	if err != nil {
		return
	}
	defer done()
	release, wait := s.auth.reserve()
	if wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "too_many_attempts")
		return
	}
	var rec *password.Record
	if s.dataDir != "" {
		rec, err = password.Load(s.dataDir)
	}
	if err != nil || rec == nil {
		writeError(w, http.StatusBadRequest, "no_password")
		return
	}
	if len(body.Current) > password.MaxPasswordBytes || !rec.Verify(rec.Username, body.Current) {
		writeError(w, http.StatusUnauthorized, "wrong_password")
		return
	}
	release()
	token, err := ReplaceToken(s.dataDir)
	if err != nil {
		s.log.Error("sign out others: new token", "err", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.auth.rotate(token)
	value, err := s.auth.newSession()
	if err != nil {
		s.log.Error("sign out others: new session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.setSessionCookie(w, r, value)
	s.log.Info("sign out others: token replaced")
	w.WriteHeader(http.StatusNoContent)
}
