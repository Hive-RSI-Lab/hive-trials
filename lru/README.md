# LRU Cache with TTL

A thread-safe LRU (Least Recently Used) cache with TTL (Time-To-Live) support for Go.

## Features

- **LRU Eviction**: Automatically evicts least recently used items when capacity is reached
- **TTL Support**: Entries expire after a configurable time-to-live
- **Thread-Safe**: Safe for concurrent use with `sync.RWMutex`
- **Injectable Clock**: Test-friendly design with injectable clock interface
- **Zero Dependencies**: Standard library only

## Installation

```bash
go get github.com/yourusername/trials/lru
```

## Usage

```go
package main

import (
	"fmt"
	"time"
	"github.com/yourusername/trials/lru"
)

func main() {
	// Create a cache with capacity 100 and 5-minute TTL
	cache := lru.New(100, 5*time.Minute, nil)

	// Set a value
	cache.Set("key", "value")

	// Get a value
	if val, ok := cache.Get("key"); ok {
		fmt.Println(val) // prints: value
	}

	// Check cache size
	fmt.Println(cache.Len()) // prints: 1
}
```

### With Custom Clock (for Testing)

```go
import (
	"time"
	"github.com/yourusername/trials/lru"
)

type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time {
	return f.now
}

func (f *fakeClock) Advance(d time.Duration) {
	f.now = f.now.Add(d)
}

func main() {
	clk := &fakeClock{now: time.Now()}
	cache := lru.New(100, time.Hour, clk)

	cache.Set("key", "value")
	clk.Advance(2 * time.Hour)
	// Key is now expired
	val, ok := cache.Get("key") // ok == false
}
```

## API

### `New(capacity int, ttl time.Duration, clock Clock) *Cache`

Creates a new LRU cache.

- `capacity`: Maximum number of entries. Use `<= 0` for unlimited capacity.
- `ttl`: Time-to-live for entries. Use `<= 0` for no expiration.
- `clock`: Clock implementation for time. Pass `nil` to use real time.

### `Get(key string) (any, bool)`

Retrieves a value from the cache. Returns the value and `true` if found and not expired, otherwise `nil` and `false`. Updates the entry's recency (moves to most recently used).

### `Set(key string, value any)`

Adds or updates a value in the cache. If the key exists, updates its value and moves it to most recently used. If at capacity, evicts the least recently used entry.

### `Len() int`

Returns the current number of entries in the cache.

## Running Tests

```bash
# Run all tests
go test -v ./lru

# Run with race detector
go test -race -v ./lru

# Run vet
go vet ./lru
```

## Test Coverage

The test suite covers:

- **LRU Eviction Order**: Basic eviction, access updates recency, update-in-place updates recency
- **TTL Expiry**: Entry expiration, access does not reset TTL (TTL from creation), update resets TTL, zero TTL means no expiry, expired entries removed from cache
- **Update In Place**: Updating existing keys keeps them in cache, update moves to front
- **Concurrency**: Concurrent readers and writers, mixed read/write operations
- **Edge Cases**: Zero capacity (unlimited), negative capacity (unlimited)

All tests pass with `-race` flag and `go vet`.