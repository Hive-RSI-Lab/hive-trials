package wc

import (
	"regexp"
	"strings"
)

func Count(s string) map[string]int {
	// Lowercase the string
	s = strings.ToLower(s)
	
	// Split on non-letters
	re := regexp.MustCompile(`[^a-z]+`)
	words := re.Split(s, -1)
	
	// Count words
	counts := make(map[string]int)
	for _, word := range words {
		if word != "" {
			counts[word]++
		}
	}
	
	return counts
}