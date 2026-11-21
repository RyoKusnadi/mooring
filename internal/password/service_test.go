package password

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/events"
	"github.com/ryokusnadi/mooring/internal/ledger"
)

type limiter struct {
	fails int
	block time.Duration
}

func testService(t *testing.T) (*Service, *limiter, string) {
	t.Helper()
	dir := t.TempDir()
	l := &limiter{}
	reserve := func() (func(), time.Duration) {
		if l.block > 0 {
			return nil, l.block
		}
		l.fails++
		return func() { l.fails-- }, 0
	}
	return NewService(Config{Dir: dir, Reserve: reserve}), l, dir
}

func run(s *Service, name string, a args) (any, error) {
	raw, _ := json.Marshal(a)
	return s.Run(events.Command{ID: "c", Name: name, Args: raw})
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *ledger.Error
	if err == nil || !asLedger(err, &e) || e.Code != code {
		t.Fatalf("got %v, want code %s", err, code)
	}
}

func asLedger(err error, out **ledger.Error) bool {
	e, ok := err.(*ledger.Error)
	*out = e
	return ok
}

func TestSetChangeRemoveNeedCurrentPassword(t *testing.T) {
	s, l, dir := testService(t)
	if _, err := run(s, "password.set", args{Username: "ada", Password: goodPassword}); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.Status(); !st.Set || st.Username != "ada" {
		t.Fatalf("status %+v", st)
	}
	_, err := run(s, "password.set", args{Username: "bob", Password: "another password"})
	wantCode(t, err, "WRONG_PASSWORD")
	_, err = run(s, "password.remove", args{Current: "nope nope nope"})
	wantCode(t, err, "WRONG_PASSWORD")
	if l.fails != 2 {
		t.Fatalf("fails %d, want 2", l.fails)
	}
	if rec, _ := Load(dir); rec == nil || rec.Username != "ada" {
		t.Fatal("a refused change altered the file")
	}
	if _, err := run(s, "password.set", args{Current: goodPassword, Username: "bob", Password: "another password"}); err != nil {
		t.Fatal(err)
	}
	if rec, _ := Load(dir); rec == nil || !rec.Verify("bob", "another password") {
		t.Fatal("change was not saved")
	}
	if _, err := run(s, "password.remove", args{Current: "another password"}); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.Status(); st.Set {
		t.Fatal("still set after remove")
	}
}

func TestBadNewValuesDoNotUseUpAGuess(t *testing.T) {
	s, l, _ := testService(t)
	_, _ = run(s, "password.set", args{Username: "ada", Password: goodPassword})
	_, err := run(s, "password.set", args{Current: "wrong wrong", Username: "ada", Password: "short"})
	wantCode(t, err, "BAD_REQUEST")
	if l.fails != 0 {
		t.Fatalf("a short new password counted as a failed guess")
	}
	_, err = run(s, "password.set", args{Current: goodPassword, Username: " ", Password: goodPassword})
	wantCode(t, err, "BAD_REQUEST")
}

func TestSharedFailureLimitStopsCurrentPasswordGuesses(t *testing.T) {
	s, l, _ := testService(t)
	_, _ = run(s, "password.set", args{Username: "ada", Password: goodPassword})
	l.block = 30 * time.Second
	_, err := run(s, "password.remove", args{Current: goodPassword})
	wantCode(t, err, "RATE_LIMITED")
}

func TestErrorsNeverHoldPasswords(t *testing.T) {
	s, _, _ := testService(t)
	_, _ = run(s, "password.set", args{Username: "ada", Password: goodPassword})
	_, err := run(s, "password.set", args{Current: "my-secret-guess", Username: "ada", Password: "new-secret-pass"})
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error repeats a password: %v", err)
	}
}

func TestCommandsAreSecret(t *testing.T) {
	if !Secret("password.set") || !Secret("password.remove") || Secret("settings.save") {
		t.Fatal("password commands must be secret, others not")
	}
}

func TestNoDataFolderMeansNoPassword(t *testing.T) {
	s := NewService(Config{Reserve: func() (func(), time.Duration) { return func() {}, 0 }})
	_, err := run(s, "password.set", args{Username: "ada", Password: goodPassword})
	wantCode(t, err, "BAD_REQUEST")
	if st, err := s.Status(); err != nil || st.Set {
		t.Fatalf("%+v %v", st, err)
	}
}
