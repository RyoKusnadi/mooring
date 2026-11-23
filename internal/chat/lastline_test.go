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

func TestLastLineDropsMarkdown(t *testing.T) {
	cases := map[string]string{
		"I added `sub(a, b)` to `calc.py`.":             "I added sub(a, b) to calc.py.",
		"**Next step:** wait for the tests.":            "Next step: wait for the tests.",
		"Done.\n\n- See [the guide](https://x.test/g).": "See the guide.",
		"## Summary":                      "Summary",
		"Fixed it.\n\n```go\nx := 1\n```": "Fixed it.",
		"Fixed it.\n\n---":                "Fixed it.",
		"| a | b |\n|---|---|":            "| a | b |",
		"a * b and snake_case stay":       "a * b and snake_case stay",
	}
	for in, want := range cases {
		if got := lastLineOf([]Message{{Kind: KindAgent, Text: in}}); got != want {
			t.Errorf("lastLineOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShortenCutsByRunes(t *testing.T) {
	got := shorten(strings.Repeat("é", 10), 3)
	if got != "ééé..." {
		t.Fatalf("shorten = %q", got)
	}
}
