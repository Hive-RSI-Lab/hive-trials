// Package wc counts words made of Unicode letters.
package wc

import (
	"strings"
	"unicode"
)

// Count returns lowercase word frequencies, splitting at every non-letter.
func Count(s string) map[string]int {
	counts := make(map[string]int)
	for _, word := range strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r)
	}) {
		counts[strings.ToLower(word)]++
	}
	return counts
}
