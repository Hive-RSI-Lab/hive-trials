# trials

A scratch project for trying models and harnesses on small real tasks before routing real work to them.

## Structure

- **cmd/score/**, **ledger.json** - The score and the ledger snapshot it reads (see Score)

- **tasks/** - Fixed task set: each task's acceptance tests, and no answer
  - No reference implementation is kept here, so a trial can't copy one: the
    tests are the spec, behind the `acceptance` build tag so main's
    `go test ./...` doesn't build them without code
  - `wc/` - Word Counter: counts words in a string (lower-case, split on non-letters)
  - `csvsum/` - CSV Column Summer: sums numeric values from a specific CSV column
  - `lru/` - LRU Cache with TTL: thread-safe LRU cache with per-entry TTL support

- **trials/<harness>/<model>/<task>/** - Each trial runs in its own isolated folder so two trials never touch the same files
  - Example: `trials/open/deepseek-v4-pro/wc/`, `trials/codex/gpt-5/csvsum/`
  - A trial writes only under its own folder
  - A trial's verdict is whether this code passes the task's acceptance tests (see Score)

- **Main project tests** - Main runs `go test ./...` (the test command in `.hive/project.json`): the score's tests, and it builds every trial folder; the acceptance tests run only through the score

## Tasks

### Word Counter (wc)
A tiny Go package that counts words in a string, lower-casing and splitting on non-letters.

```go
// in trials/<harness>/<model>/wc/, package wc

counts := wc.Count("Hello, WORLD! hello?")
// counts == map[string]int{"hello": 2, "world": 1}
```

Its acceptance tests: `tasks/wc/`

### CSV Column Summer (csvsum)
A tiny Go module that provides a function to sum numeric values from a specific column in a CSV file.

```go
import (
    "fmt"
    "os"
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

Its acceptance tests: `tasks/csvsum/`

### LRU Cache with TTL (lru)
A thread-safe LRU cache with TTL support.

```go
// in trials/<harness>/<model>/lru/, package lru

cache := lru.New(3, 100*time.Millisecond)
cache.Set("key", "value")
val, ok := cache.Get("key")
```

Its acceptance tests: `tasks/lru/`

## Score

`go run ./cmd/score` prints one number, higher is better: the share of
routable cells that have a verdict. A cell is one pick (harness and model)
on one task:

- **picks**: every named model in `hive agent models` (its `models`) on the
  `open` harness, the only one that runs them; then `codex:gpt-5`,
  `cursor:auto` and `opencode:opencode/big-pickle` (`Native` in
  `cmd/score/main.go`), each harness on a model of its own
- **tasks**: every folder under `tasks/`

A cell counts only when a ledger trial on its pick and task landed its code
(its outcome is `merged`). Its verdict is then whether the code in
`trials/<harness>/<model>/<task>/` passes the acceptance tests in
`tasks/<task>/`: the score copies the folder's non-test `.go` files beside
those tests and runs `go test`. Pass or fail, it is a verdict. A trial that
ended on a Hive fault (a 403 from the forge, closed by the lead, left open by
the warden) landed nothing and gives none; nor does code in a cell's folder
that no trial of that cell landed (written by hand, or by a trial on another
pick or task), which `-v` flags.

The score check runs sandboxed and can't call `hive agent models`, so the
score reads `ledger.json`, a snapshot of it, matching a trial to its cell by its model (a named model is
`open`, `harness:model` that harness) and its title (the task's `TASKS.md`
name, say "Word Counter", or its folder in parentheses, say "(wc)", or a title
that starts with "<dir> on ", say "wc on open/deepseek-v4-pro"). After a
trial ends, refresh and commit it:

```bash
hive agent models | go run ./cmd/score -save   # writes ledger.json, prints the score
go run ./cmd/score -v                           # every cell: pass, fail or none, unlanded code, and its trials' outcome, reason, minutes, turns, cost
```

A trial's PR adds only its `trials/<harness>/<model>/<task>/` code and never
edits `ledger.json`, which is refreshed from the record after the trial ends.

## Running All Tests

```bash
go test ./...                                 # what main runs
go vet ./... && go test -race -count=1 ./...  # stricter
```

To run a task's acceptance tests on a trial by hand, copy them beside its code
and set the tag, as the score does:

```bash
cp tasks/lru/*_test.go /tmp/t/ && cp trials/open/<model>/lru/*.go /tmp/t/   # then, in /tmp/t with a go.mod:
go test -race -tags acceptance .
```