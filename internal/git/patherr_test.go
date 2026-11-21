package git

import (
	"errors"
	"os"
	"testing"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

func message(err error) string {
	var le *ledger.Error
	if errors.As(err, &le) {
		return le.Message
	}
	return err.Error()
}

func TestMapPathErrNamesTheFile(t *testing.T) {
	if got, want := message(mapPathErr(os.ErrNotExist, "docs/a.md")), "docs/a.md does not exist."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := message(mapPathErr(os.ErrNotExist)); got != "That path does not exist." {
		t.Errorf("unnamed: got %q", got)
	}
}
