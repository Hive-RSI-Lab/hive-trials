# Fixed Task Set

This file defines the four fixed small tasks used for trials. Each task's acceptance tests are under `tasks/<task>/`; no reference implementation is kept in the repo, so a trial can't copy the answer. The first three are written from scratch; the fourth is a bug fix in existing code, which the task gives in `tasks/<task>/_start/` (a folder go ignores, so main never builds it). The tests carry the `acceptance` build tag and run only through `go run ./cmd/score`, against a trial's code.

## 1. Word Counter (wc)

**Location:** `tasks/wc/`

**Description:** A tiny Go package that counts words in a string, lower-casing and splitting on non-letters (Unicode-aware).

**API:**
```go
func Count(s string) map[string]int
```

**Acceptance Tests:** `tasks/wc/wc_test.go`
- Empty input returns empty map
- Single word
- Multiple words
- Case insensitive (Hello HELLO hello → hello: 3)
- With punctuation (hello, world! → hello: 1, world: 1)
- Mixed punctuation and case
- Numbers and symbols ignored (hello123 world456 → hello: 1, world: 1)
- Only punctuation returns empty map
- Unicode letters (Café, CAFÉ naïve → café: 2, naïve: 1)

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

**Acceptance Tests:** `tasks/csvsum/sum_test.go`
- Normal operation with numeric values
- Missing column returns error
- Bad number (non-numeric) returns error
- Empty file returns 0, no error
- Empty cell counts as zero
- Header only (no data rows) returns 0

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
- Get moves entry to front (most recent)

**Acceptance Tests:** `tasks/lru/lru_test.go`
- LRU eviction order (basic eviction, access updates recency, update in place updates recency, no eviction when under capacity)
- Exact LRU order verification
- TTL expiry (entry expires after TTL, access does not reset TTL, update resets TTL, zero TTL means no expiry, expired entry removed from cache, Set evicts expired entries before LRU when at capacity)
- Update in place (update existing key keeps it in cache, update moves to front)
- Concurrent access safety (run with `go test -race`)

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

**Acceptance Tests:** `tasks/gocache/gocache_test.go`
- Handshake names get, put and close
- Put then get through Serve; nothing written to the shared folder
- Empty put; body size mismatch; unknown command; input ending without close
- Own folder before the shared one
- Entries go's own GOCACHE layout wrote, for IDs starting 00, 0a, 0f, 10, ff; Put lays out the same way
- Cut-short, missing and folder outputs miss
- Malformed entries (version, action, newline, output ID, size, fields) miss
- Put rejects bad IDs

The starting code builds and passes every test but the four bug reports'
(`TestStartsFail` in `cmd/score` checks that it fails), so landing it
unchanged is a fail.

**Run:** `go run ./cmd/score -v` (judges every trial of it)

---

## Trial Isolation

Each trial runs in its own folder under `trials/<harness>/<model>/<task>/` so
two trials never touch the same files: `trials/open/gpt-oss-120b/lru/`,
`trials/codex/gpt-5/wc/`. Its code is in the package the task's tests name
(`package wc`, `package csvsum`, `package lru`, `package gocache`), with the API above.

A trial's verdict is whether that code passes the task's acceptance tests:
`go run ./cmd/score` runs `tasks/<task>/*_test.go` against it, so the
trial's own tests, if any, don't decide it. See the README's Score.
