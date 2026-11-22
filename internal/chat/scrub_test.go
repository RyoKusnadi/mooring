package chat

import (
	"strings"
	"testing"
)

func TestCleanLine(t *testing.T) {
	for _, tc := range []struct{ name, in, want, not string }{
		{"ansi", "\x1b[31mfailed\x1b[0m to \x1b]0;title\x07start\x00\r", "failed to start", "\x1b"},
		{"api key", "ANTHROPIC_API_KEY=sk-ant-api03-abcdefghijklmnop", "ANTHROPIC_API_KEY=[hidden]", "sk-ant"},
		{"bearer", "Authorization: Bearer abc123", "Authorization: [hidden]", "abc123"},
		{"long run", "bad value ghp_0123456789abcdefghijklmnop here", "bad value [hidden] here", "ghp_"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanLine(tc.in, 160)
			if got != tc.want || strings.Contains(got, tc.not) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	got := cleanLine(strings.Repeat("é ", 150), 160)
	if r := []rune(got); len(r) != 163 || !strings.HasSuffix(got, "...") {
		t.Fatalf("300-rune line gave %d runes: %q", len(r), got)
	}
}
