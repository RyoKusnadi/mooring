package fakessh

import (
	"os"
	"testing"
)

// The test binary doubles as the stand-in server when MOORING_FAKE_SSH=1.
func TestMain(m *testing.M) {
	if os.Getenv("MOORING_FAKE_SSH") == "1" {
		if err := Run(); err != nil {
			_, _ = os.Stderr.WriteString("fakessh: " + err.Error() + "\n")
			os.Exit(1)
		}
		return
	}
	os.Exit(m.Run())
}
