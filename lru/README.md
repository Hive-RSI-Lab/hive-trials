# lru

A concurrent-safe LRU cache with per-entry TTL, written in Go.

## Features

- **LRU Eviction**: automatically evicts least-recently-used entries at capacity
- **TTL Support**: entries expire after a configurable time-to-live
- **Concurrent-safe**: safe for concurrent use (`sync.Mutex`)
- **Test-friendly**: injectable `Clock` interface for tests
- **Zero dependencies**: standard library only

## Installation

```bash
go get trials/lru
```

## Usage

```go
package main

import (
	"fmt"
	"time"
	"trials/lru"
)

func main() {
	// Create a cache with capacity 100 and 5-minute TTL.
	cache := lru.New(100, 5*time.Minute)

	cache.Set("key", "value")
	if val, ok := cache.Get("key"); ok {
		fmt.Println(val) // "value"
	}
	fmt.Println(cache.Len()) // 1
}
```

## API

- `New(capacity int, ttl time.Duration) *Cache` — create a cache; capacity clamped to ≥ 1.
- `Get(key string) (any, bool)` — returns the value and true if present and not expired; moves the entry to the front of the LRU list.
- `Set(key string, value any)` — adds or updates an entry; refreshes TTL on update; evicts the least-recently-used entry when over capacity.
- `Len() int` — returns the current number of entries (including expired ones not yet evicted by a Get/Set).
- `SetClock(clock Clock)` — inject a clock for tests. `Clock` is `interface { Now() time.Time }`.

Expired entries are removed lazily on `Get` or when evicted by `Set`.

## Running Tests

```bash
go test -v ./lru
go test -race ./lru
go vet ./lru
```

## Test Coverage

- **LRU eviction order**: basic eviction, access updates recency, update-in-place moves to front
- **TTL expiry**: entry expiration after TTL, exact-at-boundary, update refreshes TTL
- **Expired eviction**: Set evicts expired oldest at capacity, lazy removal on Get
- **Concurrency**: 200 goroutine readers and writers
- **Edge cases**: capacity 1, zero capacity clamped to 1