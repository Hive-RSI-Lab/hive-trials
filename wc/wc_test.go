package wc

import (
	"reflect"
	"testing"
)

func TestCount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]int
	}{
		{
			name:     "empty input",
			input:    "",
			expected: map[string]int{},
		},
		{
			name:     "single word",
			input:    "hello",
			expected: map[string]int{"hello": 1},
		},
		{
			name:     "multiple words",
			input:    "hello world",
			expected: map[string]int{"hello": 1, "world": 1},
		},
		{
			name:     "case insensitive",
			input:    "Hello HELLO hello",
			expected: map[string]int{"hello": 3},
		},
		{
			name:     "with punctuation",
			input:    "hello, world!",
			expected: map[string]int{"hello": 1, "world": 1},
		},
		{
			name:     "mixed punctuation and case",
			input:    "Hello, WORLD! hello?",
			expected: map[string]int{"hello": 2, "world": 1},
		},
		{
			name:     "numbers and symbols ignored",
			input:    "hello123 world456",
			expected: map[string]int{"hello": 1, "world": 1},
		},
		{
			name:     "only punctuation",
			input:    "!!! ??? ...",
			expected: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Count(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Count(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}