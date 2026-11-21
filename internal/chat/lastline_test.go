package chat

import (
	"strings"
	"testing"
)

func TestLastLineOf(t *testing.T) {
	msgs := []Message{
		{Kind: KindUser, Text: "fix it"},
		{Kind: KindAgent, Text: "First line.\n\nDone, token=abcd1234 is set.\n"},
		{Kind: KindNotice, Text: "You stopped this reply."},
	}
	got := lastLineOf(msgs)
	if strings.Contains(got, "abcd1234") || !strings.HasPrefix(got, "Done, token=") {
		t.Fatalf("last line = %q", got)
	}
	long := lastLineOf([]Message{{Kind: KindAgent, Text: strings.Repeat("é ", 200)}})
	if n := len([]rune(long)); n > lastLineRunes+3 {
		t.Fatalf("not capped by runes: %d", n)
	}
	if lastLineOf(nil) != "" {
		t.Fatal("empty chat has no last line")
	}
}

func TestShortenCutsByRunes(t *testing.T) {
	got := shorten(strings.Repeat("é", 10), 3)
	if got != "ééé..." {
		t.Fatalf("shorten = %q", got)
	}
}
