package internal

import "sort"

// sortedStrings returns a sorted copy, so a data source built from a map
// produces the same output on two identical plans.
func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
