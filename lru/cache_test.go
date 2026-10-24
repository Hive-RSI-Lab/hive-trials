package lru

import (
    "container/list"
    "strconv"
    "sync"
    "testing"
    "time"
)

type fakeClock struct {
    mu sync.Mutex
    t  time.Time
}

func (fc *fakeClock) Now() time.Time {
    fc.mu.Lock()
    defer fc.mu.Unlock()
    return fc.t
}

func (fc *fakeClock) Add(d time.Duration) {
    fc.mu.Lock()
    fc.t = fc.t.Add(d)
    fc.mu.Unlock()
}

func TestCacheEvictionOrder(t *testing.T) {
    c := &Cache{capacity: 2, ttl: time.Hour, clock: &fakeClock{t: time.Now()}, items: make(map[string]*list.Element), order: list.New()}
    c.Set("a", 1)
    c.Set("b", 2)
    // access a to make it most recent
    if v, ok := c.Get("a"); !ok || v.(int) != 1 {
        t.Fatalf("expected a present")
    }
    // inserting c should evict b (least recent)
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
}

func TestCacheExpiry(t *testing.T) {
    fc := &fakeClock{t: time.Now()}
    c := &Cache{capacity: 2, ttl: time.Second, clock: fc, items: make(map[string]*list.Element), order: list.New()}
    c.Set("x", 42)
    if _, ok := c.Get("x"); !ok {
        t.Fatalf("expected x present initially")
    }
    fc.Add(2 * time.Second)
    if _, ok := c.Get("x"); ok {
        t.Fatalf("expected x expired")
    }
    if l := c.Len(); l != 0 {
        t.Fatalf("expected Len 0 after expiry, got %d", l)
    }
}

func TestCacheUpdateInPlace(t *testing.T) {
    c := &Cache{capacity: 1, ttl: time.Hour, clock: &fakeClock{t: time.Now()}, items: make(map[string]*list.Element), order: list.New()}
    c.Set("k", 1)
    c.Set("k", 2) // update same key
    if v, ok := c.Get("k"); !ok || v.(int) != 2 {
        t.Fatalf("expected updated value 2, got %v, ok=%v", v, ok)
    }
}

func TestCacheConcurrency(t *testing.T) {
    fc := &fakeClock{t: time.Now()}
    c := &Cache{capacity: 100, ttl: time.Hour, clock: fc, items: make(map[string]*list.Element), order: list.New()}
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(2)
        go func(i int) {
            defer wg.Done()
            c.Set("key"+strconv.Itoa(i), i)
        }(i)
        go func(i int) {
            defer wg.Done()
            c.Get("key" + strconv.Itoa(i))
        }(i)
    }
    wg.Wait()
    // just ensure no panic and Len reasonable
    if l := c.Len(); l > 100 {
        t.Fatalf("unexpected length %d", l)
    }
}
