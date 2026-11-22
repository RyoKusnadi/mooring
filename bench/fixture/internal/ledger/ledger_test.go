package ledger

import (
	"testing"

	"fixture/internal/parse"
)

func TestTotal(t *testing.T) {
	if got := Total([]parse.Row{{Name: "a", Qty: 2}, {Name: "b", Qty: 3}}); got != 5 {
		t.Fatalf("Total = %d, want 5", got)
	}
}
