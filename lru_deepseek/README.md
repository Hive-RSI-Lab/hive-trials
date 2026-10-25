# lru_deepseek

A concurrent-safe LRU cache with per-entry TTL, written in Go.

This is a trial of the **deepseek-v4-pro** model. See the `lru` package for the
openrouter trial on the same task.

## Installation

```bash
go get trials/lru_deepseek
```

## Usage

```go
package main

import (
	"fmt"
	"time"
	"trials/lru_deepseek"
)

func main() {
	// Create a cache with capacity 100 and 5-minute TTL.
	c := lru_deepseek.New(100, 5*time.Minute)

	c.Set("key", "value")
	if val, ok := c.Get("key"); ok {
		fmt.Println(val) // "value"
	}
	fmt.Println(c.Len()) // 1
}
```

## API

- `New(capacity int, ttl time.Duration) *Cache` — create a cache; capacity clamped to ≥ 1. `ttl <= 0` disables expiry.
- `Get(key string) (any, bool)` — returns the value and true if present and not expired; moves the entry to the front of the LRU list.
- `Set(key string, value any)` — adds or updates an entry; refreshes TTL on update; evicts the least-recently-used entry when over capacity.
- `Len() int` — returns the current number of entries (including expired ones not yet evicted by a Get/Set).
- `SetClock(clock Clock)` — inject a clock for tests. `Clock` is `interface { Now() time.Time }`.

Expired entries are removed lazily on `Get` or when evicted by `Set`.

## Running Tests

```bash
go test -v ./lru_deepseek
go test -race ./lru_deepseek
go vet ./lru_deepseek
```

## Test Coverage

- **LRU eviction order**: basic eviction, access updates recency, update-in-place moves to front
- **TTL expiry**: entry expiration after TTL, exact-at-boundary, update refreshes TTL, ttl ≤ 0 disables expiry
- **Expired eviction**: Set evicts expired oldest at capacity, lazy removal on Get
- **Concurrency**: 200 goroutine readers and writers
- **Edge cases**: capacity 1, zero/negative capacity clamped to 1