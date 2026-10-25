package lru_deepseek

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Unix(0, 0)}
}

// TestEvictionOrder verifies LRU eviction: the least-recently-used entry
// is dropped when capacity is exceeded.
func TestEvictionOrder(t *testing.T) {
	c := New(3, time.Hour)
	clk := newFakeClock()
	c.SetClock(clk)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	// a is now oldest, c newest. Access a to make it newest.
	c.Get("a")
	// Now b is oldest.
	c.Set("d", 4) // capacity 3 exceeded, b should be evicted

	if _, ok := c.Get("b"); ok {
		t.Error("expected b to be evicted")
	}
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("expected a=1, got %v, ok=%v", v, ok)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("expected c=3, got %v, ok=%v", v, ok)
	}
	if v, ok := c.Get("d"); !ok || v != 4 {
		t.Errorf("expected d=4, got %v, ok=%v", v, ok)
	}
}

// TestExpiry verifies entries expire after TTL.
func TestExpiry(t *testing.T) {
	c := New(10, time.Second)
	clk := newFakeClock()
	c.SetClock(clk)

	c.Set("a", 1)
	clk.advance(500 * time.Millisecond)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("expected a=1 at 0.5s, got %v, ok=%v", v, ok)
	}

	clk.advance(600 * time.Millisecond) // total 1.1s > 1s TTL
	if _, ok := c.Get("a"); ok {
		t.Error("expected a to be expired after 1.1s")
	}
}

// TestExpiryEviction verifies expired entries are removed lazily on access.
// They still count toward Len until that access.
func TestExpiryEviction(t *testing.T) {
	c := New(2, time.Second)
	clk := newFakeClock()
	c.SetClock(clk)

	c.Set("a", 1)
	c.Set("b", 2)
	clk.advance(2 * time.Second)

	// Both are expired. Get on a should remove it, returning not found.
	if _, ok := c.Get("a"); ok {
		t.Error("expected a to be expired")
	}
	// Len still counts until evicted, but the expired entry is removed on Get.
	// b is still in the list until accessed.
	if c.Len() != 1 {
		t.Errorf("expected Len=1 after a removed, got %d", c.Len())
	}
}

// TestUpdate verifies Set on an existing key updates value and moves to front.
func TestUpdate(t *testing.T) {
	c := New(2, time.Hour)
	clk := newFakeClock()
	c.SetClock(clk)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("a", 10) // update a; a moves to front, b is now oldest.
	c.Set("c", 3)  // should evict b

	if _, ok := c.Get("b"); ok {
		t.Error("expected b to be evicted")
	}
	if v, ok := c.Get("a"); !ok || v != 10 {
		t.Errorf("expected a=10, got %v, ok=%v", v, ok)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("expected c=3, got %v, ok=%v", v, ok)
	}
}

// TestUpdateRefreshesExpiry verifies Set on an existing key refreshes its TTL.
func TestUpdateRefreshesExpiry(t *testing.T) {
	c := New(10, time.Second)
	clk := newFakeClock()
	c.SetClock(clk)

	c.Set("a", 1)
	clk.advance(900 * time.Millisecond)
	c.Set("a", 2) // refresh expiry
	clk.advance(500 * time.Millisecond) // total 1.4s since first set, 0.5s since refresh

	if v, ok := c.Get("a"); !ok || v != 2 {
		t.Errorf("expected a=2 after refresh, got %v, ok=%v", v, ok)
	}

	clk.advance(600 * time.Millisecond) // 1.1s since refresh
	if _, ok := c.Get("a"); ok {
		t.Error("expected a to be expired 1.1s after refresh")
	}
}

// TestGetRefreshesPosition verifies Get moves the entry to the front.
func TestGetRefreshesPosition(t *testing.T) {
	c := New(3, time.Hour)
	clk := newFakeClock()
	c.SetClock(clk)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	// Access order: c(newest), b, a(oldest)
	c.Get("a") // a moves to front: a(newest), c, b(oldest)
	c.Get("b") // b moves to front: b(newest), a, c(oldest)
	c.Set("d", 4) // evict c

	if _, ok := c.Get("c"); ok {
		t.Error("expected c to be evicted")
	}
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("expected a=1, got %v, ok=%v", v, ok)
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("expected b=2, got %v, ok=%v", v, ok)
	}
}

// TestCapacityOne verifies eviction with a capacity-1 cache.
func TestCapacityOne(t *testing.T) {
	c := New(1, time.Hour)
	clk := newFakeClock()
	c.SetClock(clk)

	c.Set("a", 1)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("expected a=1, got %v, ok=%v", v, ok)
	}
	c.Set("b", 2)
	if _, ok := c.Get("a"); ok {
		t.Error("expected a evicted")
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("expected b=2, got %v, ok=%v", v, ok)
	}
}

// TestZeroOrNegativeCapacity verifies New clamps bad capacity.
func TestZeroOrNegativeCapacity(t *testing.T) {
	c := New(0, time.Hour)
	// Should be clamped to 1.
	c.Set("a", 1)
	c.Set("b", 2)
	if _, ok := c.Get("a"); ok {
		t.Error("expected a evicted with capacity 1")
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("expected b=2, got %v, ok=%v", v, ok)
	}
}

// TestSetEvictsExpiredOldest verifies Set evicts at capacity even when the
// oldest entry has already expired.
func TestSetEvictsExpiredOldest(t *testing.T) {
	c := New(2, time.Second)
	clk := newFakeClock()
	c.SetClock(clk)

	c.Set("a", 1)
	c.Set("b", 2)
	clk.advance(2 * time.Second) // both expired

	// Set a new key — should evict the oldest (a) even though both are expired.
	c.Set("c", 3)
	if _, ok := c.Get("a"); ok {
		t.Error("expected a to be evicted")
	}
	if _, ok := c.Get("b"); ok {
		t.Error("expected b to be expired")
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("expected c=3, got %v, ok=%v", v, ok)
	}
}

// TestExpiryAtBoundary verifies expiry exactly at the TTL boundry:
// now >= expiry (ie !now.Before(expiry)) means expired.
func TestExpiryAtBoundary(t *testing.T) {
	c := New(10, time.Second)
	clk := newFakeClock()
	c.SetClock(clk)

	c.Set("a", 1)
	clk.advance(time.Second) // exactly at TTL boundary

	if _, ok := c.Get("a"); ok {
		t.Error("expected a to be expired exactly at TTL")
	}
}

// TestTTLZeroMeansNoExpiry verifies that ttl <= 0 disables expiry.
func TestTTLZeroMeansNoExpiry(t *testing.T) {
	c := New(10, 0) // ttl = 0: no expiry
	clk := newFakeClock()
	c.SetClock(clk)

	c.Set("a", 1)
	clk.advance(24 * time.Hour) // far in the future
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("expected a=1 with ttl=0, got %v, ok=%v", v, ok)
	}

	// Also test negative TTL.
	c2 := New(10, -time.Second)
	c2.SetClock(newFakeClock())
	c2.Set("b", 2)
	if v, ok := c2.Get("b"); !ok || v != 2 {
		t.Errorf("expected b=2 with negative ttl, got %v, ok=%v", v, ok)
	}
}

// TestConcurrent runs many concurrent readers and writers to verify
// the cache is safe for concurrent use. Run with go test -race.
func TestConcurrent(t *testing.T) {
	c := New(100, 10*time.Second)

	var wg sync.WaitGroup
	n := 100

	// Writers
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				key := fmt.Sprintf("key-%d", j%200)
				c.Set(key, j)
			}
		}(i)
	}

	// Readers
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				key := fmt.Sprintf("key-%d", j%200)
				c.Get(key)
			}
		}(i)
	}

	wg.Wait()
	// If we reached here without a data race, the test passes.
	if c.Len() < 0 {
		t.Error("negative len")
	}
}