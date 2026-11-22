package accounts

import (
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strings"
)

// Usage is what Mooring knows about an account's limits.
type Usage struct {
	Source        string   `json:"source"` // "agent" once data came in, else "unknown"
	Windows       []Window `json:"windows"`
	LimitHit      int64    `json:"limitHit"`      // when the agent last refused a turn for a limit (ms), 0 if not
	LimitResetsAt int64    `json:"limitResetsAt"` // when that limit ends (ms), 0 if unknown
	UpdatedAt     int64    `json:"updatedAt"`
}

// Window is one limit window of the agent: five hours, seven days, ...
type Window struct {
	Name        string   `json:"name"`
	UsedPercent *float64 `json:"usedPercent"` // null when the agent did not say
	ResetsAt    int64    `json:"resetsAt"`    // ms, 0 when unknown
	Status      string   `json:"status"`
}

const maxWindows = 8

var (
	nameOK   = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)
	limitRE  = regexp.MustCompile(`(?i)usage limit|rate.?limit|limit reached|hit your limit|(?:quota|usage) (?:exceeded|exhausted)|exceeded your (?:current )?quota|too many requests|\b429\b.{0,20}(?:too many|rate|limit)`)
	statusOK = map[string]bool{"allowed": true, "allowed_warning": true, "rejected": true}
)

// LimitText reports whether an agent's error text says a limit was hit.
func LimitText(text string) bool { return limitRE.MatchString(text) }

// readWindow reads the usage object of the agent the reader names. For Claude it
// is `_meta["_claude/rateLimit"]` of a usage_update: one window per update, with
// `status`, `resetsAt` (Unix seconds), `rateLimitType` and `utilization` (a
// fraction; the adapter passes it through from the Claude SDK). Other fields are
// dropped.
func readWindow(kind string, raw json.RawMessage, now int64) (Window, bool) {
	if kind != "claude" {
		return Window{}, false
	}
	var in struct {
		Status        string   `json:"status"`
		ResetsAt      float64  `json:"resetsAt"`
		RateLimitType string   `json:"rateLimitType"`
		Utilization   *float64 `json:"utilization"`
	}
	if json.Unmarshal(raw, &in) != nil {
		return Window{}, false
	}
	w := Window{Name: strings.ToLower(in.RateLimitType)}
	if w.Name == "" {
		w.Name = "limit"
	}
	if !nameOK.MatchString(w.Name) {
		return Window{}, false
	}
	if statusOK[in.Status] {
		w.Status = in.Status
	}
	ms := in.ResetsAt * 1000
	if in.ResetsAt > 1e11 { // already milliseconds
		ms = in.ResetsAt
	}
	w.ResetsAt = boundReset(ms, now)
	if u := in.Utilization; u != nil && *u >= 0 {
		p := math.Min(*u*100, 100) // a fraction of the limit
		if math.IsNaN(p) {
			p = 100
		}
		w.UsedPercent = &p
	} else if w.Status == "rejected" {
		p := 100.0
		w.UsedPercent = &p
	}
	return w, true
}

// boundReset keeps a reset time (ms) only when it is between a day ago and 60 days
// ahead; anything else, including NaN and huge numbers, reads as unknown (0).
func boundReset(ms float64, now int64) int64 {
	lo, hi := float64(now-24*3600*1000), float64(now+60*24*3600*1000)
	if math.IsNaN(ms) || ms < lo || ms > hi {
		return 0
	}
	return int64(ms)
}

// merge puts w into the list, replacing the window of the same name.
func (u *Usage) merge(w Window, now int64) {
	for i := range u.Windows {
		if u.Windows[i].Name == w.Name {
			if sameWindow(u.Windows[i], w) {
				return
			}
			u.Windows[i] = w
			u.Source, u.UpdatedAt = "agent", now
			return
		}
	}
	if len(u.Windows) < maxWindows {
		u.Windows = append(u.Windows, w)
		sort.Slice(u.Windows, func(i, j int) bool { return u.Windows[i].Name < u.Windows[j].Name })
	}
	u.Source, u.UpdatedAt = "agent", now
}

// tightest is the highest used percentage of any window, or -1 when none is known.
func (u Usage) tightest() float64 {
	t := -1.0
	for _, w := range u.Windows {
		if w.UsedPercent != nil && *w.UsedPercent > t {
			t = *w.UsedPercent
		}
	}
	return t
}

// limited reports a limit hit that has not ended yet.
func (u Usage) limited(now int64) bool {
	if u.LimitHit == 0 {
		return false
	}
	return u.LimitResetsAt == 0 || u.LimitResetsAt > now
}

func newUsage() Usage { return Usage{Source: "unknown", Windows: []Window{}} }

func sameWindow(a, b Window) bool {
	if a.Name != b.Name || a.ResetsAt != b.ResetsAt || a.Status != b.Status || (a.UsedPercent == nil) != (b.UsedPercent == nil) {
		return false
	}
	return a.UsedPercent == nil || *a.UsedPercent == *b.UsedPercent
}
