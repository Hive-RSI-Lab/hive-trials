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
  - `gocache/` - Build Cache Fix: a bug fix across three files of a copy of the Hive's
    `internal/gocache`, whose buggy starting code is in `gocache/_start/` (go ignores it)
  - `config/` - Config Includes: an `include` directive added across three files of a
    small config reader in `config/_start/`, judged by hidden acceptance tests
    (`config_test.go.gz`, which the score unzips) on edges the spec leaves to the reader
  - `routeknob/` - Route Knob Takeover: a one-function bug fix in a copy of the deck's
    soft takeover (`routeknob/_start/`), from real deck work, judged by hidden tests

- **trials/<harness>/<model>/<task>/** - Each trial runs in its own isolated folder so two trials never touch the same files
  - Example: `trials/open/deepseek-v4-pro/wc/`, `trials/codex/gpt-5/csvsum/`
  - A trial writes only under its own folder
  - A trial's verdict is whether this code passes the task's acceptance tests (see Score)
  - A trial's code is sealed on main: `go run ./cmd/score -seal` zips each `.go` file of a
    plain trial folder into a `.go.gz`, so a later trial of the same task can't read or grep
    this one's answer in its checkout. A trial lands plain code; `.github/workflows/seal.yml`
    seals it on main right after the merge, and `go test ./...` fails on a folder left plain
    once the ledger shows its trial merged

- **Main project tests** - Main runs `go test ./...` (the test command in `.hive/project.json`): the score's tests, which fail on a landed trial's code left unsealed; the acceptance tests run only through the score

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

### Build Cache Fix (gocache)
A bug fix in existing code, from real Hive work: the trial copies
`tasks/gocache/_start/*.go` (a copy of the Hive's GOCACHEPROG, split into
three files, with four bugs) into its folder and fixes them from the bug
reports in `TASKS.md`.

```go
// in trials/<harness>/<model>/gocache/, package gocache

c := gocache.Cache{Shared: "/cache/main", Own: "/cache/this-check"}
err := c.Serve(os.Stdin, os.Stdout)
```

Its acceptance tests: `tasks/gocache/`

### Config Includes (config)
A feature in existing code: the trial copies `tasks/config/_start/*.go` (an
INI-like config reader in three files) into its folder and adds an `include`
directive to the spec in `TASKS.md`. Its acceptance tests are hidden: they
are gzipped, so the trial is judged on how it reads the spec, edges
included, as real work is.

```go
// in trials/<harness>/<model>/config/, package config

c, err := config.Load("/etc/app/main.conf") // main.conf: include db.conf
host, ok := c.Get("db.host")
```

Its acceptance tests: `tasks/config/config_test.go.gz`

### Route Knob Takeover (routeknob)
A bug fix in one function, from the deck: the trial copies
`tasks/routeknob/_start/knob.go` (a route knob's soft takeover) into its
folder and fixes it so a knob acts at once when the Hive's route isn't one
of its stops, keeping the rest as it is. Its acceptance tests are hidden.

```go
// in trials/<harness>/<model>/routeknob/, package routeknob

var k routeknob.Knob
act, prompt := k.Move(3, -1) // the Hive's route is off the knob: act == true
```

Its acceptance tests: `tasks/routeknob/routeknob_test.go.gz`

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
`tasks/<task>/`: the score copies the folder's non-test code beside those
tests (unzipping its sealed `.go.gz` files and a hidden `_test.go.gz`) and runs `go test`. Pass or fail, it is a verdict. A trial that
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

Each new verdict (pass or fail) then goes to the knowledge base as a routing
note, so routing can find how a pick did without reading this repo: one note
per cell, titled "Routing: <pick> passes|fails <task> (trials verdict)",
tagged `trials,routing,verdict` and the model, with the trial's task id,
commit, turns, minutes, cost and what the verdict says about routing work to
that pick:

```bash
hive agent remember --title "Routing: open:<model> passes wc (trials verdict)" \
  --tags trials,routing,verdict,<model> --text "..."
```

## Running All Tests

```bash
go test ./...                                 # what main runs
go vet ./... && go test -race -count=1 ./...  # stricter
```

To run a task's acceptance tests on a trial by hand, copy them beside its code
and set the tag, as the score does:

```bash
cp tasks/lru/*_test.go /tmp/t/ && for f in trials/open/<model>/lru/*.go.gz; do gunzip -c $f > /tmp/t/$(basename $f .gz); done   # then, in /tmp/t with a go.mod:
go test -race -tags acceptance .
```