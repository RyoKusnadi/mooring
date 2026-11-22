package files

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path"
	"sort"
	"strings"
)

type rgMsg struct {
	Type string `json:"type"`
	Data struct {
		Path       struct{ Text string }      `json:"path"`
		Lines      struct{ Text string }      `json:"lines"`
		LineNumber int                        `json:"line_number"`
		Submatches []struct{ Start, End int } `json:"submatches"`
	} `json:"data"`
}

// runRG searches with ripgrep, which reads .gitignore on its own.
func (r *searchRun) runRG() error {
	args := []string{"--json", "--no-config", "--max-count", "50", "--max-filesize", "2M", "--no-messages", "--no-require-git"}
	if !r.req.Regex {
		args = append(args, "--fixed-strings")
	}
	if r.req.Case {
		args = append(args, "--case-sensitive")
	} else {
		args = append(args, "--ignore-case")
	}
	names := make([]string, 0, len(alwaysIgnored))
	for n := range alwaysIgnored {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		args = append(args, "--glob", "!"+n+"/")
	}
	if g := strings.TrimSpace(r.req.Glob); g != "" {
		args = append(args, "--glob", g)
	}
	target := relToRoot(r.root, r.dir)
	if target == "" {
		target = "."
	}
	args = append(args, "--regexp", r.req.Query, "--", target)
	cmd := exec.CommandContext(r.ctx, r.s.rg, args...)
	cmd.Dir = r.root
	var stderr bytes.Buffer
	cmd.Stderr = &limitBuf{b: &stderr, max: 4096}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		r.s.log.Warn("files: start rg", "err", err)
		return r.fallback()
	}
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		var m rgMsg
		if json.Unmarshal(sc.Bytes(), &m) != nil || m.Type != "match" || m.Data.Path.Text == "" {
			continue
		}
		rel := path.Clean(strings.TrimPrefix(m.Data.Path.Text, "./"))
		ranges := make([][2]int, 0, len(m.Data.Submatches))
		for _, sm := range m.Data.Submatches {
			ranges = append(ranges, [2]int{sm.Start, sm.End})
		}
		if !r.add(rel, m.Data.LineNumber, m.Data.Lines.Text, ranges) {
			break
		}
	}
	stopped := r.full || r.emitErr != nil || r.ctx.Err() != nil
	scanErr := sc.Err()
	if stopped || scanErr != nil {
		// rg would otherwise scan on until it ends or times out.
		_ = cmd.Process.Kill()
	}
	_, _ = io.Copy(io.Discard, out)
	werr := cmd.Wait()
	if stopped {
		return nil
	}
	if scanErr != nil {
		r.s.log.Warn("files: read rg output", "err", scanErr)
		return nil
	}
	var ee *exec.ExitError
	if errors.As(werr, &ee) && ee.ExitCode() == 2 && r.count == 0 && stderr.Len() > 0 {
		return &searchErr{"BAD_QUERY", firstLine(stderr.String())}
	}
	return nil
}

// fallback runs the Go walk when rg cannot start.
func (r *searchRun) fallback() error { return r.runWalk() }

type limitBuf struct {
	b   *bytes.Buffer
	max int
}

func (l *limitBuf) Write(p []byte) (int, error) {
	if room := l.max - l.b.Len(); room > 0 {
		l.b.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}
