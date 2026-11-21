// Package compact shortens long command output before an agent reads it. It keeps the start, the end and the
// lines that look like errors, and says how much was cut.
package compact

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Limits of the filter. They count lines and bytes, not tokens, so a tokenizer
// that turns the same text into more tokens does not change them. Whether they
// should be recounted is unverified and waits for the bench, which needs token approval.
const (
	MinLines    = 200      // output at or under this many lines and MinBytes is left alone
	MinBytes    = 16 << 10 // see MinLines
	HeadLines   = 40       // lines kept from the start
	TailLines   = 80       // lines kept from the end
	ErrorLines  = 60       // error lines kept from the middle
	MaxLineLen  = 400      // longer lines are cut
	MaxResult   = 16 << 10 // the result is cut to this size
	repeatAfter = 3        // identical lines in a row that collapse
	headShare   = 6 << 10  // of MaxResult, when the result still is too long
)

var (
	ansiCSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	ansiOSC = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
	errLine = regexp.MustCompile(`(?i)error|fail|panic|exception|fatal|traceback|^\s+at |:\d+:\d+`)
)

// Result describes one run of the filter.
type Result struct {
	Lines   int  // lines of the input
	Bytes   int  // bytes of the input
	Kept    int  // lines of the output, without the closing note
	Applied bool // false when the input was short and came back unchanged
}

// CountLines counts the lines of text; a last line without a newline counts.
func CountLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// Apply returns the compacted text. Output of at most 200 lines and 16 KB is
// returned as it is.
func Apply(in string) (string, Result) {
	res := Result{Lines: CountLines(in), Bytes: len(in)}
	if res.Lines <= MinLines && res.Bytes <= MinBytes {
		res.Kept = res.Lines
		return in, res
	}
	res.Applied = true
	lines := collapse(clean(in))
	lines = pick(lines)
	for i, l := range lines {
		lines[i] = cutLine(l)
	}
	out := strings.Join(lines, "\n")
	if len(out) > MaxResult {
		out = squeeze(out)
	}
	res.Kept = CountLines(out)
	cut := max(res.Lines-res.Kept, 0)
	out += fmt.Sprintf("\n[mooring: %d of %d lines cut; ask the user for the full output]", cut, res.Lines)
	return out, res
}

// clean strips terminal codes and keeps the last version of lines rewritten with \r.
func clean(in string) []string {
	in = ansiOSC.ReplaceAllString(ansiCSI.ReplaceAllString(in, ""), "")
	in = strings.TrimSuffix(in, "\n")
	lines := strings.Split(in, "\n")
	for i, l := range lines {
		l = strings.TrimSuffix(l, "\r")
		if j := strings.LastIndex(l, "\r"); j >= 0 {
			l = l[j+1:]
		}
		lines[i] = l
	}
	return lines
}

// collapse turns three or more identical lines in a row into the line and a count.
func collapse(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		j := i
		for j < len(lines) && lines[j] == lines[i] {
			j++
		}
		out = append(out, lines[i])
		if n := j - i; n >= repeatAfter {
			out = append(out, fmt.Sprintf("[repeated %d times]", n))
		} else {
			out = append(out, lines[i+1:j]...)
		}
		i = j
	}
	return out
}

// pick keeps the head, the tail and the error lines of the middle with one line of context.
func pick(lines []string) []string {
	if len(lines) <= HeadLines+TailLines {
		return lines
	}
	keep := make([]bool, len(lines))
	for i := 0; i < HeadLines; i++ {
		keep[i] = true
	}
	for i := len(lines) - TailLines; i < len(lines); i++ {
		keep[i] = true
	}
	found := 0
	for i := HeadLines; i < len(lines)-TailLines && found < ErrorLines; i++ {
		if errLine.MatchString(lines[i]) {
			found++
			keep[i], keep[i-1], keep[i+1] = true, true, true
		}
	}
	out := make([]string, 0, HeadLines+TailLines+found*3)
	for i, k := range keep {
		if k {
			out = append(out, lines[i])
		}
	}
	return out
}

func cutLine(l string) string {
	if len(l) <= MaxLineLen {
		return l
	}
	l = l[:MaxLineLen]
	for !utf8.ValidString(l) && len(l) > 0 {
		l = l[:len(l)-1]
	}
	return l
}

// squeeze cuts text over MaxResult to its start and its end, on line borders.
func squeeze(s string) string {
	head := s[:headShare]
	if i := strings.LastIndex(head, "\n"); i > 0 {
		head = head[:i]
	}
	tail := s[len(s)-(MaxResult-headShare):]
	if i := strings.Index(tail, "\n"); i >= 0 {
		tail = tail[i+1:]
	}
	return head + "\n" + tail
}
