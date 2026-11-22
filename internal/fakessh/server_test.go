package fakessh

import (
	"io"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func dial(t *testing.T, s *Server, user string, auth ...ssh.AuthMethod) (*ssh.Client, error) {
	t.Helper()
	return ssh.Dial("tcp", s.Addr(), &ssh.ClientConfig{
		User: user, Auth: auth, HostKeyCallback: ssh.FixedHostKey(s.HostKey()), Timeout: 5 * time.Second,
	})
}

func start(t *testing.T, o Options) *Server {
	t.Helper()
	if o.HostKey == nil {
		k, err := GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		o.HostKey = k
	}
	s, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestPasswordAndKeyAuth(t *testing.T) {
	k, _ := GenerateKey()
	s := start(t, Options{User: "u", Password: "pw", AuthorizedKeys: []ssh.PublicKey{k.PublicKey()}})
	if _, err := dial(t, s, "u", ssh.Password("wrong")); err == nil {
		t.Fatal("a wrong password signed in")
	}
	if _, err := dial(t, s, "other", ssh.Password("pw")); err == nil {
		t.Fatal("a wrong user signed in")
	}
	c, err := dial(t, s, "u", ssh.Password("pw"))
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	c, err = dial(t, s, "u", ssh.PublicKeys(k))
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	if got := strings.Join(s.Attempts(), ","); !strings.Contains(got, "password") || !strings.Contains(got, "publickey") {
		t.Errorf("attempts: %s", got)
	}
}

func TestKeyboardInteractive(t *testing.T) {
	s := start(t, Options{User: "u", Questions: []Question{{Text: "Code: ", Answer: "7"}}})
	answer := func(a string) ssh.AuthMethod {
		return ssh.KeyboardInteractive(func(_, _ string, qs []string, _ []bool) ([]string, error) {
			if len(qs) != 1 || qs[0] != "Code: " {
				t.Errorf("questions: %v", qs)
			}
			return []string{a}, nil
		})
	}
	if _, err := dial(t, s, "u", answer("8")); err == nil {
		t.Error("a wrong answer signed in")
	}
	c, err := dial(t, s, "u", answer("7"))
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
}

func shell(t *testing.T, c *ssh.Client) (*ssh.Session, io.WriteCloser, io.Reader) {
	t.Helper()
	sess, err := c.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	in, _ := sess.StdinPipe()
	out, _ := sess.StdoutPipe()
	if err := sess.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{}); err != nil {
		t.Fatal(err)
	}
	if err := sess.Shell(); err != nil {
		t.Fatal(err)
	}
	return sess, in, out
}

func readUntil(t *testing.T, r io.Reader, want string) string {
	t.Helper()
	var sb strings.Builder
	buf := make([]byte, 512)
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(sb.String(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("waited for %q, got %q", want, sb.String())
		}
		n, err := r.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	if !strings.Contains(sb.String(), want) {
		t.Fatalf("waited for %q, got %q", want, sb.String())
	}
	return sb.String()
}

func TestEchoShellExitStatus(t *testing.T) {
	s := start(t, Options{User: "u", Password: "pw"})
	c, err := dial(t, s, "u", ssh.Password("pw"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	sess, in, out := shell(t, c)
	readUntil(t, out, "fake$")
	_, _ = io.WriteString(in, "hi\n")
	readUntil(t, out, "got:hi")
	_, _ = io.WriteString(in, "exit 4\n")
	err = sess.Wait()
	if ee, ok := err.(*ssh.ExitError); !ok || ee.ExitStatus() != 4 {
		t.Errorf("wait: %v", err)
	}
}

func TestRealShellOnPTY(t *testing.T) {
	s := start(t, Options{User: "u", Password: "pw", Shell: "sh", Home: t.TempDir()})
	c, err := dial(t, s, "u", ssh.Password("pw"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_, in, out := shell(t, c)
	_, _ = io.WriteString(in, "echo $((6*7))ok\n")
	readUntil(t, out, "42ok")
}

func TestDropConnectionsKeepsListening(t *testing.T) {
	s := start(t, Options{User: "u", Password: "pw"})
	c, err := dial(t, s, "u", ssh.Password("pw"))
	if err != nil {
		t.Fatal(err)
	}
	s.DropConnections()
	if err := c.Wait(); err == nil {
		t.Log("closed cleanly")
	}
	c2, err := dial(t, s, "u", ssh.Password("pw"))
	if err != nil {
		t.Fatalf("after a drop: %v", err)
	}
	_ = c2.Close()
}
