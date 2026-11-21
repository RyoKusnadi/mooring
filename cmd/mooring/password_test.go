package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryokusnadi/mooring/internal/password"
	"golang.org/x/sys/unix"
)

func lowCost(t *testing.T) { t.Cleanup(password.UseLowCost()) }

// fakeInput answers the prompts in order; lines and secrets are separate queues.
type fakeInput struct {
	tty     bool
	lines   []string
	secrets []string
	prompts []string
}

func (f *fakeInput) isTerminal() bool { return f.tty }

func (f *fakeInput) line(p string) (string, error) {
	f.prompts = append(f.prompts, p)
	if len(f.lines) == 0 {
		return "", errors.New("no more input")
	}
	s := f.lines[0]
	f.lines = f.lines[1:]
	return s, nil
}

func (f *fakeInput) secret(p string) (string, error) {
	f.prompts = append(f.prompts, p)
	if len(f.secrets) == 0 {
		return "", errors.New("no more input")
	}
	s := f.secrets[0]
	f.secrets = f.secrets[1:]
	return s, nil
}

func TestPasswordCommandSetsAndKeepsTheOldUsernameAsDefault(t *testing.T) {
	lowCost(t)
	dir := t.TempDir()
	in := &fakeInput{tty: true, lines: []string{"ada"}, secrets: []string{"first password", "first password"}}
	var out bytes.Buffer
	if err := runPassword([]string{"-data", dir}, in, &out, &out); err != nil {
		t.Fatal(err)
	}
	rec, err := password.Load(dir)
	if err != nil || rec == nil || !rec.Verify("ada", "first password") {
		t.Fatalf("not saved: %v %v", rec, err)
	}
	info, _ := os.Stat(filepath.Join(dir, password.FileName))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
	if strings.Contains(out.String(), "first password") {
		t.Fatal("the output repeats the password")
	}

	// A second run: an empty username keeps "ada", and the prompt shows it.
	in = &fakeInput{tty: true, lines: []string{""}, secrets: []string{"second password", "second password"}}
	out.Reset()
	if err := runPassword([]string{"-data", dir}, in, &out, &out); err != nil {
		t.Fatal(err)
	}
	if in.prompts[0] != "Username [ada]: " {
		t.Fatalf("prompt %q", in.prompts[0])
	}
	if rec, _ := password.Load(dir); rec == nil || !rec.Verify("ada", "second password") {
		t.Fatal("not changed")
	}
}

func TestPasswordCommandRefusesWithoutATerminal(t *testing.T) {
	lowCost(t)
	dir := t.TempDir()
	in := &fakeInput{tty: false, lines: []string{"ada"}, secrets: []string{"first password", "first password"}}
	err := runPassword([]string{"-data", dir}, in, new(bytes.Buffer), new(bytes.Buffer))
	if err == nil || !strings.Contains(err.Error(), "needs a terminal") {
		t.Fatalf("got %v", err)
	}
	if len(in.prompts) != 0 {
		t.Fatal("asked something anyway")
	}
	if rec, _ := password.Load(dir); rec != nil {
		t.Fatal("saved without a terminal")
	}
}

func TestPasswordCommandRefusesMismatchAndShortPasswords(t *testing.T) {
	lowCost(t)
	dir := t.TempDir()
	for _, secrets := range [][]string{{"first password", "other password"}, {"short", "short"}} {
		in := &fakeInput{tty: true, lines: []string{"ada"}, secrets: secrets}
		if err := runPassword([]string{"-data", dir}, in, new(bytes.Buffer), new(bytes.Buffer)); err == nil {
			t.Fatalf("%v accepted", secrets)
		}
		if rec, _ := password.Load(dir); rec != nil {
			t.Fatalf("%v saved", secrets)
		}
	}
}

func TestPasswordCommandRemove(t *testing.T) {
	lowCost(t)
	dir := t.TempDir()
	rec, _ := password.NewRecord("ada", "first password")
	_ = password.Save(dir, rec)
	if err := runPassword([]string{"-remove", "-data", dir}, &fakeInput{}, new(bytes.Buffer), new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	if rec, _ := password.Load(dir); rec != nil {
		t.Fatal("still set")
	}
	if err := runPassword([]string{"-remove", "-data", dir}, &fakeInput{}, new(bytes.Buffer), new(bytes.Buffer)); err != nil {
		t.Fatalf("removing nothing: %v", err)
	}
}

func TestPasswordCommandReplacesAnUnknownVersionFile(t *testing.T) {
	lowCost(t)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, password.FileName), []byte("v9:x\n"), 0o600)
	in := &fakeInput{tty: true, lines: []string{"ada"}, secrets: []string{"first password", "first password"}}
	if err := runPassword([]string{"-data", dir}, in, new(bytes.Buffer), new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	if rec, err := password.Load(dir); err != nil || rec == nil {
		t.Fatalf("%v %v", rec, err)
	}
}

func TestSilenceTurnsEchoOffAndRestoresOnce(t *testing.T) {
	orig := &unix.Termios{Lflag: unix.ECHO | unix.ICANON}
	var sets []uint64
	c := echoControl{
		get: func() (*unix.Termios, error) { return orig, nil },
		set: func(m *unix.Termios) error { sets = append(sets, uint64(m.Lflag)); return nil },
	}
	restore, err := silence(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0]&unix.ECHO != 0 || sets[0]&unix.ICANON == 0 {
		t.Fatalf("echo not turned off alone: %v", sets)
	}
	restore()
	restore()
	if len(sets) != 2 || sets[1] != uint64(unix.ECHO|unix.ICANON) {
		t.Fatalf("not restored exactly once: %v", sets)
	}
	if orig.Lflag&unix.ECHO == 0 {
		t.Fatal("the saved modes were changed")
	}
}

func TestSilenceReportsErrors(t *testing.T) {
	if _, err := silence(echoControl{get: func() (*unix.Termios, error) { return nil, errors.New("no") }}); err == nil {
		t.Fatal("get error hidden")
	}
	_, err := silence(echoControl{
		get: func() (*unix.Termios, error) { return &unix.Termios{}, nil },
		set: func(*unix.Termios) error { return errors.New("no") },
	})
	if err == nil {
		t.Fatal("set error hidden")
	}
}
