//go:build !e2e

package main

import (
	"flag"

	"github.com/ryokusnadi/mooring/internal/chat"
)

// addTestFlags registers nothing in release builds; the browser-test build (-tags e2e) adds test-only flags.
func addTestFlags(*flag.FlagSet) *string { return new(string) }

func withFakeAccount(defs []chat.AgentDef, _ string) ([]chat.AgentDef, error) { return defs, nil }
