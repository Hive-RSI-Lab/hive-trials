package csvsum

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
)

// Sum reads CSV data from r, looks for a column named column (case-sensitive),
// and returns the sum of all numeric values in that column.
// Empty cells are treated as zero. An empty input (no header) returns 0 with no error.
// It returns an error if the column does not exist or any value cannot be parsed as a number.
func Sum(r io.Reader, column string) (float64, error) {
	cr := csv.NewReader(r)
	// Read header
	header, err := cr.Read()
	if err != nil {
		if err == io.EOF {
			// Empty file – nothing to sum
			return 0, nil
		}
		return 0, fmt.Errorf("reading header: %w", err)
	}
	// Find column index
	idx := -1
	for i, name := range header {
		if name == column {
			idx = i
			break
		}
	}
	if idx == -1 {
		return 0, fmt.Errorf("column %s not found", column)
	}
	var sum float64
	for {
		record, err := cr.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, fmt.Errorf("reading record: %w", err)
		}
		if idx >= len(record) {
			return 0, fmt.Errorf("missing column %s in a row", column)
		}
		valStr := record[idx]
		if valStr == "" {
			// Treat empty as zero
			continue
		}
		v, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing value %q in column %s: %w", valStr, column, err)
		}
		sum += v
	}
	return sum, nil
}
