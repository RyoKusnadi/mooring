// Package parse reads the "name, quantity" rows the tally tool counts.
package parse

import (
	"fmt"
	"strconv"
	"strings"
)

// Row is one line of input.
type Row struct {
	Name string
	Qty  int
}

// ParseRow reads "name,qty". Spaces around the quantity are ignored.
func ParseRow(line string) (Row, error) {
	fields := strings.Split(line, ",")
	if len(fields) != 2 {
		return Row{}, fmt.Errorf("want name,qty, got %q", line)
	}
	qty, err := strconv.Atoi(strings.TrimSpace(fields[1]))
	if err != nil {
		return Row{}, fmt.Errorf("quantity in %q: %w", line, err)
	}
	return Row{Name: fields[0], Qty: qty}, nil
}
