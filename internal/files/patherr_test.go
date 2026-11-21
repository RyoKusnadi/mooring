package files

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

func TestPathErrNamesTheFile(t *testing.T) {
	if got, want := message(pathErr(os.ErrNotExist, "src/main.go")), "src/main.go does not exist."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := message(pathErr(os.ErrNotExist)); got != "That file or folder does not exist." {
		t.Errorf("unnamed: got %q", got)
	}
	if got := message(pathErr(os.ErrPermission, "a.txt")); got != "The server is not allowed to use a.txt." {
		t.Errorf("permission: got %q", got)
	}
	other := errors.New("boom")
	if pathErr(other, "x") != other {
		t.Error("an unknown error passes through")
	}
}

func TestListDotFoldersLastAndNoGitFolder(t *testing.T) {
	r := newRig(t, nil)
	for _, n := range []string{".git/HEAD", ".github/x.yml", "zed/a", "app/a", "readme.md", ".env"} {
		r.write(n, "x")
	}
	res, err := r.s.List("p1", "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range res.Entries {
		got = append(got, e.Name)
	}
	want := []string{"app", "zed", ".github", ".env", "readme.md"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
