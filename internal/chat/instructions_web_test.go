package chat

import (
	"os"
	"strings"
	"testing"
)

// Settings shows the fixed text in "What is sent"; it must be the text agents get.
func TestInstructionsTextMatchesSettingsPage(t *testing.T) {
	page, err := os.ReadFile("../../web/src/SettingsInstructions.svelte")
	if err != nil {
		t.Fatalf("read the settings page: %v", err)
	}
	if !strings.Contains(string(page), "'"+instructionsText+"'") {
		t.Fatal("web/src/SettingsInstructions.svelte must contain instructionsText exactly, as one single-quoted string")
	}
}
