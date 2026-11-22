package main

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestVersionCommandPrintsOneLine(t *testing.T) {
	old := version
	version = "v9.9.9"
	defer func() { version = old }()
	var out bytes.Buffer
	if err := run([]string{"version"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	line := out.String()
	if strings.Count(line, "\n") != 1 || !strings.HasPrefix(line, "mooring v9.9.9 ") ||
		!strings.Contains(line, runtime.GOOS+"/"+runtime.GOARCH) || !strings.Contains(line, runtime.Version()) {
		t.Fatalf("version line: %q", line)
	}
}
