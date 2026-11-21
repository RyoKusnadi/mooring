package mcp

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type readCloser struct {
	io.Reader
	io.Closer
}

// checkStdio starts a local server the way a chat would (in the home folder, own
// process group, with its env), lists its tools and stops it. The whole group
// is killed at the end, also when the check timed out.
func (s *Service) checkStdio(ctx context.Context, r rec, env map[string]string) ([]Tool, time.Duration, error) {
	secretVals := make([]string, 0, len(env))
	cmdEnv := os.Environ()
	for k, v := range env {
		secretVals = append(secretVals, v)
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	path, err := exec.LookPath(r.Command)
	if err != nil {
		return nil, 0, apiErr("CHECK_FAILED", "The program %q was not found on this machine.", r.Command)
	}
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, 0, fmt.Errorf("make pipe: %w", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, 0, fmt.Errorf("make pipe: %w", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		return nil, 0, fmt.Errorf("make pipe: %w", err)
	}
	cmd := exec.Command(path, r.Args...)
	cmd.Dir, cmd.Env = s.Home, cmdEnv
	cmd.Stdin, cmd.Stdout, cmd.Stderr = inR, outW, errW
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	startErr := cmd.Start()
	_ = inR.Close()
	_ = outW.Close()
	_ = errW.Close()
	if startErr != nil {
		_ = inW.Close()
		_ = outR.Close()
		_ = errR.Close()
		return nil, 0, apiErr("CHECK_FAILED", "%s", scrub(fmt.Sprintf("The program did not start: %v", startErr), secretVals))
	}
	stderr := &capture{}
	drained := make(chan struct{})
	go func() { _, _ = io.Copy(stderr, errR); _ = errR.Close(); close(drained) }()
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	defer func() {
		_ = inW.Close()
		stopGroup(cmd.Process.Pid, exited, filepath.Base(r.Command) == "docker")
		_ = outR.Close()
		_ = errR.Close()
	}()

	spy := &ttlSpy{}
	t := &sdk.IOTransport{
		Reader: readCloser{spy.wrap(&capReader{r: outR, left: maxToolListBody}), outR},
		Writer: inW,
	}
	tools, err := listToolsCtx(ctx, t)
	if err != nil {
		// Let the process say why it stopped before the output is read.
		select {
		case <-exited:
			select {
			case <-drained:
			case <-time.After(200 * time.Millisecond):
			}
		case <-time.After(300 * time.Millisecond):
		}
		why := scrub(err.Error(), secretVals)
		if ctx.Err() != nil {
			why = "It did not answer in time."
		}
		msg := "The server did not give a tool list: " + why
		if out := strings.TrimSpace(stderr.text(secretVals)); out != "" {
			msg += "\nIts error output:\n" + out
		}
		return nil, 0, apiErr("CHECK_FAILED", "%s", msg)
	}
	return tools, spy.ttl(), nil
}

// stopGroup ends the process group of a check. A docker command gets SIGTERM and a few
// seconds first, because the docker CLI passes it on and --rm then removes the
// container; killing only the CLI would leave the container running. The group is
// signalled only while the leader has not been reaped, so a reused id is never hit.
func stopGroup(pid int, exited <-chan struct{}, graceful bool) {
	signal := func(sig syscall.Signal) {
		select {
		case <-exited:
		default:
			_ = syscall.Kill(-pid, sig)
		}
	}
	if graceful {
		signal(syscall.SIGTERM)
		select {
		case <-exited:
			return
		case <-time.After(5 * time.Second):
		}
	}
	signal(syscall.SIGKILL)
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
	}
}

// listToolsCtx is listTools that gives up when ctx ends, even if the transport hangs.
func listToolsCtx(ctx context.Context, t sdk.Transport) ([]Tool, error) {
	type result struct {
		tools []Tool
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		tools, err := listTools(ctx, t)
		ch <- result{tools, err}
	}()
	select {
	case r := <-ch:
		return r.tools, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// listTools connects, lists the tools (at most maxTools, descriptions cut) and closes.
func listTools(ctx context.Context, t sdk.Transport) ([]Tool, error) {
	client := sdk.NewClient(&sdk.Implementation{Name: "mooring", Version: "0"}, nil)
	cs, err := client.Connect(ctx, t, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cs.Close() }()
	tools := []Tool{}
	cursor := ""
	for {
		res, err := cs.ListTools(ctx, &sdk.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		for _, tl := range res.Tools {
			if len(tools) < maxTools {
				tools = append(tools, Tool{Name: clip(tl.Name, 128), Description: clip(tl.Description, 300)})
			}
		}
		if res.NextCursor == "" || len(tools) >= maxTools {
			return tools, nil
		}
		cursor = res.NextCursor
	}
}
