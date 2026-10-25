# LRU Cache with TTL

A thread-safe LRU (Least Recently Used) cache with TTL (Time To Live) expiration support for Go.

## Features

- **LRU Eviction**: Automatically evicts least recently used entries when capacity is reached
- **TTL Expiration**: Entries automatically expire after a configurable time-to-live
- **Thread-Safe**: Safe for concurrent use with `sync.RWMutex`
- **Testable**: Inject a fake clock for deterministic time-based testing
- **Zero Dependencies**: Uses only Go standard library

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
	// Create a cache with capacity 100 and 1-hour TTL
	cache := lru.New(100, time.Hour)
	
	// Set values
	cache.Set("user:1", User{Name: "Alice", Email: "alice@example.com"})
	cache.Set("user:2", User{Name: "Bob", Email: "bob@example.com"})
	
	// Get values
	if user, ok := cache.Get("user:1"); ok {
		fmt.Println("Found:", user.(User).Name)
	}
	
	// Check cache size
	fmt.Println("Cache size:", cache.Len())
}

// With custom clock for testing
func testWithFakeClock() {
	clock := lru.NewFakeClock(time.Now())
	cache := lru.New(10, time.Hour, clock)
	
	cache.Set("key", "value")
	
	// Advance time past TTL
	clock.Advance(2 * time.Hour)
	
	// Entry should be expired
	_, ok := cache.Get("key")
	fmt.Println("Expired:", !ok) // true
}
```

## API

### `New(capacity int, ttl time.Duration, clock ...Clock) *Cache`

Creates a new LRU cache.

- `capacity`: Maximum number of entries before LRU eviction kicks in
- `ttl`: Time-to-live for entries. Use `0` or negative for no expiration
- `clock`: Optional clock implementation for testing (defaults to real clock)

### `Get(key string) (value any, ok bool)`

Retrieves a value from the cache. Returns the value and `true` if found and not expired. Returns `nil, false` if not found or expired. Updates the entry's position to most recently used.

### `Set(key string, value any)`

Adds or updates a value in the cache. If the key exists, updates the value and resets its TTL. If at capacity, evicts expired entries first, then the least recently used entry.

### `Len() int`

Returns the current number of entries in the cache.

### `Clock` Interface

```go
type Clock interface {
	Now() time.Time
}
```

Implement this interface to provide a custom time source for testing.

### `NewFakeClock(start time.Time) *FakeClock` (for testing)

Creates a controllable clock for testing time-based behavior.

```go
clock := lru.NewFakeClock(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
cache := lru.New(10, time.Hour, clock)

cache.Set("key", "value")
clock.Advance(2 * time.Hour) // Advance time
// Entry is now expired
```

## Testing

Run tests with race detection:

```bash
go test -race ./lru/...
```