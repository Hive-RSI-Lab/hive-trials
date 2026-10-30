# Fixed Task Set

This file defines the six fixed small tasks used for trials. Each task's acceptance tests are under `tasks/<task>/`, hidden: gzipped (`*_test.go.gz`), which the score unzips. Don't open them: a trial is judged on how it reads its task's spec here (and its starting code), edges included, as real work is, so it can fail. No reference implementation is kept in the repo, so a trial can't copy the answer. The first three are written from scratch; the fourth and sixth are bug fixes and the fifth a feature in existing code, which the task gives in `tasks/<task>/_start/` (a folder go ignores, so main never builds it). The tests carry the `acceptance` build tag and run only through `go run ./cmd/score`, against a trial's code.

## 1. Word Counter (wc)

**Location:** `tasks/wc/`

**Description:** A tiny Go package that counts words in a string, lower-casing and splitting on non-letters (Unicode-aware).

**API:**
```go
func Count(s string) map[string]int
```

**Behavior:**
- A word is a run of letters (`unicode.IsLetter`); anything else (space,
  punctuation, digits, symbols) ends it
- Words are counted lower-cased (Hello HELLO hello → hello: 3)
- Input with no letters returns an empty map

**Acceptance Tests:** hidden, in `tasks/wc/wc_test.go.gz`. They test empty
input, case, punctuation, digits, whitespace and Unicode.

**Run:** `go run ./cmd/score -v` (judges every trial of it)

---

## 2. CSV Column Summer (csvsum)

**Location:** `tasks/csvsum/`

**Description:** A tiny Go module that provides a function to sum numeric values from a specific column in a CSV file.

**API:**
```go
func Sum(r io.Reader, column string) (float64, error)
```

**Behavior:**
- Reads CSV with header row
- Returns sum of all numeric values in the named column
- Empty cells treated as zero
- Empty input (no header) returns 0 with no error
- Returns error if column is missing or contains non-numeric data

**Acceptance Tests:** hidden, in `tasks/csvsum/sum_test.go.gz`. They test
sums, missing columns, bad numbers, empty input and cells, and CSV as CSV is
written.

**Run:** `go run ./cmd/score -v` (judges every trial of it)

---

## 3. LRU Cache with TTL (lru)

**Location:** `tasks/lru/`

**Description:** A thread-safe LRU cache with TTL support. Uses a doubly-linked list for O(1) LRU ordering and a map for O(1) lookup.

**API:**
```go
type Cache struct { ... }
func New(capacity int, ttl time.Duration) *Cache
func (c *Cache) Get(key string) (any, bool)
func (c *Cache) Set(key string, value any)
func (c *Cache) Len() int

// The acceptance tests swap the clock to test TTLs without sleeping:
type Clock interface { Now() time.Time }
func (c *Cache) setClockForTest(clock Clock)
```

**Behavior:**
- `capacity <= 0` means no capacity limit
- `ttl <= 0` means entries do not expire
- Thread-safe with RWMutex
- Expired entries removed on Get or on Set when at capacity
- Update (Set on existing key) resets TTL and moves to front (most recent)
- Get moves entry to front (most recent); it does not reset TTL

**Acceptance Tests:** hidden, in `tasks/lru/lru_test.go.gz`. They test LRU
order, TTL expiry, updates, capacity limits and concurrent use (under
`go test -race`), through the API above.

**Run:** `go run ./cmd/score -v` (judges every trial of it)

---

## 4. Build Cache Fix (gocache)

**Location:** `tasks/gocache/`, starting code in `tasks/gocache/_start/`

**Description:** A bug fix across files in an existing Go package, from real
Hive work: a copy of the Hive's `internal/gocache`, the GOCACHEPROG
(https://pkg.go.dev/cmd/go/internal/cacheprog) a check's `go` command uses to
reuse what main's checks built. It answers the go command's JSON requests
(`get`, `put`, `close`) from two folders laid out as go lays out GOCACHE: a
read-only one shared with main's checks, and the check's own. The copy is
split into `serve.go` (the protocol), `store.go` (reading and writing
entries) and `layout.go` (where an entry lives and what it says), and has
bugs. Copy the three files into your trial folder, keep the package and its
API, find the causes and fix them.

**API (as in `_start/`):**
```go
type Cache struct{ Shared, Own string }
type Entry struct { Output []byte; Size int64; At time.Time; Path string }
func (c Cache) Serve(in io.Reader, out io.Writer) error
func (c Cache) Get(action []byte) (Entry, bool)
func Put(dir string, action, output, body []byte) (string, error)
```

**Bug reports** (what was seen, not where the cause is):
1. A check hangs, or answers the go command's next request wrongly, after go
   puts an empty output (a `put` with `BodySize` 0, which has no body line).
2. A branch check gets main's output for an action it rebuilt itself, when
   both folders have it: what the check built itself must win.
3. About one action in sixteen always misses in main's shared cache though go
   wrote it there: those whose ID starts with a byte below 0x10.
4. An output file cut short (a full disk, a killed copy) is served, and the
   build fails on a corrupt object instead of rebuilding.

**Acceptance Tests:** hidden, in `tasks/gocache/gocache_test.go.gz`. They
test the protocol, the four bug reports, go's own GOCACHE layout, and
entries and outputs that should miss.

The starting code builds and passes every test but the four bug reports'
(`TestStartsFail` in `cmd/score` checks that it fails), so landing it
unchanged is a fail.

**Run:** `go run ./cmd/score -v` (judges every trial of it)

---

## 5. Config Includes (config)

**Location:** `tasks/config/`, starting code in `tasks/config/_start/`

**Description:** A feature across files in an existing Go package: a small
reader for INI-like config files, split into `config.go` (the API),
`parse.go` (the lines) and `errors.go` (errors that name a file and line).
Copy the three files into your trial folder, keep the package and its API,
and add an `include` directive. Read the starting code first: what it does
now is part of the spec.

**API (as in `_start/`):**
```go
type Config struct { ... }
func Load(path string) (*Config, error)
func Parse(r io.Reader, name string) (*Config, error)
func (c *Config) Get(key string) (string, bool)
func (c *Config) Keys() []string
type Error struct { File string; Line int; Msg string } // Error() is "file:line: msg"; add fields if you need them
```

**The change:**
- A line `include <path>` reads the file at path in its place: its keys are
  set as if they stood there, so a key set before the include is overridden
  by it and one set after overrides it. The path is written as a value is
  (quoted when it has spaces; a ` #` comment may follow).
- A relative path is relative to the folder of the file that names it (for
  `Parse`, the folder of `name`).
- An included file is read as a file of its own: it starts outside any
  section, and after it the including file goes on in its own section.
- A file that includes itself, directly or through other files, is an error,
  not a hang.
- Every error still names the file and line it is at: a bad line in an
  included file is an error at that file and line, and an include that can't
  be read is an error at the include line. A missing include is such an
  error, and `errors.Is(err, fs.ErrNotExist)` holds for it, as it does now
  for a missing file passed to `Load`.
- Every file that loads now loads to the same values.

**Acceptance Tests:** hidden, in `tasks/config/config_test.go.gz`, which the
score unzips. Don't open it: the trial is judged on how it reads the spec
above and the starting code, edges included, as real work would be. It
tests what loads now, includes in place, paths, sections, cycles and errors.
The starting code builds and fails them (`TestStartsFail`).

**Run:** `go run ./cmd/score -v` (judges every trial of it)

---

## 6. Route Knob Takeover (routeknob)

**Location:** `tasks/routeknob/`, starting code in `tasks/routeknob/_start/`

**Description:** A bug fix in one function, from real Hive work on the
deck (the DDJ-FLX4 that drives the Hive): `Knob.Move`, a copy of the soft
takeover in deck's `deck/engine.go`. A route knob has a stop per model; it
changes the Hive's route only once it has caught up with the stop the Hive
holds, so a knob left somewhere else never jumps the setting. Copy
`knob.go` into your trial folder, keep the package and its API, and fix it.
Read the starting code first: what it does now, except for the bug, is part
of the spec.

**API (as in `_start/`):**
```go
type Knob struct { ... } // the zero value is a knob not yet moved
func (k *Knob) Move(pos, cur int) (act bool, prompt string)
```

`pos` is the stop the knob moved to; `cur` is the Hive's stop, or -1 when
the Hive's route is none of the knob's stops (another model, or a list of
several). The caller passes the Hive's route as it is now, so after a move
acts, the next `cur` is usually that `pos`.

**What it does now (keep it):**
- The knob picks up the Hive's stop by reaching it, or by crossing it from
  one move to the next (the first move crosses nothing). Until then a move
  does nothing.
- A knob that needs a pickup says so once, on its first move that doesn't
  pick up, and not again until it has picked up.
- Once picked up, a move to another stop acts; a move onto the Hive's stop
  doesn't. When the Hive's stop changes, the knob has to pick it up again.

**Bug report** (what the person saw, and their call on it):
When the Hive's route isn't one of the knob's stops, the knob never sets a
route: it asks to be moved "deliberately", and no move does anything.
There is no stop to reach or cross, so the knob should act on its first
move with no prompt, as any move after it does while the route is still off
its stops. Doing so is a pickup like any other.

**Acceptance Tests:** hidden, in `tasks/routeknob/routeknob_test.go.gz`,
which the score unzips. Don't open it: the trial is judged on how it reads
the spec above and the starting code. It tests the knob off its stops,
pickups by reaching and crossing, the prompt, and the Hive changing its
route. The starting code builds and fails them (`TestStartsFail`).

**Run:** `go run ./cmd/score -v` (judges every trial of it)

---

## Trial Isolation

Each trial runs in its own folder under `trials/<harness>/<model>/<task>/` so
two trials never touch the same files: `trials/open/gpt-oss-120b/lru/`,
`trials/codex/default/wc/`. Its code is in the package the task's tests name
(`package wc`, `package csvsum`, `package lru`, `package gocache`, `package config`, `package routeknob`), with the API above.

A trial's verdict is whether that code passes the task's acceptance tests:
`go run ./cmd/score` runs `tasks/<task>/*_test.go.gz`, unzipped, against it, so the
trial's own tests, if any, don't decide it. See the README's Score.

A trial's code is sealed on main right after it lands (`go run ./cmd/score
-seal`, run by `.github/workflows/seal.yml`: each `.go` file zipped to a
`.go.gz`), so no trial's answer sits readable in the next one's checkout; a
sealed trial is another trial's answer key, not yours to open.
