package files

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/paths"
)

// Search limits.
const (
	maxSearches   = 4
	maxQuery      = 500
	defaultLimit  = 200
	maxLimit      = 1000
	perFileMatch  = 50
	maxSearchFile = 2 << 20
	maxLineUnits  = 300
	searchTimeout = 30 * time.Second
)

// SearchRequest is one search.
type SearchRequest struct {
	ProjectID string
	Query     string
	Regex     bool
	Case      bool
	Glob      string
	Path      string
	Limit     int
}

// SearchLine is one line of the result stream.
type SearchLine struct {
	T         string   `json:"t"`
	Seq       int      `json:"seq"`
	Path      string   `json:"path,omitempty"`
	Line      int      `json:"line,omitempty"`
	Col       int      `json:"col,omitempty"`
	Text      string   `json:"text,omitempty"`
	Ranges    [][2]int `json:"ranges,omitempty"`
	Count     int      `json:"count"`
	Truncated bool     `json:"truncated"`
	Engine    string   `json:"engine,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Code      string   `json:"code,omitempty"`
	Message   string   `json:"message,omitempty"`
}

// searchRun is the state both engines share.
type searchRun struct {
	s     *Service
	ctx   context.Context
	req   SearchRequest
	root  string // real project folder
	dir   string // real folder searched, inside root
	re    *regexp.Regexp
	glob  *rule
	emit  func(SearchLine) error
	seq   int
	count int
	limit int
	full  bool // stopped at the limit

	emitErr error // the reader of the stream is gone
}

// Search runs a search and calls emit for each line. It returns when the search
// is done, cancelled (ctx) or emit fails. Errors in the query come back as the
// stream's "error" line and a nil error; the caller only sees failures to emit.
func (s *Service) Search(ctx context.Context, req SearchRequest, emit func(SearchLine) error) error {
	r := &searchRun{s: s, req: req, emit: emit}
	if code, msg := r.prepare(); code != "" {
		return emit(SearchLine{T: "error", Seq: 1, Code: code, Message: msg})
	}
	select {
	case s.srch <- struct{}{}:
		defer func() { <-s.srch }()
	default:
		return emit(SearchLine{T: "error", Seq: 1, Code: "SEARCH_BUSY", Message: "Four searches are running. Try again in a moment."})
	}
	var cancel context.CancelFunc
	r.ctx, cancel = context.WithTimeout(ctx, searchTimeout)
	defer cancel()
	engine := "walk"
	var err error
	if s.rg != "" {
		engine = "rg"
		err = r.runRG()
	} else {
		err = r.runWalk()
	}
	if r.emitErr != nil {
		return r.emitErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		if se, ok := err.(*searchErr); ok {
			return emit(SearchLine{T: "error", Seq: r.seq + 1, Code: se.code, Message: se.msg})
		}
		return err
	}
	done := SearchLine{T: "done", Seq: r.seq + 1, Count: r.count, Truncated: r.full, Engine: engine}
	if r.ctx.Err() != nil {
		done.Truncated, done.Reason = true, "timeout"
	}
	return emit(done)
}

type searchErr struct{ code, msg string }

func (e *searchErr) Error() string { return e.code + ": " + e.msg }

// prepare checks the request and resolves the folder. It returns an error code for the stream.
func (r *searchRun) prepare() (code, msg string) {
	q := r.req.Query
	if n := utf8.RuneCountInString(q); n == 0 || n > maxQuery {
		return "BAD_QUERY", "The search text must be 1 to 500 characters."
	}
	root, err := r.s.root(r.req.ProjectID)
	if err != nil {
		return errCode(err)
	}
	dir, err := r.s.follow(root, r.req.Path)
	if err != nil {
		return errCode(err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "NOT_A_FOLDER", "The folder to search does not exist."
	}
	r.root, r.dir = root, dir
	expr := q
	if !r.req.Regex {
		expr = regexp.QuoteMeta(q)
	}
	if !r.req.Case {
		expr = "(?i)" + expr
	}
	if r.re, err = regexp.Compile(expr); err != nil {
		return "BAD_QUERY", "That is not a valid pattern: " + err.Error()
	}
	if g := strings.TrimSpace(r.req.Glob); g != "" {
		gr, ok := parseRule(g)
		if !ok {
			return "BAD_QUERY", "That file pattern is not valid."
		}
		r.glob = &gr
	}
	r.limit = r.req.Limit
	if r.limit <= 0 {
		r.limit = defaultLimit
	}
	r.limit = min(r.limit, maxLimit)
	return "", ""
}

func errCode(err error) (string, string) {
	var le *ledger.Error
	if errors.As(pathErr(err), &le) {
		return le.Code, le.Message
	}
	return "INTERNAL", "The search could not start."
}

// add sends one matching line, after the path check. It reports false at the limit.
func (r *searchRun) add(rel string, line int, text string, byteRanges [][2]int) bool {
	if r.count >= r.limit {
		r.full = true
		return false
	}
	// Links are never followed, but check anyway: a result must resolve inside the project.
	if real, err := r.s.follow(r.root, rel); err != nil || !paths.Inside(r.root, real) {
		return true
	}
	units := func(off int) int { return len(utf16.Encode([]rune(text[:off]))) }
	col := 1
	if len(byteRanges) > 0 {
		col = units(byteRanges[0][0]) + 1
	}
	ranges := make([][2]int, 0, len(byteRanges))
	for _, br := range byteRanges {
		ranges = append(ranges, [2]int{units(br[0]), units(br[1])})
	}
	t16 := utf16.Encode([]rune(text))
	if len(t16) > maxLineUnits {
		from := max(col-1-80, 0)
		to := min(from+maxLineUnits, len(t16))
		kept := ranges[:0]
		for _, rg := range ranges {
			if rg[0] >= from && rg[1] <= to {
				kept = append(kept, [2]int{rg[0] - from, rg[1] - from})
			}
		}
		ranges = kept
		text = string(utf16.Decode(t16[from:to]))
	}
	r.seq++
	r.count++
	l := SearchLine{T: "match", Seq: r.seq, Path: rel, Line: line, Col: col, Text: strings.TrimRight(text, "\r\n"), Ranges: ranges}
	if r.emitErr = r.emit(l); r.emitErr != nil {
		return false
	}
	return true
}

func findRG(cfg string) string {
	if cfg == "-" {
		return ""
	}
	if cfg != "" {
		return cfg
	}
	p, err := exec.LookPath("rg")
	if err != nil {
		return ""
	}
	return p
}

func relToRoot(root, real string) string {
	r, _ := filepath.Rel(root, real)
	return filepath.ToSlash(r)
}
