// Package csvsum provides a function to sum numeric values from a specific
// column in a CSV file.
package csvsum

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Sum reads CSV data from r (with a header row) and returns the sum of all
// numeric values in the named column. Empty cells are treated as zero.
// Empty input (no header) returns 0 with no error. It returns an error if
// the column is missing or contains non-numeric data that cannot be parsed.
func Sum(r io.Reader, column string) (float64, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return 0, err
	}

	// Empty input: no header row.
	if len(records) == 0 {
		return 0, nil
	}

	// Header row.
	header := records[0]

	// Find the column index by matching the header name (case-sensitive as
	// the field name). Trim spaces for robustness.
	colIdx := -1
	for i, name := range header {
		if strings.TrimSpace(name) == column {
			colIdx = i
			break
		}
	}
	if colIdx == -1 {
		return 0, fmt.Errorf("column %q not found in header", column)
	}

	var sum float64
	for _, row := range records[1:] {
		if colIdx >= len(row) {
			return 0, fmt.Errorf("row has no column %q", column)
		}
		cell := strings.TrimSpace(row[colIdx])
		if cell == "" {
			// Empty cell counts as zero.
			continue
		}
		val, err := strconv.ParseFloat(cell, 64)
		if err != nil {
			return 0, fmt.Errorf("non-numeric value %q in column %q: %w", cell, column, err)
		}
		sum += val
	}

	return sum, nil
}
