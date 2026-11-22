package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func passFile(t *testing.T, dir, pass string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, "pass")
	if err := os.WriteFile(p, []byte(pass+"\n"), mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFileRoundTripAndWrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenFile(dir, passFile(t, dir, "right horse", 0o600), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("h1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty store: %v", err)
	}
	const secret = "hunter2 \"quoted\"\nsecond line"
	if err := s.Set("h1", secret); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "secrets.enc"))
	if strings.Contains(string(raw), "hunter2") {
		t.Fatal("the file holds the value in clear text")
	}
	again, err := OpenFile(dir, passFile(t, dir, "right horse", 0o600), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := again.Get("h1"); err != nil || got != secret {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := OpenFile(dir, passFile(t, dir, "wrong horse", 0o600), 1000); !errors.Is(err, ErrWrongPassphrase) {
		t.Fatalf("wrong passphrase: %v", err)
	}
	if err := again.Delete("h1"); err != nil {
		t.Fatal(err)
	}
	if _, err := again.Get("h1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestPassphraseFileMustBePrivate(t *testing.T) {
	dir := t.TempDir()
	if _, err := OpenFile(dir, passFile(t, dir, "x", 0o644), 1000); err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("got %v", err)
	}
}

func TestBadNamesAreRefused(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenFile(dir, passFile(t, dir, "x", 0o600), 1000)
	for _, n := range []string{"", "a b", "../x", strings.Repeat("a", 129)} {
		if s.Set(n, "v") == nil {
			t.Errorf("name %q accepted", n)
		}
	}
}

func TestKeychainKeepsValuesOutOfArgv(t *testing.T) {
	const secret = "pa$$ \"word\" with 'quotes'"
	var argvs [][]string
	var stdins []string
	r := func(bin, stdin string, args ...string) ([]byte, int, error) {
		argvs = append(argvs, append([]string{bin}, args...))
		stdins = append(stdins, stdin)
		if args[0] == "find-generic-password" {
			return []byte("cGEkJCAid29yZCIgd2l0aCAncXVvdGVzJw==\n"), 0, nil
		}
		return nil, 0, nil
	}
	k := newKeychain(r)
	if err := k.Set("host1", secret); err != nil {
		t.Fatal(err)
	}
	if got, err := k.Get("host1"); err != nil || got != secret {
		t.Fatalf("get %q, %v", got, err)
	}
	for _, a := range argvs {
		if strings.Contains(strings.Join(a, " "), "quotes") {
			t.Fatalf("value in argv: %v", a)
		}
	}
	if argvs[0][1] != "-i" || !strings.HasPrefix(stdins[0], "add-generic-password -U -s mooring -a host1 -w ") {
		t.Fatalf("set did not use security -i: %v %q", argvs[0], stdins[0])
	}
	if strings.Contains(stdins[0], "quotes") {
		t.Fatal("value sent raw, not encoded")
	}
}

func TestKeychainNotFound(t *testing.T) {
	k := newKeychain(func(string, string, ...string) ([]byte, int, error) { return nil, 44, nil })
	if _, err := k.Get("h"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	if err := k.Delete("h"); err != nil {
		t.Fatal(err)
	}
}

func TestSecretToolUsesStdin(t *testing.T) {
	var stdin string
	var argv []string
	s := newSecretTool("/bin/secret-tool", func(bin, in string, args ...string) ([]byte, int, error) {
		stdin, argv = in, args
		return nil, 0, nil
	})
	if err := s.Set("h", "topsecret"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(argv, " "), "topsecret") || stdin == "" || stdin == "topsecret" {
		t.Fatalf("argv %v stdin %q", argv, stdin)
	}
}
