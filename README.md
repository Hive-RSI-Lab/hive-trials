# trials

A scratch project for trying models and harnesses on small real tasks before routing real work to them.

## Structure

- **tasks/** - Fixed task set with reference implementations and acceptance tests
  - `wc/` - Word Counter: counts words in a string (lower-case, split on non-letters)
  - `csvsum/` - CSV Column Summer: sums numeric values from a specific CSV column
  - `lru/` - LRU Cache with TTL: thread-safe LRU cache with per-entry TTL support

- **trials/<model>/<task>/** - Each trial runs in its own isolated folder so two trials never touch the same files
  - Example: `trials/deepseek/wc/`, `trials/claude/csvsum/`, etc.
  - A trial writes only under its own folder
  - A trial's result is its ledger record in `hive agent models`, not the code it produces

- **Main project tests** - Main runs `go test ./...` (the test command in `.hive/project.json`) over `tasks/` and any trial folders

## Tasks

### Word Counter (wc)
A tiny Go package that counts words in a string, lower-casing and splitting on non-letters.

```go
import "trials/tasks/wc"

counts := wc.Count("Hello, WORLD! hello?")
// counts == map[string]int{"hello": 2, "world": 1}
```

Run tests: `go test ./tasks/wc/...`

### CSV Column Summer (csvsum)
A tiny Go module that provides a function to sum numeric values from a specific column in a CSV file.

```go
import (
    "fmt"
    "os"
    "trials/tasks/csvsum"
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

Run tests: `go test ./tasks/csvsum/...`

### LRU Cache with TTL (lru)
A thread-safe LRU cache with TTL support.

```go
import "trials/tasks/lru"

cache := lru.New(3, 100*time.Millisecond)
cache.Set("key", "value")
val, ok := cache.Get("key")
```

Run tests: `go test ./tasks/lru/...`

## Running All Tests

```bash
go test ./...                                 # what main runs
go vet ./... && go test -race -count=1 ./...  # stricter, for the LRU concurrency tests
```