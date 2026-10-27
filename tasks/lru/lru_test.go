//go:build acceptance

package lru

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeClock is a controllable clock for testing.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func TestLRUEvictionOrder(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		ops      func(*Cache)
		wantKeys []string // expected keys in order from most to least recent
	}{
		{
			name:     "basic eviction",
			capacity: 3,
			ops: func(c *Cache) {
				c.Set("a", 1)
				c.Set("b", 2)
				c.Set("c", 3)
				c.Set("d", 4) // should evict "a"
			},
			wantKeys: []string{"d", "c", "b"},
		},
		{
			name:     "access updates recency",
			capacity: 3,
			ops: func(c *Cache) {
				c.Set("a", 1)
				c.Set("b", 2)
				c.Set("c", 3)
				c.Get("a")    // access "a" - should become most recent
				c.Set("d", 4) // should evict "b" (least recent)
			},
			wantKeys: []string{"d", "a", "c"},
		},
		{
			name:     "update in place updates recency",
			capacity: 3,
			ops: func(c *Cache) {
				c.Set("a", 1)
				c.Set("b", 2)
				c.Set("c", 3)
				c.Set("b", 20) // update "b" - should become most recent
				c.Set("d", 4)  // should evict "a" (least recent)
			},
			wantKeys: []string{"d", "b", "c"},
		},
		{
			name:     "no eviction when under capacity",
			capacity: 5,
			ops: func(c *Cache) {
				c.Set("a", 1)
				c.Set("b", 2)
				c.Set("c", 3)
			},
			wantKeys: []string{"c", "b", "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.capacity, 0)
			tt.ops(c)

			// Verify length
			if c.Len() != len(tt.wantKeys) {
				t.Errorf("Len() = %d, want %d", c.Len(), len(tt.wantKeys))
			}

			// Verify all expected keys exist and have correct values
			for _, key := range tt.wantKeys {
				val, ok := c.Get(key)
				if !ok {
					t.Errorf("key %q not found", key)
				}
				if val == nil {
					t.Errorf("key %q has nil value", key)
				}
			}

			// Verify evicted keys are gone
			allKeys := []string{"a", "b", "c", "d"}
			for _, key := range allKeys {
				found := false
				for _, wk := range tt.wantKeys {
					if key == wk {
						found = true
						break
					}
				}
				if !found {
					_, ok := c.Get(key)
					if ok {
						t.Errorf("key %q should have been evicted", key)
					}
				}
			}
		})
	}
}

func TestLRUEvictionOrderExact(t *testing.T) {
	// Test exact LRU order by forcing evictions and checking which key is evicted
	c := New(3, 0)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	// Order: c (MRU), b, a (LRU)

	// Add d - should evict a
	c.Set("d", 4)
	if c.Len() != 3 {
		t.Errorf("expected Len=3, got %d", c.Len())
	}
	_, ok := c.Get("a")
	if ok {
		t.Errorf("a should be evicted")
	}

	// Order now: d, c, b
	// Access b - moves to front
	c.Get("b")
	// Order: b, d, c
	// Add e - should evict c
	c.Set("e", 5)
	_, ok = c.Get("c")
	if ok {
		t.Errorf("c should be evicted")
	}

	// Order now: e, b, d
	// Update d - moves to front
	c.Set("d", 40)
	// Order: d, e, b
	// Add f - should evict b
	c.Set("f", 6)
	_, ok = c.Get("b")
	if ok {
		t.Errorf("b should be evicted")
	}

	// Final: f, d, e
	for _, key := range []string{"f", "d", "e"} {
		if _, ok := c.Get(key); !ok {
			t.Errorf("%s should exist", key)
		}
	}
}

func TestTTLExpiry(t *testing.T) {
	tests := []struct {
		name     string
		ttl      time.Duration
		capacity int
		setup    func(*Cache, *fakeClock)
		check    func(*testing.T, *Cache, *fakeClock)
	}{
		{
			name: "entry expires after TTL",
			ttl:  100 * time.Millisecond,
			setup: func(c *Cache, clk *fakeClock) {
				clk.now = time.Now()
				c.Set("key", "value")
			},
			check: func(t *testing.T, c *Cache, clk *fakeClock) {
				// Immediately after set - should exist
				val, ok := c.Get("key")
				if !ok || val != "value" {
					t.Errorf("expected key to exist immediately, got %v, %v", val, ok)
				}

				// Advance past TTL - should be expired
				clk.Advance(150 * time.Millisecond)
				val, ok = c.Get("key")
				if ok {
					t.Errorf("expected key to be expired, got %v", val)
				}
			},
		},
		{
			name: "access does not reset TTL (TTL is from creation)",
			ttl:  100 * time.Millisecond,
			setup: func(c *Cache, clk *fakeClock) {
				clk.now = time.Now()
				c.Set("key", "value")
			},
			check: func(t *testing.T, c *Cache, clk *fakeClock) {
				// Advance half TTL
				clk.Advance(50 * time.Millisecond)
				// Access the key - TTL NOT reset
				val, ok := c.Get("key")
				if !ok || val != "value" {
					t.Errorf("expected key to exist at 50ms, got %v, %v", val, ok)
				}

				// Advance another 60ms (total 110ms) - should expire
				clk.Advance(60 * time.Millisecond)
				val, ok = c.Get("key")
				if ok {
					t.Errorf("expected key to be expired at 110ms, got %v", val)
				}
			},
		},
		{
			name: "update resets TTL",
			ttl:  100 * time.Millisecond,
			setup: func(c *Cache, clk *fakeClock) {
				clk.now = time.Now()
				c.Set("key", "value1")
			},
			check: func(t *testing.T, c *Cache, clk *fakeClock) {
				// Advance half TTL
				clk.Advance(50 * time.Millisecond)
				// Update the key - should reset TTL
				c.Set("key", "value2")

				// Advance another 75ms (total 125ms, but TTL was reset at 50ms)
				clk.Advance(75 * time.Millisecond)
				val, ok := c.Get("key")
				if !ok || val != "value2" {
					t.Errorf("expected key to exist with new value at 125ms, got %v, %v", val, ok)
				}

				// Advance another 50ms - should expire
				clk.Advance(50 * time.Millisecond)
				val, ok = c.Get("key")
				if ok {
					t.Errorf("expected key to be expired at 175ms, got %v", val)
				}
			},
		},
		{
			name: "zero TTL means no expiry",
			ttl:  0,
			setup: func(c *Cache, clk *fakeClock) {
				clk.now = time.Now()
				c.Set("key", "value")
			},
			check: func(t *testing.T, c *Cache, clk *fakeClock) {
				clk.Advance(10 * time.Hour)
				val, ok := c.Get("key")
				if !ok || val != "value" {
					t.Errorf("expected key to never expire with TTL=0, got %v, %v", val, ok)
				}
			},
		},
		{
			name: "expired entry is removed from cache",
			ttl:  100 * time.Millisecond,
			setup: func(c *Cache, clk *fakeClock) {
				clk.now = time.Now()
				c.Set("key", "value")
			},
			check: func(t *testing.T, c *Cache, clk *fakeClock) {
				clk.Advance(150 * time.Millisecond)
				// Access should return false and remove from cache
				val, ok := c.Get("key")
				if ok {
					t.Errorf("expected expired key to return false, got %v", val)
				}
				// Len should be 0
				if c.Len() != 0 {
					t.Errorf("expected Len()=0 after expiry, got %d", c.Len())
				}
			},
		},
		{
			name:     "Set evicts expired entries before LRU when at capacity",
			ttl:      100 * time.Millisecond,
			capacity: 3,
			setup: func(c *Cache, clk *fakeClock) {
				clk.now = time.Now()
				// Fill cache to capacity (3)
				c.Set("a", 1)
				c.Set("b", 2)
				c.Set("c", 3)
				// a is LRU, c is MRU
				// Advance past TTL - all entries expired
				clk.Advance(150 * time.Millisecond)
			},
			check: func(t *testing.T, c *Cache, clk *fakeClock) {
				// All entries are expired but still in cache (Len=3)
				if c.Len() != 3 {
					t.Errorf("expected Len()=3 with expired entries, got %d", c.Len())
				}

				// Set a new entry - should evict expired entries first
				c.Set("d", 4)

				// Expired entries should be gone, only "d" remains
				if c.Len() != 1 {
					t.Errorf("expected Len()=1 after evicting expired, got %d", c.Len())
				}

				// "d" should exist
				val, ok := c.Get("d")
				if !ok || val != 4 {
					t.Errorf("expected d=4, got %v, %v", val, ok)
				}

				// Expired entries should be gone
				for _, key := range []string{"a", "b", "c"} {
					_, ok := c.Get(key)
					if ok {
						t.Errorf("expired key %q should have been evicted", key)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clk := &fakeClock{now: time.Now()}
			capacity := 10
			if tt.capacity > 0 {
				capacity = tt.capacity
			}
			c := New(capacity, tt.ttl)
			c.setClockForTest(clk)
			tt.setup(c, clk)
			tt.check(t, c, clk)
		})
	}
}

func TestUpdateInPlace(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Cache)
		check func(*testing.T, *Cache)
	}{
		{
			name: "update existing key keeps it in cache",
			setup: func(c *Cache) {
				c.Set("a", 1)
				c.Set("b", 2)
			},
			check: func(t *testing.T, c *Cache) {
				c.Set("a", 100) // update
				val, ok := c.Get("a")
				if !ok || val != 100 {
					t.Errorf("expected a=100, got %v, %v", val, ok)
				}
				val, ok = c.Get("b")
				if !ok || val != 2 {
					t.Errorf("expected b=2, got %v, %v", val, ok)
				}
				if c.Len() != 2 {
					t.Errorf("expected Len()=2, got %d", c.Len())
				}
			},
		},
		{
			name: "update moves to front (most recent)",
			setup: func(c *Cache) {
				c.Set("a", 1)
				c.Set("b", 2)
				c.Set("c", 3)
			},
			check: func(t *testing.T, c *Cache) {
				c.Set("a", 100) // update "a" - should become most recent
				// Add "d" which should evict "b" (now least recent)
				c.Set("d", 4)

				// "a" should exist (updated)
				val, ok := c.Get("a")
				if !ok || val != 100 {
					t.Errorf("expected a=100, got %v, %v", val, ok)
				}
				// "d" should exist (newest)
				val, ok = c.Get("d")
				if !ok || val != 4 {
					t.Errorf("expected d=4, got %v, %v", val, ok)
				}
				// "c" should exist
				val, ok = c.Get("c")
				if !ok || val != 3 {
					t.Errorf("expected c=3, got %v, %v", val, ok)
				}
				// "b" should be evicted
				_, ok = c.Get("b")
				if ok {
					t.Errorf("expected b to be evicted")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(3, 0)
			tt.setup(c)
			tt.check(t, c)
		})
	}
}

// verifyCacheConsistency checks that the linked list and map are in sync,
// and that Len() is within capacity bounds.
func verifyCacheConsistency(t *testing.T, c *Cache, capacity int) {
	t.Helper()

	if c.Len() < 0 {
		t.Errorf("Len() should not be negative, got %d", c.Len())
	}
	if capacity > 0 && c.Len() > capacity {
		t.Errorf("Len() = %d exceeds capacity %d", c.Len(), capacity)
	}

	// Walk the linked list and verify each node is in the map
	count := 0
	for n := c.head.next; n != c.tail; n = n.next {
		count++
		if _, ok := c.items[n.key]; !ok {
			t.Errorf("node with key %q in list but not in map", n.key)
		}
	}

	// Verify map size matches list count
	if count != len(c.items) {
		t.Errorf("list count (%d) != map size (%d)", count, len(c.items))
	}

	// Verify all map entries are in the list
	for key, n := range c.items {
		found := false
		for m := c.head.next; m != c.tail; m = m.next {
			if m == n {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("key %q in map but not in list", key)
		}
	}
}

func TestConcurrency(t *testing.T) {
	c := New(100, time.Hour)
	var wg sync.WaitGroup
	numGoroutines := 50
	opsPerGoroutine := 100

	// Concurrent writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				c.Set(key, id*opsPerGoroutine+j)
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				c.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Verify cache consistency and capacity bounds
	verifyCacheConsistency(t, c, 100)
}

func TestConcurrentReadWrite(t *testing.T) {
	c := New(1000, time.Hour)
	var wg sync.WaitGroup
	numGoroutines := 100
	opsPerGoroutine := 1000

	// Pre-populate
	for i := 0; i < 500; i++ {
		c.Set(fmt.Sprintf("pre-%d", i), i)
	}

	// Mix of readers and writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				if j%2 == 0 {
					c.Set(fmt.Sprintf("key-%d-%d", id, j), id*opsPerGoroutine+j)
				} else {
					c.Get(fmt.Sprintf("key-%d-%d", id, j))
					c.Get(fmt.Sprintf("pre-%d", j%500))
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify cache consistency and capacity bounds
	verifyCacheConsistency(t, c, 1000)
}

func TestCapacityZero(t *testing.T) {
	c := New(0, 0) // No capacity limit
	for i := 0; i < 1000; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i)
	}
	if c.Len() != 1000 {
		t.Errorf("expected Len()=1000 with capacity 0, got %d", c.Len())
	}
}

func TestNegativeCapacity(t *testing.T) {
	c := New(-1, 0) // No capacity limit
	for i := 0; i < 1000; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i)
	}
	if c.Len() != 1000 {
		t.Errorf("expected Len()=1000 with negative capacity, got %d", c.Len())
	}
}
