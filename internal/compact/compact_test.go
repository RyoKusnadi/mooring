package compact

import (
	"fmt"
	"strings"
	"testing"
)

func numbered(n int, prefix string) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%s%d\n", prefix, i)
	}
	return b.String()
}

func TestShortOutputUnchanged(t *testing.T) {
	in := numbered(200, "line ")
	out, res := Apply(in)
	if out != in || res.Applied || res.Lines != 200 || res.Kept != 200 {
		t.Fatalf("short output changed: applied=%v lines=%d", res.Applied, res.Lines)
	}
}

func TestLongBytesTriggers(t *testing.T) {
	in := strings.Repeat("x", 100) + "\n"
	in = strings.Repeat(in, 180) // 18 KB, 180 lines
	if _, res := Apply(in); !res.Applied {
		t.Fatal("18 KB was not compacted")
	}
}

func TestStripsAnsiAndCarriageReturns(t *testing.T) {
	var b strings.Builder
	b.WriteString("\x1b[31mred\x1b[0m title\x1b]0;window\x07\n")
	b.WriteString("10%\r50%\r100%\n")
	b.WriteString("windows\r\n")
	b.WriteString(numbered(250, "n"))
	out, _ := Apply(b.String())
	lines := strings.Split(out, "\n")
	if lines[0] != "red title" || lines[1] != "100%" || lines[2] != "windows" {
		t.Fatalf("first lines: %q", lines[:3])
	}
}

func TestRepeatedLines(t *testing.T) {
	in := strings.Repeat("same\n", 300) + "two\ntwo\nend\n"
	out, _ := Apply(in)
	if !strings.HasPrefix(out, "same\n[repeated 300 times]\ntwo\ntwo\nend\n") {
		t.Fatalf("out: %q", out[:60])
	}
	two := collapse([]string{"a", "a", "b"})
	if strings.Join(two, ",") != "a,a,b" {
		t.Fatalf("two in a row collapsed: %v", two)
	}
}

func TestHeadTailAndErrorContext(t *testing.T) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = fmt.Sprintf("ok %d", i)
	}
	lines[500] = "FAIL: TestThing"
	out, res := Apply(strings.Join(lines, "\n") + "\n")
	for _, want := range []string{"ok 0\n", "ok 39\n", "ok 499\n", "FAIL: TestThing", "ok 501\n", "ok 920\n", "ok 999"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q", want)
		}
	}
	for _, gone := range []string{"ok 40\n", "ok 498\n", "ok 502\n", "ok 919\n"} {
		if strings.Contains(out, gone) {
			t.Fatalf("kept %q", gone)
		}
	}
	if res.Kept != 40+3+80 {
		t.Fatalf("kept %d lines", res.Kept)
	}
	if !strings.HasSuffix(out, "[mooring: 877 of 1000 lines cut; ask the user for the full output]") {
		t.Fatalf("closing note: %q", out[len(out)-90:])
	}
}

func TestErrorLinesCapAndPatterns(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&b, "filler %d\n", i)
		if i%2 == 0 {
			b.WriteString("error again\n")
		}
	}
	out, _ := Apply(b.String())
	if n := strings.Count(out, "error again"); n > 60+40+80 {
		t.Fatalf("%d error lines kept", n)
	}
	for _, l := range []string{"panic: x", "Traceback (most recent call last)", "  at Foo.bar", "main.go:12:3 oops", "FATAL", "unhandled Exception"} {
		if !errLine.MatchString(l) {
			t.Errorf("%q not an error line", l)
		}
	}
	if errLine.MatchString("all good") {
		t.Error("plain line matched")
	}
}

func TestLongLinesAndResultCut(t *testing.T) {
	in := strings.Repeat("é", 500) + "\n" + numbered(300, "z")
	out, _ := Apply(in)
	first := strings.SplitN(out, "\n", 2)[0]
	if len(first) > MaxLineLen || strings.ContainsRune(first, '�') {
		t.Fatalf("first line %d bytes", len(first))
	}
	wide := strings.Repeat(strings.Repeat("w", 399)+"\n", 150)
	out, _ = Apply(wide)
	if len(out) > MaxResult+120 {
		t.Fatalf("result %d bytes", len(out))
	}
}

func TestCountLines(t *testing.T) {
	for in, want := range map[string]int{"": 0, "a": 1, "a\n": 1, "a\nb": 2, "\n": 1} {
		if got := CountLines(in); got != want {
			t.Errorf("CountLines(%q) = %d, want %d", in, got, want)
		}
	}
}
