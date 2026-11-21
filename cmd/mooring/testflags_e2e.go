//go:build e2e

package main

import (
	"flag"
	"fmt"

	"github.com/ryokusnadi/mooring/internal/accounts"
	"github.com/ryokusnadi/mooring/internal/chat"
)

// addTestFlags registers the flags that exist only in browser-test builds (go build -tags e2e).
func addTestFlags(flags *flag.FlagSet) *string {
	return flags.String("fake-account-agent", "", "test only: give this agent id the account folder FAKE_CONFIG, for the stand-in agent")
}

// withFakeAccount gives the catalog agent with id fake an account adapter on FAKE_CONFIG.
func withFakeAccount(defs []chat.AgentDef, fake string) ([]chat.AgentDef, error) {
	if fake == "" {
		return defs, nil
	}
	for i := range defs {
		if defs[i].ID == fake {
			defs[i].Account = &accounts.Adapter{Env: "FAKE_CONFIG", Login: "touch signed-in", Usage: "claude", Home: ".fake-agent"}
			return defs, nil
		}
	}
	return nil, fmt.Errorf("-fake-account-agent: no agent %q", fake)
}
