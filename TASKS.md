# Fixed Task Set

This file defines the three fixed small tasks used for trials. Each task has its reference implementation and acceptance tests under `tasks/<task>/`.

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

**Run:** `go test ./tasks/wc/...`

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

**Run:** `go test ./tasks/csvsum/...`

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

**Run:** `go test ./tasks/lru/...`

---

## Trial Isolation

Each trial runs in its own folder under `trials/<harness>/<model>/<task>/` so
two trials never touch the same files: `trials/open/gpt-oss-120b/lru/`,
`trials/codex/gpt-5/wc/`. Its code is in the package the task's tests name
(`package wc`, `package csvsum`, `package lru`), with the API above.

A trial's verdict is whether that code passes the task's acceptance tests:
`go run ./cmd/score` runs `tasks/<task>/*_test.go` against it, so the
trial's own tests, if any, don't decide it. See the README's Score.
