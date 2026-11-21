#!/bin/sh
# Canned fix for the stand-in agent: add -limit to cmd/tally.
cd "$1" || exit 1
cat > cmd/tally/main.go <<'GO'
// Command tally reads "name,qty" lines on standard input and prints each name
// with its total, largest first. -limit N prints only the first N.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"

	"fixture/internal/parse"
)

func main() {
	limit := flag.Int("limit", 0, "print only the first N names (0 prints all)")
	flag.Parse()
	totals := map[string]int{}
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		row, err := parse.ParseRow(sc.Text())
		if err != nil {
			fmt.Fprintln(os.Stderr, "tally:", err)
			os.Exit(1)
		}
		totals[row.Name] += row.Qty
	}
	names := make([]string, 0, len(totals))
	for name := range totals {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if totals[names[i]] != totals[names[j]] {
			return totals[names[i]] > totals[names[j]]
		}
		return names[i] < names[j]
	})
	if *limit > 0 && len(names) > *limit {
		names = names[:*limit]
	}
	for _, name := range names {
		fmt.Printf("%s %d\n", name, totals[name])
	}
}
GO
