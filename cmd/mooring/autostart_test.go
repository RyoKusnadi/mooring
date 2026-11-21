package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ryokusnadi/mooring/internal/autostart"
)

// noMachine fails the test if autostart reads the machine or runs a program.
func noMachine(t *testing.T) func() (autostart.Env, error) {
	return func() (autostart.Env, error) {
		t.Helper()
		t.Fatal("autostart must not reach the machine for this input")
		return autostart.Env{}, errors.New("unreachable")
	}
}

func TestAutostartRefusesBadInputBeforeTouchingTheMachine(t *testing.T) {
	cases := map[string][]string{
		"usage":            {},
		"usage: ":          {"maybe"},
		"not a local":      {"on", "-addr", "0.0.0.0:7340"},
		"unknown argument": {"off", "extra"},
	}
	for want, args := range cases {
		err := runAutostart(args, noMachine(t), &bytes.Buffer{}, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%v: err = %v", args, err)
		}
	}
}

func TestAutostartOffGoesThroughTheGivenEnvironment(t *testing.T) {
	var out bytes.Buffer
	env := autostart.Env{GOOS: "linux", Home: t.TempDir(), Exe: "/opt/mooring",
		Run: func(string, ...string) ([]byte, error) {
			t.Fatal("nothing is installed, nothing runs")
			return nil, nil
		}}
	if err := runAutostart([]string{"off"}, func() (autostart.Env, error) { return env, nil }, &out, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Autostart is off.\n" {
		t.Fatalf("out = %q", out.String())
	}
}
