package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

func spyTTL(body string) time.Duration {
	spy := &ttlSpy{}
	rd := spy.wrap(strings.NewReader(body))
	buf := make([]byte, 5)
	for {
		if _, err := rd.Read(buf); err != nil {
			break
		}
	}
	return spy.ttl()
}

func TestTTLSpyValues(t *testing.T) {
	for name, c := range map[string]struct {
		ttl  string
		want time.Duration
	}{
		"whole":     {`"ttlMs":5000,`, 5 * time.Second},
		"one":       {`"ttlMs":1,`, time.Millisecond},
		"limit":     {`"ttlMs":86400000,`, 24 * time.Hour},
		"zero":      {`"ttlMs":0,`, 0},
		"negative":  {`"ttlMs":-5,`, 0},
		"float":     {`"ttlMs":1.5,`, 0},
		"float 1e3": {`"ttlMs":1e3,`, 0},
		"string":    {`"ttlMs":"5000",`, 0},
		"null":      {`"ttlMs":null,`, 0},
		"too long":  {`"ttlMs":86400001,`, 0},
		"huge":      {`"ttlMs":99999999999999999999,`, 0},
		"absent":    {``, defaultToolTTL},
	} {
		body := `{"jsonrpc":"2.0","id":2,"result":{` + c.ttl + `"tools":[]}}` + "\n"
		if got := spyTTL(body); got != c.want {
			t.Errorf("%s: ttl %v, want %v", name, got, c.want)
		}
	}
}

func TestTTLSpyJoinsDataLinesOfOneEvent(t *testing.T) {
	body := "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\ndata: \"result\":{\"tools\":[],\ndata: \"ttlMs\":7000}}\n\n"
	if got := spyTTL(body); got != 7*time.Second {
		t.Fatalf("ttl %v", got)
	}
	// The same event cut at the end of the stream with no blank line.
	if got := spyTTL(strings.TrimSuffix(body, "\n\n")); got != 7*time.Second {
		t.Fatalf("no blank line: ttl %v", got)
	}
	// Two events: the shorter ttl wins.
	two := "data: {\"result\":{\"tools\":[],\"ttlMs\":9000}}\n\ndata: {\"result\":{\"tools\":[],\"ttlMs\":3000}}\n\n"
	if got := spyTTL(two); got != 3*time.Second {
		t.Fatalf("two events: ttl %v", got)
	}
}

func TestSignInDropsTheSavedToolList(t *testing.T) {
	r := newRig(t)
	r.fake.TTLms = 90000
	ts := r.save(r.remoteArgs("resign"))
	r.signInViaCallback(ts.ID)
	if _, err := r.svc.Check(context.Background(), ts.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.svc.cache.m[ts.ID]; !ok {
		t.Fatal("no entry to drop")
	}
	r.signInViaCallback(ts.ID)
	if _, ok := r.svc.cache.m[ts.ID]; ok {
		t.Fatal("a new sign-in kept the old tool list")
	}
	if _, err := r.svc.Check(context.Background(), ts.ID); err != nil {
		t.Fatal(err)
	}
	r.svc.lostSignIn(ts.ID)
	if _, ok := r.svc.cache.m[ts.ID]; ok {
		t.Fatal("a lost sign-in kept the tool list")
	}
}

func TestRemoveDropsTheSavedToolList(t *testing.T) {
	r := newRig(t)
	r.fake.TTLms = 90000
	ts := r.save(r.remoteArgs("gone"))
	r.signInViaCallback(ts.ID)
	if _, err := r.svc.Check(context.Background(), ts.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.Remove(ts.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.svc.cache.m[ts.ID]; ok {
		t.Fatal("remove kept the tool list")
	}
}

func expireToken(t *testing.T, r *rig, id string) {
	t.Helper()
	tok, ok := r.svc.loadToken(id)
	if !ok {
		t.Fatal("no token")
	}
	tok.Expires = time.Now().Add(-time.Minute).UnixMilli()
	if err := r.svc.saveToken(id, tok); err != nil {
		t.Fatal(err)
	}
}

func TestIssuerChangeKeepsItsReason(t *testing.T) {
	r := newRig(t)
	r.fake.Iss = "ok"
	r.fake.TTLms = 90000
	ts := r.save(r.remoteArgs("moved site"))
	r.signInViaCallback(ts.ID)
	if _, err := r.svc.Check(context.Background(), ts.ID); err != nil {
		t.Fatal(err)
	}
	expireToken(t, r, ts.ID)
	r.fake.MovedIssuer = true
	_, err := r.svc.CheckFresh(context.Background(), ts.ID)
	if err == nil || !strings.Contains(err.Error(), "sign-in site changed") {
		t.Fatalf("got %v", err)
	}
	got, _ := r.svc.get(ts.ID)
	if got.SignIn != "needed" || !strings.Contains(got.Note, "sign-in site changed") {
		t.Fatalf("state %q note %q", got.SignIn, got.Note)
	}
	if _, ok := r.svc.cache.m[ts.ID]; ok {
		t.Fatal("the tool list was kept after the sign-in was lost")
	}
	// Signing in again clears the reason.
	r.fake.MovedIssuer = false
	r.signInViaCallback(ts.ID)
	if got, _ = r.svc.get(ts.ID); got.Note != "" || got.SignIn != "signedIn" {
		t.Fatalf("after sign-in: %q %q", got.SignIn, got.Note)
	}
}

func TestWrongIssKeepsItsReason(t *testing.T) {
	r, id, _, err := signInWith(t, "wrong", "")
	if err == nil {
		t.Fatal("accepted")
	}
	got, _ := r.svc.get(id)
	if got.SignIn != "failed" || !strings.Contains(got.Note, "different sign-in site") {
		t.Fatalf("state %q note %q", got.SignIn, got.Note)
	}
}

func TestRefreshLookupFailureIsAConnectionError(t *testing.T) {
	r := newRig(t)
	r.fake.Iss = "ok"
	ts := r.save(r.remoteArgs("offline"))
	r.signInViaCallback(ts.ID)
	expireToken(t, r, ts.ID)
	rc, _ := r.svc.get(ts.ID)
	r.fts.Close()
	_, err := r.svc.accessToken(context.Background(), rc, false)
	var le *ledger.Error
	if !errors.As(err, &le) || le.Code != "CHECK_FAILED" {
		t.Fatalf("got %v", err)
	}
	if _, ok := r.svc.loadToken(ts.ID); !ok {
		t.Fatal("the saved sign-in was dropped")
	}
	if got, _ := r.svc.get(ts.ID); got.SignIn != "signedIn" {
		t.Fatalf("sign-in state %q", got.SignIn)
	}
}
