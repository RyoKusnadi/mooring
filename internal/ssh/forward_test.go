package ssh

import (
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

func echoServer(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if n > 0 {
						_, _ = c.Write([]byte("echo:" + string(buf[:n])))
					}
					if err != nil {
						return
					}
				}
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestForwardCarriesBytes(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.mustConnect("c1", e.saveHost("password", ""))
	target := echoServer(t)
	f, err := e.m.Forward("c1", "127.0.0.1", 0, "127.0.0.1", target)
	if err != nil || f.LocalPort == 0 || f.Conn != "c1" {
		t.Fatalf("forward: %+v %v", f, err)
	}
	nc, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(f.LocalPort)), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = nc.Close() }()
	_ = nc.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := nc.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len("echo:hello"))
	if _, err := io.ReadFull(nc, buf); err != nil || string(buf) != "echo:hello" {
		t.Fatalf("through the forward: %q %v", buf, err)
	}
	if got := e.m.Snapshot()["sshForwards"].([]Forward); len(got) != 1 || got[0].ID != f.ID {
		t.Errorf("snapshot: %v", got)
	}
	if e.pub.count("ssh.forwarded") != 1 {
		t.Error("no ssh.forwarded event")
	}

	// A taken port.
	if _, err := e.m.Forward("c1", "127.0.0.1", f.LocalPort, "127.0.0.1", target); code(err) != "ALREADY_EXISTS" {
		t.Errorf("taken port: %v", err)
	}

	e.m.StopForward(f.ID)
	if e.pub.count("ssh.forwardStopped") != 1 {
		t.Error("no ssh.forwardStopped event")
	}
	if _, err := nc.Read(buf); err == nil {
		t.Error("the carried connection stayed open after the stop")
	}
	wait(t, "the port to close", func() bool {
		c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(f.LocalPort)), time.Second)
		if err == nil {
			_ = c.Close()
		}
		return err != nil
	})
}

func TestForwardRefusesNonLoopback(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.mustConnect("c1", e.saveHost("password", ""))
	for _, bind := range []string{"0.0.0.0", "::", "", "192.168.1.5", "localhost", "example.com"} {
		if _, err := e.m.Forward("c1", bind, 0, "127.0.0.1", 80); code(err) != "BAD_REQUEST" {
			t.Errorf("bind %q: %v", bind, err)
		}
	}
	if _, err := e.m.Forward("c1", "127.0.0.1", 0, "127.0.0.1", 0); code(err) != "BAD_REQUEST" {
		t.Errorf("remote port 0: %v", err)
	}
	if _, err := e.m.Forward("nope", "127.0.0.1", 0, "127.0.0.1", 80); code(err) != "NOT_CONNECTED" {
		t.Errorf("unknown connection: %v", err)
	}
}

func TestForwardLimitAndEndsWithConnection(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.mustConnect("c1", e.saveHost("password", ""))
	var last Forward
	for i := 0; i < MaxForwards; i++ {
		f, err := e.m.Forward("c1", "127.0.0.1", 0, "127.0.0.1", 9)
		if err != nil {
			t.Fatalf("forward %d: %v", i, err)
		}
		last = f
	}
	if _, err := e.m.Forward("c1", "127.0.0.1", 0, "127.0.0.1", 9); code(err) != "LIMIT_REACHED" {
		t.Errorf("21st forward: %v", err)
	}
	if err := e.m.Disconnect("c1"); err != nil {
		t.Fatal(err)
	}
	wait(t, "forwards to end", func() bool {
		c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(last.LocalPort)), time.Second)
		if err == nil {
			_ = c.Close()
		}
		return err != nil
	})
	if got := e.m.Snapshot()["sshForwards"].([]Forward); len(got) != 0 {
		t.Errorf("forwards outlived the connection: %v", got)
	}
}
