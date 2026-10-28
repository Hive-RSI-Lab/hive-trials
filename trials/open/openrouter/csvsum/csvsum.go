package csvsum

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

func Sum(r io.Reader, column string) (float64, error) {
	reader := csv.NewReader(r)
	
	// Read header
	header, err := reader.Read()
	if err == io.EOF {
		// Empty input
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	
	// Find column index
	colIndex := -1
	for i, h := range header {
		if h == column {
			colIndex = i
			break
		}
	}
	if colIndex == -1 {
		return 0, &ColumnNotFoundError{Column: column}
	}
	
	var sum float64
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		
		if colIndex >= len(record) {
			// Missing column in this row, treat as zero
			continue
		}
		
		val := strings.TrimSpace(record[colIndex])
		if val == "" {
			// Empty cell counts as zero
			continue
		}
		
		num, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, &ParseError{Column: column, Value: val}
		}
		
		sum += num
	}
	
	return sum, nil
}

type ColumnNotFoundError struct {
	Column string
}

func (e *ColumnNotFoundError) Error() string {
	return "column not found: " + e.Column
}

type ParseError struct {
	Column string
	Value  string
}

func (e *ParseError) Error() string {
	return "non-numeric value in column " + e.Column + ": " + e.Value
}