// Command tally reads "name,qty" lines on standard input and prints each name
// with its total, largest first.
package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"

	"fixture/internal/parse"
)

func main() {
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
	for _, name := range names {
		fmt.Printf("%s %d\n", name, totals[name])
	}
}
