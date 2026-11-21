package parse

import (
	"fmt"
	"testing"
)

// 300 cases, each logging what it does. Case 137 has spaces around the name.
func TestParseRow(t *testing.T) {
	for i := 0; i < 300; i++ {
		line := fmt.Sprintf("item%03d,%d", i, i%7+1)
		if i == 137 {
			line = fmt.Sprintf("  item%03d , %d", i, i%7+1)
		}
		t.Run(fmt.Sprintf("case%03d", i), func(t *testing.T) {
			t.Logf("parsing %q", line)
			row, err := ParseRow(line)
			if err != nil {
				t.Fatalf("ParseRow(%q): %v", line, err)
			}
			if want := fmt.Sprintf("item%03d", i); row.Name != want {
				t.Errorf("name = %q, want %q", row.Name, want)
			}
			if row.Qty != i%7+1 {
				t.Errorf("qty = %d, want %d", row.Qty, i%7+1)
			}
			t.Logf("got %+v", row)
		})
	}
}

func TestParseRowRejectsBadInput(t *testing.T) {
	for _, line := range []string{"", "a", "a,b", "a,1,2"} {
		if _, err := ParseRow(line); err == nil {
			t.Errorf("ParseRow(%q) should fail", line)
		}
	}
}
