# LRU Cache with TTL

A thread-safe LRU (Least Recently Used) cache with TTL (Time-To-Live) support for Go.

## Features

- **LRU Eviction**: Automatically evicts least recently used items when capacity is reached
- **TTL Support**: Entries expire after a configurable time-to-live
- **Thread-Safe**: Safe for concurrent use with `sync.RWMutex`
- **Test-Friendly**: Internal clock can be replaced for testing
- **Zero Dependencies**: Standard library only

## Installation

This module is part of the `trials` module. Import it as:

```go
import "trials/lru"
```

(This requires working within the `trials` module or replacing the module path with your own.)

## Usage

```go
package main

import (
	"fmt"
	"time"
	"trials/lru"
)

func main() {
	// Create a cache with capacity 100 and 5-minute TTL
	cache := lru.New(100, 5*time.Minute)

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

### Testing with Custom Clock

The package uses an internal clock that can be replaced in tests via an unexported method. See `lru_test.go` for examples.

## API

### `New(capacity int, ttl time.Duration) *Cache`

Creates a new LRU cache.

- `capacity`: Maximum number of entries. Use `<= 0` for unlimited capacity.
- `ttl`: Time-to-live for entries. Use `<= 0` for no expiration.

### `Get(key string) (any, bool)`

Retrieves a value from the cache. Returns the value and `true` if found and not expired, otherwise `nil` and `false`. Updates the entry's recency (moves to most recently used).

### `Set(key string, value any)`

Adds or updates a value in the cache. If the key exists, updates its value and moves it to most recently used. If at capacity, first removes all expired entries, then evicts the least recently used entry if still over capacity.

### `Len() int`

Returns the current number of entries in the cache (including expired entries that have not yet been accessed or evicted).

Note: expired entries remain in the cache until they are accessed (via Get) or until they become the least-recently-used entry and are evicted during a Set operation when the cache is at capacity.

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

- **LRU Eviction Order**: Basic eviction, access updates recency, update-in-place updates recency, exact order verification via forced evictions
- **TTL Expiry**: Entry expiration, access does not reset TTL (TTL from creation), update resets TTL, zero TTL means no expiry, expired entries removed from cache
- **Update In Place**: Updating existing keys keeps them in cache, update moves to front
- **Concurrency**: Concurrent readers and writers, mixed read/write operations
- **Edge Cases**: Zero capacity (unlimited), negative capacity (unlimited)

All tests pass with `-race` flag and `go vet`.