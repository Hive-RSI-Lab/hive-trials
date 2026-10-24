# LRU Cache with TTL

`lru` provides a simple thread‑safe LRU (least‑recently‑used) cache where each entry expires after a configurable TTL (time‑to‑live).

## Install

```bash
 go get ./...
```

## Usage

```go
import (
    "time"
    "your/module/path/lru"
)

func main() {
    // Create a cache that holds up to 100 items, each expiring after 30 seconds.
    c := lru.New(100, 30*time.Second)

    c.Set("foo", "bar")
    if v, ok := c.Get("foo"); ok {
        fmt.Println("value:", v)
    }

    // Length of the cache (including any items that may have expired but not yet accessed).
    fmt.Println("size:", c.Len())
}
```

## TTL in tests

The cache uses an injectable `Clock` (default `time.Now`). Tests can replace it via `SetClock` to deterministically control time:

```go
clk, advance := newMockClock(time.Now())
cache := lru.New(10, 5*time.Second)
cache.SetClock(clk)
cache.Set("key", "value")
advance(6 * time.Second) // move time forward
_, ok := cache.Get("key") // ok will be false – entry expired
```

## Concurrency

All public methods (`Set`, `Get`, `Len`, `SetClock`) are safe for concurrent use. The implementation guards internal state with a read‑write mutex.

## License

MIT – see the root `LICENSE` (if any).
