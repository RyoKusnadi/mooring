package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// version is set at build time: -ldflags "-X main.version=v0.13.0".
var version = "dev"

// versionLine is "mooring v0.13.0 (commit 0fe6833, 2026-11-20) darwin/arm64 go1.27.4".
func versionLine() string {
	var commit, when string
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				commit = s.Value
			case "vcs.time":
				when = s.Value
			}
		}
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	if len(when) >= 10 {
		when = when[:10]
	}
	detail := ""
	switch {
	case commit != "" && when != "":
		detail = fmt.Sprintf(" (commit %s, %s)", commit, when)
	case commit != "":
		detail = fmt.Sprintf(" (commit %s)", commit)
	}
	return fmt.Sprintf("mooring %s%s %s/%s %s", version, detail, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
