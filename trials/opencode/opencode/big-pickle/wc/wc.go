package wc

import (
	"strings"
	"unicode"
)

// Count counts words in a string, lower-casing and splitting on non-letters (Unicode-aware).
func Count(s string) map[string]int {
	result := map[string]int{}
	if s == "" {
		return result
	}

	var word strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) {
			word.WriteRune(unicode.ToLower(r))
		} else {
			if word.Len() > 0 {
				result[word.String()]++
				word.Reset()
			}
		}
	}
	if word.Len() > 0 {
		result[word.String()]++
	}
	return result
}