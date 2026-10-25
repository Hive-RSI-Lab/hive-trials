package wc

import (
	"strings"
	"unicode"
)

// Count returns a map of word frequencies in s.
// Words are lower-cased and split on non-letters (Unicode-aware).
func Count(s string) map[string]int {
	// Split on non-letters using Unicode-aware function
	words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r)
	})

	// Count words
	counts := make(map[string]int)
	for _, word := range words {
		counts[word]++
	}

	return counts
}