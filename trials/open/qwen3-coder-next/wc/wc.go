// Package wc counts words in a string, lower-casing and splitting on non-letters.
package wc

import (
	"strings"
	"unicode"
)

// Count takes a string and returns a map of lower-cased words to their
// occurrence count. Words are sequences of Unicode letters; any non-letter
// character acts as a separator. The result uses lower case.
func Count(s string) map[string]int {
	result := make(map[string]int)
	var current strings.Builder

	for _, r := range s {
		if unicode.IsLetter(r) {
			current.WriteRune(unicode.ToLower(r))
		} else {
			if current.Len() > 0 {
				word := current.String()
				result[word]++
				current.Reset()
			}
		}
	}
	// Flush any remaining word
	if current.Len() > 0 {
		word := current.String()
		result[word]++
	}

	return result
}