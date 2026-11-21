package fakemcp

import (
	"os"
	"testing"
)

// The test binary doubles as the stdio tool server when MOORING_FAKE_MCP=1, and as
// the HTTP one (for browser tests) when FAKE_MCP_HTTP_ADDR is set.
func TestMain(m *testing.M) {
	if os.Getenv("MOORING_FAKE_MCP") == "1" {
		var err error
		if addr := os.Getenv("FAKE_MCP_HTTP_ADDR"); addr != "" {
			err = ListenAndServe(addr)
		} else {
			err = RunStdio()
		}
		if err != nil {
			_, _ = os.Stderr.WriteString("fakemcp: " + err.Error() + "\n")
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
