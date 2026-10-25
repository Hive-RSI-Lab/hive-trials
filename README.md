# trials

A scratch project for trying models and harnesses on small real tasks before routing real work to them.

## CSV Column Summer (csvsum)

A tiny Go module that provides a function to sum numeric values from a specific column in a CSV file.

### Usage

```go
import (
    "fmt"
    "os"
    "trials/csvsum"
)

func main() {
    f, _ := os.Open("data.csv")
    defer f.Close()
    sum, err := csvsum.Sum(f, "price")
    if err != nil {
        // handle error
    }
    fmt.Printf("Total: %v\n", sum)
}
```

The `Sum` function reads the CSV, expects a header row, and returns the sum of all numeric values in the named column. Empty cells are treated as zero. It returns an error if the column is missing or contains non‑numeric data.

## Tests

Run `go test ./...` to execute the test suite covering normal operation, missing column, bad number, and empty input.

## Word Counter (wc)

A tiny Go package that counts words in a string, lower‑casing and splitting on non‑letters.

### Usage

```go
import "trials/wc"

counts := wc.Count("Hello, WORLD! hello?")
// counts == map[string]int{"hello": 2, "world": 1}
```

### Running tests

```bash
go test ./wc/...
```
