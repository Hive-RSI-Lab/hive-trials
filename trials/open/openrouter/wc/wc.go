package wc

import (
	"unicode"
)

// Count counts words in a string, lower-casing and splitting on non-letters (Unicode-aware).
func Count(s string) map[string]int {
	result := make(map[string]int)
	var word []rune

	for _, r := range s {
		if unicode.IsLetter(r) {
			word = append(word, unicode.ToLower(r))
		} else {
			if len(word) > 0 {
				result[string(word)]++
				word = word[:0]
			}
		}
	}

	if len(word) > 0 {
		result[string(word)]++
	}

	return result
}