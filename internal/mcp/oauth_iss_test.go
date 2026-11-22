package mcp

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/fakemcp"
)

func TestCheckIss(t *testing.T) {
	const site = "https://as.example"
	for _, c := range []struct {
		name       string
		query      string
		advertised bool
		ok         bool
	}{
		{"match, advertised", "iss=https%3A%2F%2Fas.example", true, true},
		{"match, not advertised", "iss=https%3A%2F%2Fas.example", false, true},
		{"wrong, advertised", "iss=https%3A%2F%2Fevil.example", true, false},
		{"wrong, not advertised", "iss=https%3A%2F%2Fevil.example", false, false},
		{"missing, advertised", "code=x", true, false},
		{"missing, not advertised", "code=x", false, true},
		{"trailing slash is not equal", "iss=https%3A%2F%2Fas.example%2F", false, false},
		{"case is not folded", "iss=https%3A%2F%2FAS.example", true, false},
		{"empty", "iss=", false, false},
		{"twice", "iss=https%3A%2F%2Fas.example&iss=https%3A%2F%2Fas.example", true, false},
	} {
		q, _ := url.ParseQuery(c.query)
		err := checkIss(q, site, c.advertised)
		if (err == nil) != c.ok {
			t.Errorf("%s: got %v", c.name, err)
		}
		if err != nil && !strings.Contains(err.Error(), "different sign-in site than expected") {
			t.Errorf("%s: message %q", c.name, err)
		}
	}
}

// signInWith runs a sign-in against the fake in the given iss mode and finishes it
// with the address the browser would have come back to.
func signInWith(t *testing.T, mode, edit string) (*rig, string, ToolServer, error) {
	t.Helper()
	r := newRig(t)
	r.fake.Iss = mode
	ts := r.save(r.remoteArgs("iss"))
	authURL, _, err := r.svc.SignIn(context.Background(), ts.ID)
	if err != nil {
		t.Fatal(err)
	}
	back, err := fakemcp.Approve(authURL)
	if err != nil {
		t.Fatal(err)
	}
	if edit == "error" {
		u, _ := url.Parse(back)
		q := u.Query()
		q.Del("code")
		q.Set("error", "access_denied")
		u.RawQuery = q.Encode()
		back = u.String()
	}
	out, err := r.svc.SignInFinish(context.Background(), ts.ID, back)
	return r, ts.ID, out, err
}

func TestSignInIssModes(t *testing.T) {
	for _, c := range []struct {
		mode string
		ok   bool
	}{{"ok", true}, {"wrong", false}, {"missing", false}, {"", true}} {
		r, id, out, err := signInWith(t, c.mode, "")
		if c.ok {
			if err != nil || out.SignIn != "signedIn" {
				t.Errorf("iss %q: %v %+v", c.mode, err, out)
			}
			tok, _ := r.svc.loadToken(out.ID)
			if tok.Issuer != r.fts.URL {
				t.Errorf("iss %q: saved issuer %q", c.mode, tok.Issuer)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), "different sign-in site than expected") {
			t.Errorf("iss %q: %v", c.mode, err)
		}
		if n := r.fake.Stats().Exchanges; n != 0 {
			t.Errorf("iss %q: the code went to the token endpoint %d times", c.mode, n)
		}
		if _, ok := r.svc.loadToken(id); ok {
			t.Errorf("iss %q: a token was saved", c.mode)
		}
	}
}

func TestWrongIssOnErrorReplyHidesTheError(t *testing.T) {
	_, _, _, err := signInWith(t, "wrong", "error")
	if err == nil || !strings.Contains(err.Error(), "different sign-in site than expected") {
		t.Fatalf("got %v", err)
	}
	_, _, _, err = signInWith(t, "ok", "error")
	if err == nil || !strings.Contains(err.Error(), "not completed") {
		t.Fatalf("an error reply from the right site: %v", err)
	}
}

func TestRefreshWithChangedIssuerAsksForSignIn(t *testing.T) {
	r := newRig(t)
	r.fake.Iss = "ok"
	ts := r.save(r.remoteArgs("moved"))
	r.signInViaCallback(ts.ID)
	tok, ok := r.svc.loadToken(ts.ID)
	if !ok || tok.Issuer != r.fts.URL {
		t.Fatalf("token %+v", tok)
	}

	// Same issuer: a refresh works.
	tok.Expires = time.Now().Add(-time.Minute).UnixMilli()
	if err := r.svc.saveToken(ts.ID, tok); err != nil {
		t.Fatal(err)
	}
	rc, _ := r.svc.get(ts.ID)
	if _, err := r.svc.accessToken(context.Background(), rc, false); err != nil || r.fake.Stats().Refreshes != 1 {
		t.Fatalf("refresh: %v %+v", err, r.fake.Stats())
	}

	// The saved issuer is not the site's any more.
	tok, _ = r.svc.loadToken(ts.ID)
	tok.Issuer, tok.Expires = "https://old-site.example", time.Now().Add(-time.Minute).UnixMilli()
	if err := r.svc.saveToken(ts.ID, tok); err != nil {
		t.Fatal(err)
	}
	_, err := r.svc.accessToken(context.Background(), rc, false)
	if err == nil || !strings.Contains(err.Error(), "sign-in site changed") {
		t.Fatalf("got %v", err)
	}
	if r.fake.Stats().Refreshes != 1 {
		t.Fatal("the refresh token was sent to a site with another issuer")
	}
	if _, ok := r.svc.loadToken(ts.ID); ok {
		t.Fatal("tokens kept")
	}
	if got, _ := r.svc.get(ts.ID); got.SignIn != "needed" {
		t.Fatalf("sign-in state %q", got.SignIn)
	}
}
