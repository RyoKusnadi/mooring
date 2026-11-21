package secrets

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

// runner runs a program with stdin and returns its stdout and exit code.
// A program that could not be started returns an error.
type runner func(bin string, stdin string, args ...string) (stdout []byte, code int, err error)

const runTimeout = 20 * time.Second

func execRunner(bin, stdin string, args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	var out bytes.Buffer
	cmd.Stdout = &out
	// Standard error is dropped: a tool may echo what it was given.
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out.Bytes(), ee.ExitCode(), nil
	}
	if err != nil {
		return nil, -1, err
	}
	return out.Bytes(), 0, nil
}
