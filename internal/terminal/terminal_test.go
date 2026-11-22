package terminal

import (
	"strings"
	"testing"
	"time"

	"github.com/ryokusnadi/mooring/internal/ledger"
)

func TestReplayFromPosition(t *testing.T) {
	m, _ := newManager(t, script("printf 'hello '; sleep 0.2; printf 'world'; sleep 30"))
	term := open(t, m, "a")
	waitFor(t, "output", func() bool { return term.pos() >= 11 })

	c, f := term.attach(term.stream.Epoch(), 6)
	if f.Reset || f.Pos != 6 || f.Modes != "" {
		t.Fatalf("resume: %+v", f)
	}
	if data, _, _ := drain(c); data != "world" {
		t.Errorf("replayed %q, want %q", data, "world")
	}
	// A client that is current gets nothing more until output arrives.
	c2, f2 := term.attach(term.stream.Epoch(), term.pos())
	if data, _, _ := drain(c2); f2.Reset || data != "" {
		t.Errorf("current client got %q reset=%v", data, f2.Reset)
	}
	// Another epoch, or a position ahead, starts from a reset.
	for _, tc := range []struct {
		epoch string
		pos   uint64
	}{{"", 0}, {"stale", 6}, {term.stream.Epoch(), 9999}} {
		cr, fr := term.attach(tc.epoch, tc.pos)
		if data, _, _ := drain(cr); !fr.Reset || data != "hello world" {
			t.Errorf("attach(%q,%d): reset=%v data=%q", tc.epoch, tc.pos, fr.Reset, data)
		}
	}
}

func TestResetSendsStartModes(t *testing.T) {
	cfg := script("printf '\\033[?1049h\\033[?25l'; " + fmtLines(300) + "; sleep 30")
	cfg.RingSize = 4096
	m, _ := newManager(t, cfg)
	term := open(t, m, "a")
	waitFor(t, "ring to wrap", func() bool { return term.pos() > 10000 })
	for last := uint64(0); term.pos() != last; { // let the output finish
		last = term.pos()
		time.Sleep(100 * time.Millisecond)
	}

	c, f := term.attach("", 0)
	if !f.Reset || f.Pos == 0 {
		t.Fatalf("hello: %+v", f)
	}
	if !contains(f.Modes, "\x1b[?25l", "\x1b[?1049h") {
		t.Errorf("modes %q lack the alternate screen and the hidden cursor", f.Modes)
	}
	data, _, _ := drain(c)
	if len(data) == 0 || data[0]&0xC0 == 0x80 {
		t.Errorf("replay starts mid-character: %q", data[:min(len(data), 8)])
	}
	if c.cursor != term.pos() || term.pos()-f.Pos != uint64(len(data)) {
		t.Errorf("positions: start %d sent %d pos %d", f.Pos, len(data), term.pos())
	}
}

func TestClientBehindRingIsReset(t *testing.T) {
	cfg := script("sleep 0.3; " + fmtLines(300) + "; sleep 30")
	cfg.RingSize = 4096
	m, _ := newManager(t, cfg)
	term := open(t, m, "a")
	c, f := term.attach("", 0)
	if f.Reset && f.Pos != 0 {
		t.Fatalf("hello: %+v", f)
	}
	waitFor(t, "ring to wrap", func() bool { return term.pos() > 10000 })
	_, state, _ := drain(c)
	if state == nil || state.T != "reset" || !state.Reset {
		t.Fatalf("a client past the ring must get a reset frame, got %+v", state)
	}
}

func TestRepaintSignalsForegroundGroup(t *testing.T) {
	m, _ := newManager(t, script("trap 'echo WINCHED' WINCH; printf '\\033[?1049h'; while :; do sleep 0.05; done"))
	term := open(t, m, "a")
	waitFor(t, "alternate screen", func() bool { term.mu.Lock(); defer term.mu.Unlock(); return term.cur.alt() })
	term.attach("", 0)
	waitFor(t, "repaint", func() bool { return strings.Contains(term.ringText(), "WINCHED") })
}

func TestCloseKillsProcessGroup(t *testing.T) {
	m, rec := newManager(t, Config{Args: []string{"-i"}})
	term := open(t, m, "a")
	shellGroup := term.b.(*localBackend).pid()
	term.input(&client{}, []byte("sleep 300\n"))
	var fg int
	waitFor(t, "a foreground program in its own group", func() bool {
		fg, _ = term.b.(*localBackend).fgGroup()
		return fg > 0 && fg != shellGroup
	})
	m.Close("a")
	waitFor(t, "groups to end", func() bool { return groupGone(fg) && groupGone(shellGroup) })
	if !rec.has("terminal.closed") || m.get("a") != nil {
		t.Error("closed terminal must be forgotten and announced")
	}
}

func TestShutdownKillsEveryGroup(t *testing.T) {
	m, _ := newManager(t, script("sleep 300 & wait"))
	var groups []int
	for _, id := range []string{"a", "b"} {
		groups = append(groups, open(t, m, id).b.(*localBackend).pid())
	}
	m.Shutdown()
	// Shutdown waits for each shell; a background child got the same hangup but may take a moment to end.
	for _, pg := range groups {
		waitFor(t, "process group to end after shutdown", func() bool { return groupGone(pg) })
	}
}

func TestShutdownKillsShellThatIgnoresHangup(t *testing.T) {
	cfg := script("trap '' HUP; sleep 300")
	cfg.KillGrace = 100 * time.Millisecond
	m, _ := newManager(t, cfg)
	pg := open(t, m, "a").b.(*localBackend).pid()
	time.Sleep(200 * time.Millisecond)
	m.Shutdown()
	// On Linux a killed child stays in the group until it is reaped, so wait instead of checking once.
	waitFor(t, "a shell that ignores SIGHUP to be killed", func() bool { return groupGone(pg) })
}

func TestLimits(t *testing.T) {
	m, _ := newManager(t, script("sleep 300"))
	for i := range MaxPerProject {
		if _, err := m.Open("p1", "", "a"+string(rune('a'+i)), 80, 24); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}
	expect := func(err error, code string) {
		t.Helper()
		if e, ok := err.(*ledger.Error); !ok || e.Code != code {
			t.Errorf("got %v, want %s", err, code)
		}
	}
	_, err := m.Open("p1", "", "extra", 80, 24)
	expect(err, "LIMIT_REACHED")
	_, err = m.Open("p1", "", "aa", 80, 24)
	expect(err, "ALREADY_EXISTS")
	_, err = m.Open("p1", "", "bad id!", 80, 24)
	expect(err, "BAD_REQUEST")
	_, err = m.Open("p1", "", "tiny", 1, 24)
	expect(err, "BAD_REQUEST")
	for _, p := range []string{"p2", "p3"} {
		for i := range MaxPerProject {
			if _, err := m.Open(p, "", p+string(rune('a'+i)), 80, 24); err != nil {
				t.Fatalf("open %s %d: %v", p, i, err)
			}
		}
	}
	_, err = m.Open("p4", "", "one-too-many", 80, 24)
	expect(err, "LIMIT_REACHED")
	// Closing one makes room.
	m.Close("p3a")
	if _, err := m.Open("p4", "", "fits", 80, 24); err != nil {
		t.Errorf("after a close: %v", err)
	}
}

func TestFlowControlPausesAndResumes(t *testing.T) {
	cfg := script("yes")
	cfg.FlowWindow = 64 << 10
	m, _ := newManager(t, cfg)
	term := open(t, m, "a")
	c, _ := term.attach(term.stream.Epoch(), 0)

	// The browser draws nothing: the shell is held back near the window.
	waitFor(t, "output", func() bool { return term.pos() > 64<<10 })
	time.Sleep(300 * time.Millisecond)
	stuck := term.pos()
	time.Sleep(300 * time.Millisecond)
	if term.pos() != stuck {
		t.Fatalf("output kept coming without acks: %d then %d", stuck, term.pos())
	}
	if stuck > 64<<10+512<<10 {
		t.Errorf("paused at %d, far past the 64 KiB window", stuck)
	}

	// Drawing and acking lets it run again.
	drain(c)
	term.ack(c, c.cursor)
	waitFor(t, "output to resume", func() bool { return term.pos() > stuck+32<<10 })

	// A second client that never acks does not hold the shell back once the
	// first is the active one again.
	term.attach(term.stream.Epoch(), term.pos())
	term.input(c, []byte("x"))
	for range 3 {
		drain(c)
		term.ack(c, c.cursor)
		time.Sleep(50 * time.Millisecond)
	}
	before := term.pos()
	time.Sleep(100 * time.Millisecond)
	drain(c)
	term.ack(c, c.cursor)
	waitFor(t, "output with the active client acking", func() bool { return term.pos() > before })
}

func TestExitKeepsOutputThenSweeps(t *testing.T) {
	now := time.Now()
	cfg := script("echo bye; exit 3")
	cfg.ExitedKeep = time.Minute
	cfg.Now = func() time.Time { return now }
	m, rec := newManager(t, cfg)
	term := open(t, m, "a")
	<-term.done
	if info := term.info(); !info.Exited || info.Code != 3 {
		t.Fatalf("info %+v", info)
	}
	if !rec.has("terminal.exited") {
		t.Error("no exited event")
	}
	c, f := term.attach("", 0)
	data, _, exit := drain(c)
	if !f.Exited || !strings.Contains(data, "bye") || exit == nil || exit.Code != 3 {
		t.Errorf("replay of an exited terminal: %+v %q %+v", f, data, exit)
	}
	m.sweep()
	if len(m.Snapshot()["p1"]) != 1 {
		t.Fatal("swept too early")
	}
	now = now.Add(2 * time.Minute)
	m.sweep()
	if len(m.Snapshot()) != 0 || !rec.has("terminal.closed") {
		t.Error("an exited terminal must go after the keep time")
	}
}

func TestExitedTerminalsDoNotCountAgainstLimits(t *testing.T) {
	m, _ := newManager(t, script("exit 0"))
	for i := range MaxPerProject + 2 {
		term := open(t, m, "e"+string(rune('a'+i)))
		<-term.done
	}
}

func TestChildSeesTerminalType(t *testing.T) {
	m, _ := newManager(t, script("echo type=$TERM; pwd; sleep 30"))
	term := open(t, m, "a")
	waitFor(t, "output", func() bool { return strings.Contains(term.ringText(), "type=xterm-256color") })
	if !strings.Contains(term.ringText(), "\n") {
		t.Error("no working directory printed")
	}
}

func TestOpenWithSetsEnvAndShowsBanner(t *testing.T) {
	m, _ := newManager(t, script("printf '[%s]' \"$ACCT_DIR\"; sleep 30"))
	if err := m.OpenWith("p1", "login-1", []string{"ACCT_DIR=/some/folder"}, "Sign in here\r\n"); err != nil {
		t.Fatal(err)
	}
	term := m.get("login-1")
	waitFor(t, "output", func() bool { return strings.Contains(term.ringText(), "[/some/folder]") })
	if !strings.HasPrefix(term.ringText(), "Sign in here\r\n") {
		t.Fatalf("banner missing: %q", term.ringText())
	}
}

func TestRemoteShellRefusesEnvAndBanner(t *testing.T) {
	m, _ := newManager(t, script("sleep 30"))
	_, err := m.open(openReq{projectID: "p1", ssh: "conn", id: "r1", env: []string{"A=b"}})
	if err == nil || !strings.Contains(err.Error(), "BAD_REQUEST") {
		t.Fatalf("remote shell with env: %v", err)
	}
}
