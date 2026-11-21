package accounts

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestUsageReaderAndSwitchChoice(t *testing.T) {
	r := newRig(t)
	soon := time.Now().Add(time.Hour).Unix()
	a, b, c := r.save("A"), r.save("B"), r.save("C")
	for _, x := range []Account{a, b, c} {
		r.s.Ran("claude", x.ID)
	}
	raw := func(status, kind string, u float64) json.RawMessage {
		out, _ := json.Marshal(map[string]any{"status": status, "rateLimitType": kind, "utilization": u, "resetsAt": soon, "extra": "dropped"})
		return out
	}
	r.s.RateLimit("claude", a.ID, raw("allowed", "five_hour", 0.2))
	r.s.RateLimit("claude", a.ID, raw("allowed", "seven_day", 0.9))
	r.s.RateLimit("claude", b.ID, raw("allowed", "five_hour", 0.5))
	if got := r.s.SwitchFor("claude", "default:claude"); got != b.ID && got != c.ID {
		t.Fatalf("switch: %q", got)
	}
	if got := r.s.SwitchFor("claude", c.ID); got != b.ID {
		t.Fatalf("known use comes before unknown, tightest window counts: want %q got %q", b.ID, got)
	}
	if rej, at := r.s.RateLimit("claude", b.ID, raw("rejected", "five_hour", 1)); !rej || at != soon*1000 {
		t.Fatalf("rejected window: %v %d", rej, at)
	}
	r.s.LimitHit("claude", b.ID, soon*1000)
	if got := r.s.SwitchFor("claude", c.ID); got != a.ID {
		t.Fatalf("a limited account must not be offered: %q", got)
	}
	r.s.NeedsLogin("claude", a.ID)
	r.s.Ran("claude", "default:claude")
	if got := r.s.SwitchFor("claude", c.ID); got != "default:claude" {
		t.Fatalf("an account that needs a login must not be offered: %q", got)
	}
	r.s.Ran("claude", b.ID)
	body, _ := r.s.Snapshot()
	for _, x := range body["accounts"].([]Account) {
		if x.ID == b.ID && x.Usage.LimitHit != 0 {
			t.Fatalf("a turn that ran should clear the limit: %+v", x.Usage)
		}
	}
	rawJSON, _ := json.Marshal(body)
	if strings.Contains(string(rawJSON), "dropped") {
		t.Fatal("unknown agent fields were kept")
	}
	r.s.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	r.s.LimitHit("claude", a.ID, soon*1000)
	body, _ = r.s.Snapshot()
	for _, x := range body["accounts"].([]Account) {
		if x.ID == a.ID && x.Usage.LimitHit != 0 {
			t.Fatalf("a limit past its reset should read as over: %+v", x.Usage)
		}
	}
}

func TestLimitText(t *testing.T) {
	for _, s := range []string{"Claude usage limit reached", "You've hit your limit", "429 Too Many Requests", "rate limit exceeded"} {
		if !LimitText(s) {
			t.Fatalf("%q not read as a limit", s)
		}
	}
	for _, s := range []string{"Authentication required", "cannot switch"} {
		if LimitText(s) {
			t.Fatalf("%q read as a limit", s)
		}
	}
}

func TestWindowBoundsAndPercent(t *testing.T) {
	now := time.Now().UnixMilli()
	read := func(body string) Window {
		w, ok := readWindow("claude", json.RawMessage(body), now)
		if !ok {
			t.Fatalf("not read: %s", body)
		}
		return w
	}
	soon := time.Now().Add(time.Hour).Unix()
	if w := read(`{"status":"allowed","rateLimitType":"five_hour","resetsAt":` + itoa(soon) + `}`); w.ResetsAt != soon*1000 {
		t.Fatalf("good reset lost: %+v", w)
	}
	for _, at := range []string{"1e300", "1", "-5", "1e12", "4102444800", itoa(time.Now().Add(-48 * time.Hour).Unix())} {
		if w := read(`{"status":"rejected","rateLimitType":"x","resetsAt":` + at + `}`); w.ResetsAt != 0 {
			t.Fatalf("reset %s should read as unknown: %d", at, w.ResetsAt)
		}
	}
	for in, want := range map[string]float64{"0.5": 50, "1": 100, "1.5": 100, "0.01": 1, "7": 100} {
		w := read(`{"status":"allowed","rateLimitType":"x","utilization":` + in + `}`)
		if w.UsedPercent == nil || *w.UsedPercent != want {
			t.Fatalf("utilization %s read as %v, want %v", in, w.UsedPercent, want)
		}
	}
}

func TestLimitedAccountWithUnknownResetIsNotOffered(t *testing.T) {
	r := newRig(t)
	a, b := r.save("A"), r.save("B")
	r.s.Ran("claude", a.ID)
	r.s.Ran("claude", b.ID)
	r.s.LimitHit("claude", a.ID, 1) // far in the past: unknown, so still limited
	if got := r.s.SwitchFor("claude", b.ID); got == a.ID {
		t.Fatalf("offered an account that is limited: %q", got)
	}
}

func TestLimitTextIsSpecific(t *testing.T) {
	for _, s := range []string{"the quota file is missing", "listening on port 429", "request 4290 failed"} {
		if LimitText(s) {
			t.Fatalf("%q read as a limit", s)
		}
	}
	for _, s := range []string{"API Error: 429 Too Many Requests", "You exceeded your current quota", "usage limit reached"} {
		if !LimitText(s) {
			t.Fatalf("%q not read as a limit", s)
		}
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
