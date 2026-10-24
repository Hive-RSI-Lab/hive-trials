# lru

A concurrent-safe LRU cache with per-entry TTL, written in Go.

## Usage

```go
import "github.com/Hive-RSI-Lab/trials/lru"

// Create a cache that holds up to 100 entries, each expiring after 5 minutes.
c := lru.New(100, 5*time.Minute)

c.Set("key", "value")
v, ok := c.Get("key")
n := c.Len()
```

- `New(capacity int, ttl time.Duration) *Cache` — create a cache; capacity clamped to ≥1.
- `Get(key string) (any, bool)` — returns the value and true if present and not expired; moves the entry to the front of the LRU list.
- `Set(key string, value any)` — adds or updates an entry; refreshes TTL on update; evicts the least-recently-used entry when over capacity.
- `Len() int` — returns the current number of entries (including expired ones not yet evicted by a Get/Set).
- `SetClock(clock Clock)` — inject a clock for tests; `Clock` is `interface { Now() time.Time }`.

Expired entries are removed lazily on `Get` or when evicted by `Set`. The cache is safe for concurrent use.

## Testing

```sh
go test -race ./lru/
```
