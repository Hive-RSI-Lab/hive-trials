package lru

import (
    "fmt"
    "sync"
    "testing"
    "time"
)

// newMockClock returns a controllable Clock and a function to advance time.
func newMockClock(start time.Time) (Clock, func(d time.Duration)) {
    mu := sync.Mutex{}
    cur := start
    advance := func(d time.Duration) {
        mu.Lock()
        defer mu.Unlock()
        cur = cur.Add(d)
    }
    clk := func() time.Time {
        mu.Lock()
        defer mu.Unlock()
        return cur
    }
    return clk, advance
}

func TestCacheEvictionOrder(t *testing.T) {
    c := New(2, 0) // no ttl
    c.Set("a", 1)
    c.Set("b", 2)
    // Access a to make it most recently used.
    if _, ok := c.Get("a"); !ok {
        t.Fatalf("expected a present")
    }
    // Adding a third key should evict b.
    c.Set("c", 3)
    if _, ok := c.Get("b"); ok {
        t.Fatalf("expected b to be evicted")
    }
    if _, ok := c.Get("a"); !ok {
        t.Fatalf("expected a still present")
    }
    if _, ok := c.Get("c"); !ok {
        t.Fatalf("expected c present")
    }
    if got, want := c.Len(), 2; got != want {
        t.Fatalf("len = %d, want %d", got, want)
    }
}

func TestCacheTTLExpiry(t *testing.T) {
    start := time.Unix(0, 0)
    clk, adv := newMockClock(start)
    ttl := 10 * time.Second
    c := New(5, ttl)
    c.SetClock(clk)
    c.Set("x", "val")
    if _, ok := c.Get("x"); !ok {
        t.Fatalf("expected x present before ttl")
    }
    // advance beyond ttl
    adv(11 * time.Second)
    if _, ok := c.Get("x"); ok {
        t.Fatalf("expected x expired")
    }
    if got := c.Len(); got != 0 {
        t.Fatalf("expected Len 0 after expiry, got %d", got)
    }
}

func TestCacheUpdateResetsTTL(t *testing.T) {
    start := time.Unix(0, 0)
    clk, adv := newMockClock(start)
    ttl := 5 * time.Second
    c := New(5, ttl)
    c.SetClock(clk)
    c.Set("k", 1)
    adv(3 * time.Second)
    // update should reset ttl
    c.Set("k", 2)
    adv(3 * time.Second) // total 6 sec from original set, but only 3 from update
    if v, ok := c.Get("k"); !ok || v.(int) != 2 {
        t.Fatalf("expected k still valid after update, got %v, ok=%v", v, ok)
    }
}

func TestCacheConcurrentAccess(t *testing.T) {
    c := New(100, 0)
    wg := sync.WaitGroup{}
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                key := fmt.Sprintf("%d-%d", id, j)
                c.Set(key, j)
                if v, ok := c.Get(key); ok && v.(int) != j {
                    t.Errorf("mismatch for %s: got %v, want %d", key, v, j)
                }
            }
        }(i)
    }
    wg.Wait()
}
