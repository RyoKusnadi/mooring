package git

import (
	"fmt"
	"strings"
)

// patchSpec says which lines of a diff to turn into a patch.
type patchSpec struct {
	file    rawFile
	path    string
	kind    string // "new", "deleted" or "" for a changed file
	mode    string // the file mode of a new or deleted file
	chosen  map[[2]int]bool
	reverse bool // the diff is HEAD to index and the patch is applied backwards
}

// fileKind reads what the lines before the first hunk say about the file.
func fileKind(header []string) (kind, mode string, modeChange bool) {
	for _, l := range header {
		switch {
		case strings.HasPrefix(l, "new file mode "):
			kind, mode = "new", strings.TrimPrefix(l, "new file mode ")
		case strings.HasPrefix(l, "deleted file mode "):
			kind, mode = "deleted", strings.TrimPrefix(l, "deleted file mode ")
		case strings.HasPrefix(l, "old mode "), strings.HasPrefix(l, "new mode "):
			modeChange = true
		}
	}
	return kind, mode, modeChange
}

// build makes the patch. full is true when every changed line is chosen, in
// which case the patch is the whole diff of the file.
//
// The target side is the file the patch is applied to: the old side when
// staging, the new side when unstaging (git apply --reverse). On that side a
// changed line that is not chosen stays as context. On the other side it is
// left out.
func (ps *patchSpec) build() (patch string, full bool, err error) {
	target, other := byte('-'), byte('+')
	if ps.reverse {
		target, other = '+', '-'
	}
	full = true
	var body strings.Builder
	delta, changes := 0, 0
	for hi, h := range ps.file.hunks {
		var lines []string
		tCount, oCount, chosenHere := 0, 0, 0
		prevKept, noEOLCtx := false, false
		for li, l := range h.raw {
			k := l[0]
			switch k {
			case '\\':
				if prevKept {
					lines = append(lines, l)
				}
				continue
			case ' ':
				lines = append(lines, l)
				tCount, oCount = tCount+1, oCount+1
				prevKept = true
				continue
			}
			pick := ps.chosen[[2]int{hi, li}]
			full = full && pick
			switch {
			case k == target && pick:
				lines, prevKept = append(lines, l), true
				tCount++
			case k == target:
				lines, prevKept = append(lines, " "+l[1:]), true
				tCount, oCount = tCount+1, oCount+1
				noEOLCtx = noEOLCtx || (li+1 < len(h.raw) && h.raw[li+1][0] == '\\')
			case k == other && pick:
				if noEOLCtx {
					return "", false, apiErr("BAD_REQUEST", "The line before it has no newline at the end of the file. Choose that line change too, or stage the whole file.")
				}
				lines, prevKept = append(lines, l), true
				oCount++
			default:
				prevKept = false
			}
			if pick {
				chosenHere++
			}
		}
		if chosenHere == 0 {
			continue
		}
		changes += chosenHere
		tStart, tOrig := h.OldStart, h.OldLines
		if ps.reverse {
			tStart, tOrig = h.NewStart, h.NewLines
		}
		first := tStart
		if tOrig == 0 {
			first++
		}
		oStart := first + delta
		if oCount == 0 {
			oStart--
		}
		delta += oCount - tCount
		oldS, oldC, newS, newC := tStart, tCount, oStart, oCount
		if ps.reverse {
			oldS, oldC, newS, newC = oStart, oCount, tStart, tCount
		}
		fmt.Fprintf(&body, "@@ -%d,%d +%d,%d @@", oldS, oldC, newS, newC)
		if h.Header != "" {
			body.WriteString(" " + h.Header)
		}
		body.WriteByte('\n')
		for _, l := range lines {
			body.WriteString(l + "\n")
		}
	}
	if changes == 0 {
		return "", false, apiErr("BAD_REQUEST", "Choose at least one added or removed line.")
	}
	keep := full || (ps.kind == "deleted" && ps.reverse) || (ps.kind == "new" && !ps.reverse)
	var head strings.Builder
	fmt.Fprintf(&head, "diff --git a/%s b/%s\n", ps.path, ps.path)
	from, to := "a/"+ps.path, "b/"+ps.path
	if keep && ps.kind == "new" {
		fmt.Fprintf(&head, "new file mode %s\n", ps.mode)
		from = "/dev/null"
	}
	if keep && ps.kind == "deleted" {
		fmt.Fprintf(&head, "deleted file mode %s\n", ps.mode)
		to = "/dev/null"
	}
	fmt.Fprintf(&head, "--- %s\n+++ %s\n", from, to)
	return head.String() + body.String(), full, nil
}

// newFileContent is the content of a new file holding only the chosen lines
// of its all-added diff.
func newFileContent(f rawFile, chosen map[[2]int]bool) []byte {
	var b strings.Builder
	for hi, h := range f.hunks {
		for li, l := range h.raw {
			if l[0] != '+' || !chosen[[2]int{hi, li}] {
				continue
			}
			b.WriteString(l[1:])
			if li+1 >= len(h.raw) || h.raw[li+1][0] != '\\' {
				b.WriteByte('\n')
			}
		}
	}
	return []byte(b.String())
}
