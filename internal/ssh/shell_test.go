package ssh

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/ryokusnadi/mooring/internal/fakessh"
	"github.com/ryokusnadi/mooring/internal/terminal"
)

type termEnv struct {
	*env
	terms *terminal.Manager
	srv   *httptest.Server
}

func newTermEnv(t *testing.T, o envOpts) *termEnv {
	t.Helper()
	e := newEnv(t, o)
	tm := terminal.New(terminal.Config{
		Pub: e.pub, Dir: func(string) (string, error) { return t.TempDir(), nil }, Log: nil,
		Remote: func(connID string, cols, rows uint16) (terminal.Backend, error) {
			sh, err := e.m.OpenShell(connID, cols, rows)
			if err != nil {
				return nil, err
			}
			return sh, nil
		},
	})
	t.Cleanup(tm.Shutdown)
	mux := http.NewServeMux()
	mux.Handle("GET /api/terminals/{id}", tm.Handler())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &termEnv{env: e, terms: tm, srv: srv}
}

type sock struct {
	t   *testing.T
	ws  *websocket.Conn
	ctx context.Context
}

func (te *termEnv) dial(id, epoch string, pos uint64) (*sock, map[string]any) {
	te.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	te.t.Cleanup(cancel)
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(te.srv.URL, "http")+"/api/terminals/"+id, nil)
	if err != nil {
		te.t.Fatal(err)
	}
	te.t.Cleanup(func() { _ = ws.CloseNow() })
	hello, _ := json.Marshal(map[string]any{"t": "hello", "epoch": epoch, "pos": pos})
	if err := ws.Write(ctx, websocket.MessageText, hello); err != nil {
		te.t.Fatal(err)
	}
	s := &sock{t: te.t, ws: ws, ctx: ctx}
	m, _, _ := s.frame()
	return s, m
}

func (s *sock) frame() (map[string]any, uint64, []byte) {
	s.t.Helper()
	typ, data, err := s.ws.Read(s.ctx)
	if err != nil {
		s.t.Fatalf("read: %v", err)
	}
	if typ == websocket.MessageBinary {
		return nil, binary.BigEndian.Uint64(data), data[8:]
	}
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	return m, 0, nil
}

// until reads output until it contains want, and returns everything read and the position after it.
func (s *sock) until(want string) (string, uint64) {
	s.t.Helper()
	var got []byte
	var end uint64
	for !bytes.Contains(got, []byte(want)) {
		m, pos, b := s.frame()
		if b != nil {
			got = append(got, b...)
			end = pos + uint64(len(b))
		} else if m != nil && m["t"] == "exit" && want != "" {
			s.t.Fatalf("terminal exited while waiting for %q: %v; got %q", want, m, got)
		}
	}
	return string(got), end
}

func (s *sock) send(text string) {
	s.t.Helper()
	if err := s.ws.Write(s.ctx, websocket.MessageBinary, []byte(text)); err != nil {
		s.t.Fatal(err)
	}
}

func TestRemoteShellSurvivesSocketReconnect(t *testing.T) {
	te := newTermEnv(t, envOpts{})
	te.mustConnect("c1", te.saveHost("password", ""))
	info, err := te.terms.OpenSSH("p1", "c1", "t1", 80, 24)
	if err != nil || info.SSH != "c1" {
		t.Fatalf("open: %+v %v", info, err)
	}
	a, hello := te.dial("t1", "", 0)
	epoch := hello["epoch"].(string)
	a.send("one\n")
	_, end := a.until("got:one")
	_ = a.ws.CloseNow()

	b, h2 := te.dial("t1", epoch, end)
	if h2["reset"] != false || h2["pos"] != float64(end) {
		t.Fatalf("resume hello: %v", h2)
	}
	b.send("two\n")
	got, _ := b.until("got:two")
	if strings.Contains(got, "got:one") {
		t.Errorf("the replay repeated old output: %q", got)
	}
	_ = b.ws.CloseNow()

	// From the start of the ring, the same shell replays everything.
	c, h3 := te.dial("t1", epoch, 0)
	if h3["reset"] != false {
		t.Fatalf("hello from 0: %v", h3)
	}
	all, _ := c.until("got:two")
	if !strings.Contains(all, "got:one") {
		t.Errorf("replay from 0 lacks the first line: %q", all)
	}
	te.terms.Close("t1")
	if n := te.fake.Execs(); len(n) != 0 {
		t.Errorf("unexpected execs: %v", n)
	}
}

func TestRemoteShellResizeAndAgentForwardOff(t *testing.T) {
	te := newTermEnv(t, envOpts{})
	te.mustConnect("c1", te.saveHost("password", ""))
	if _, err := te.terms.OpenSSH("p1", "c1", "t1", 100, 30); err != nil {
		t.Fatal(err)
	}
	s, _ := te.dial("t1", "", 0)
	s.until("fake$")
	if cols, rows := te.fake.Size(); cols != 100 || rows != 30 {
		t.Errorf("pty size %dx%d", cols, rows)
	}
	if err := s.ws.Write(s.ctx, websocket.MessageText, []byte(`{"t":"resize","cols":120,"rows":40}`)); err != nil {
		t.Fatal(err)
	}
	wait(t, "window-change", func() bool { c, r := te.fake.Size(); return c == 120 && r == 40 })
	if te.fake.AgentRequests() != 0 {
		t.Error("agent forwarding was requested for a host that does not allow it")
	}
}

func TestConnectionCloseGivesExit255(t *testing.T) {
	te := newTermEnv(t, envOpts{})
	te.mustConnect("c1", te.saveHost("password", ""))
	if _, err := te.terms.OpenSSH("p1", "c1", "t1", 80, 24); err != nil {
		t.Fatal(err)
	}
	s, _ := te.dial("t1", "", 0)
	s.until("fake$")
	te.fake.DropConnections()
	wait(t, "exit", func() bool {
		for _, i := range te.terms.Snapshot()["p1"] {
			if i.ID == "t1" && i.Exited {
				return i.Code == 255
			}
		}
		return false
	})
	wait(t, "ssh.closed", func() bool { return te.pub.count("ssh.closed") == 1 })
	if _, err := te.m.OpenShell("c1", 80, 24); code(err) != "NOT_CONNECTED" {
		t.Errorf("shell on a closed connection: %v", err)
	}
	// The output stays, and a new socket is told the shell ended with 255.
	_, h := te.dial("t1", "", 0)
	if h["exited"] != true || h["code"] != float64(255) {
		t.Errorf("hello after the end: %v", h)
	}
}

func TestDisconnectClosesShellWithExit255(t *testing.T) {
	te := newTermEnv(t, envOpts{})
	te.mustConnect("c1", te.saveHost("password", ""))
	if _, err := te.terms.OpenSSH("p1", "c1", "t1", 80, 24); err != nil {
		t.Fatal(err)
	}
	if err := te.m.Disconnect("c1"); err != nil {
		t.Fatal(err)
	}
	wait(t, "exit", func() bool {
		for _, i := range te.terms.Snapshot()["p1"] {
			if i.ID == "t1" && i.Exited {
				return i.Code == 255
			}
		}
		return false
	})
}

func TestShellExitCodeAndNoSFTP(t *testing.T) {
	te := newTermEnv(t, envOpts{fake: fakessh.Options{NoSFTP: true}})
	te.mustConnect("c1", te.saveHost("password", ""))
	if _, err := te.m.List("c1", ""); code(err) != "SFTP_UNAVAILABLE" {
		t.Errorf("list without sftp: %v", err)
	}
	if _, err := te.terms.OpenSSH("p1", "c1", "t1", 80, 24); err != nil {
		t.Fatal(err)
	}
	s, _ := te.dial("t1", "", 0)
	s.until("fake$")
	s.send("exit 3\n")
	wait(t, "exit 3", func() bool {
		for _, i := range te.terms.Snapshot()["p1"] {
			if i.ID == "t1" && i.Exited {
				return i.Code == 3
			}
		}
		return false
	})
}

func TestKeepaliveClosesDeadConnection(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.m.KeepaliveEvery = 50 * time.Millisecond
	e.m.KeepaliveWait = 100 * time.Millisecond
	e.mustConnect("c1", e.saveHost("password", ""))
	e.fake.Pause()
	wait(t, "ssh.closed", func() bool { return e.pub.count("ssh.closed") == 1 })
	e.fake.Resume()
	if _, _, _, err := e.m.Open("c1", "x"); code(err) != "NOT_CONNECTED" {
		t.Errorf("after the keepalive gave up: %v", err)
	}
}

func TestUnreachableHost(t *testing.T) {
	e := newEnv(t, envOpts{})
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	h, err := e.m.SaveHost(Host{Name: "gone", Address: "127.0.0.1", Port: port, User: "u", Auth: "password"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.connect("c1", h, standard("")); code(err) != "HOST_UNREACHABLE" {
		t.Fatalf("got %v (port %s)", err, strconv.Itoa(port))
	}
}
