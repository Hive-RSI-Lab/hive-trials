// Package wc counts words in a string.
package wc

import "unicode"

// Count returns the number of each word in s.
// Words are lower-cased runs of Unicode letters.
// Non-letters separate words and are otherwise ignored.
func Count(s string) map[string]int {
	counts := map[string]int{}
	word := []rune{}
	for _, r := range s {
		if unicode.IsLetter(r) {
			word = append(word, unicode.ToLower(r))
		} else {
			if len(word) > 0 {
				counts[string(word)]++
				word = word[:0]
			}
		}
	}
	if len(word) > 0 {
		counts[string(word)]++
	}
	return counts
}
