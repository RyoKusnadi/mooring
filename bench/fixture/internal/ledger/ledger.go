// Package ledger adds up rows.
package ledger

import "fixture/internal/parse"

// Total adds the quantities of rows.
func Total(rows []parse.Row) int {
	n := 0
	for _, r := range rows {
		n += r.Qty
	}
	return n
}
