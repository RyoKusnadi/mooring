package chat

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ryokusnadi/mooring/internal/paths"
)

// Terminals an agent may ask Mooring to run.
const (
	maxTerminals = 8
	maxRawOutput = 4 << 20
	drainAfter   = 150 * time.Millisecond // after exit, how long a pipe held by a grandchild is read
)

// term is one command run for the agent: no PTY, own process group, stdout and
// stderr in one stream.
type term struct {
	id    string
	cmd   *exec.Cmd
	rd    *os.File
	done  chan struct{} // closed when the command ended and its output was read
	outID string        // id of the saved raw output, once there is one

	mu      sync.Mutex
	buf     []byte // the last maxRawOutput bytes
	cut     bool   // the start of the output was dropped
	exit    *int
	signal  string
	limit   int // outputByteLimit of the agent; 0 means none
	running bool
}

// termHost holds the terminals of one agent run.
type termHost struct {
	mu    sync.Mutex
	terms map[string]*term
}

func newTermHost() *termHost { return &termHost{terms: map[string]*term{}} }

func randomID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func termError(code int, api, msg string) *rpcError {
	data, _ := json.Marshal(map[string]string{"code": api})
	return &rpcError{Code: code, Message: msg, Data: data}
}

type exitStatus struct {
	ExitCode *int   `json:"exitCode"`
	Signal   string `json:"signal,omitempty"`
}

func (t *term) status() *exitStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		return nil
	}
	return &exitStatus{ExitCode: t.exit, Signal: t.signal}
}

// terminal serves terminal/create, output, wait_for_exit, kill and release.
func (r *run) terminal(method string, raw json.RawMessage) (any, error) {
	c := r.c
	c.mu.Lock()
	c.termCalls++
	c.mu.Unlock()
	var p struct {
		Command         string   `json:"command"`
		Args            []string `json:"args"`
		Cwd             string   `json:"cwd"`
		OutputByteLimit int      `json:"outputByteLimit"`
		TerminalID      string   `json:"terminalId"`
		Env             []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"env"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "Invalid params"}
	}
	if method == "terminal/create" {
		env := make([]string, 0, len(p.Env))
		for _, e := range p.Env {
			if e.Name == "" || strings.ContainsAny(e.Name, "=\x00") {
				return nil, &rpcError{Code: -32602, Message: "Invalid params"}
			}
			env = append(env, e.Name+"="+e.Value)
		}
		return c.termCreate(r, p.Command, p.Args, p.Cwd, env, p.OutputByteLimit)
	}
	t := r.terms.get(p.TerminalID)
	if t == nil {
		return nil, &rpcError{Code: -32602, Message: "No such terminal"}
	}
	switch method {
	case "terminal/output":
		return c.termOutput(t)
	case "terminal/wait_for_exit":
		select {
		case <-t.done:
		case <-r.rpc.Done():
			return nil, errClosed
		}
		return t.status(), nil
	case "terminal/kill":
		t.kill()
		return map[string]any{}, nil
	case "terminal/release":
		r.terms.release(t)
		return map[string]any{}, nil
	}
	return nil, &rpcError{Code: -32601, Message: "Method not found: " + method}
}

func (h *termHost) get(id string) *term {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.terms[id]
}

func (c *Chat) termCreate(r *run, command string, args []string, cwd string, env []string, limit int) (any, error) {
	if command == "" || limit < 0 {
		return nil, &rpcError{Code: -32602, Message: "Invalid params"}
	}
	c.mu.Lock()
	root := c.dir
	c.mu.Unlock()
	dir, err := paths.Resolve(root, cwd)
	if errors.Is(err, paths.ErrOutsideRoot) {
		return nil, termError(-32602, "OUTSIDE_ROOT", "OUTSIDE_ROOT: the folder is outside the chat folder")
	}
	if err != nil {
		return nil, &rpcError{Code: -32602, Message: "The folder cannot be used"}
	}
	r.terms.mu.Lock()
	defer r.terms.mu.Unlock()
	if len(r.terms.terms) >= maxTerminals {
		return nil, termError(-32002, "LIMIT_REACHED", fmt.Sprintf("LIMIT_REACHED: a chat runs at most %d terminals; release one", maxTerminals))
	}
	t, err := startTerm(command, args, dir, r.env, env, limit)
	if err != nil {
		c.m.log.Warn("chat: terminal did not start", "chat", c.id, "err", err)
		return nil, &rpcError{Code: -32002, Message: "The command could not be started"}
	}
	r.terms.terms[t.id] = t
	return map[string]string{"terminalId": t.id}, nil
}

// startTerm runs a command in its own process group. Its environment is the one the
// agent was launched with (launchEnv: the agent's own settings and the account folder),
// then the variables of the request.
func startTerm(command string, args []string, dir string, launchEnv, env []string, limit int) (*term, error) {
	rd, wr, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("make pipe: %w", err)
	}
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), launchEnv...), env...)
	cmd.Stdout, cmd.Stderr = wr, wr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err = cmd.Start()
	_ = wr.Close()
	if err != nil {
		_ = rd.Close()
		return nil, fmt.Errorf("start %s: %w", command, err)
	}
	t := &term{id: randomID("t"), cmd: cmd, rd: rd, done: make(chan struct{}), limit: limit, running: true}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		chunk := make([]byte, 32<<10)
		for {
			n, err := rd.Read(chunk)
			if n > 0 {
				t.add(chunk[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		_ = cmd.Wait()
		_ = rd.SetReadDeadline(time.Now().Add(drainAfter))
		<-readDone
		t.mu.Lock()
		t.running = false
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			t.signal = ws.Signal().String()
		} else {
			code := cmd.ProcessState.ExitCode()
			t.exit = &code
		}
		t.mu.Unlock()
		close(t.done)
	}()
	return t, nil
}

func (t *term) add(b []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, b...)
	if len(t.buf) > maxRawOutput {
		t.buf = append([]byte{}, t.buf[len(t.buf)-maxRawOutput:]...)
		t.cut = true
	}
}

// kill ends the command and everything it started.
func (t *term) kill() {
	select {
	case <-t.done:
		return
	default:
	}
	_ = syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
}

// release kills the command if it still runs and forgets the terminal.
func (h *termHost) release(t *term) {
	t.kill()
	h.mu.Lock()
	delete(h.terms, t.id)
	h.mu.Unlock()
	select {
	case <-t.done:
	case <-time.After(time.Second):
	}
	_ = t.rd.Close()
}

// killAll kills every running command; the terminals stay so the agent can read them.
func (h *termHost) killAll() {
	h.mu.Lock()
	all := make([]*term, 0, len(h.terms))
	for _, t := range h.terms {
		all = append(all, t)
	}
	h.mu.Unlock()
	for _, t := range all {
		t.kill()
	}
}

// closeAll kills and forgets every terminal (the agent stopped).
func (h *termHost) closeAll() {
	h.mu.Lock()
	all := make([]*term, 0, len(h.terms))
	for _, t := range h.terms {
		all = append(all, t)
	}
	h.terms = map[string]*term{}
	h.mu.Unlock()
	for _, t := range all {
		t.kill()
	}
	for _, t := range all {
		select {
		case <-t.done:
		case <-time.After(time.Second):
		}
		_ = t.rd.Close()
	}
}

// snapshot returns the raw output and what the agent should see of it.
func (t *term) snapshot() (raw string, cut bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf), t.cut
}
