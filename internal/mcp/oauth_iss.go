package mcp

import (
	"errors"
	"net/url"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

// The sign-in reply is checked against the issuer recorded before the browser left, as
// MCP 2026-07-28 "Authorization Response Validation" asks
// (https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization)
// and RFC 9207 section 2.4 (https://www.rfc-editor.org/rfc/rfc9207#section-2.4):
// a present iss must equal the recorded issuer byte for byte; a missing iss is
// refused only when the site's metadata says authorization_response_iss_parameter_supported.
// This runs on error replies too, so a reply from the wrong site is never shown or acted on.
func checkIss(q url.Values, expected string, advertised bool) error {
	got, present := q["iss"]
	switch {
	case !present && advertised:
		return errWrongIssuer()
	case !present:
		return nil
	case len(got) != 1 || got[0] != expected:
		return errWrongIssuer()
	}
	return nil
}

func errWrongIssuer() error {
	return signInFailed("The sign-in reply came from a different sign-in site than expected. Start the sign-in again.")
}

func errIssuerChanged() error {
	return apiErr("SIGN_IN_REQUIRED", "This tool server's sign-in site changed. Sign in again.")
}

// note is why a server needs a sign-in or failed one, when something more specific than the
// default is known. It lives in memory and is cleared by the next sign-in start, a success or a sign-out.
func (s *Service) note(id string) string {
	v, _ := s.notes.Load(id)
	m, _ := v.(string)
	return m
}

func (s *Service) setNote(id string, err error) {
	var le *ledger.Error
	if errors.As(err, &le) {
		s.notes.Store(id, le.Message)
	}
}
