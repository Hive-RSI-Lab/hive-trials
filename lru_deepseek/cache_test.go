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

// evictionTestCase groups LRU eviction scenarios.
type evictionTestCase struct {
	name        string
	capacity    int
	ttl         time.Duration
	ops         func(c *Cache, clk *fakeClock)
	want        map[string]any // key -> expected value
	wantMissing []string       // keys that should not be present
	wantLen     int            // expected Len() at end (optional, -1 to skip)
}

// evictionCases covers basic eviction, access updates recency,
// update-in-place moves to front, and Set evicting expired oldest at capacity.
var evictionCases = []evictionTestCase{
	{
		name:     "basic LRU eviction",
		capacity: 3,
		ttl:      time.Hour,
		ops: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
			c.Set("b", 2)
			c.Set("c", 3)
			c.Get("a")    // a becomes most recent
			c.Set("d", 4) // evicts b
		},
		want:        map[string]any{"a": 1, "c": 3, "d": 4},
		wantMissing: []string{"b"},
		wantLen:     3,
	},
	{
		name:     "update moves entry to front",
		capacity: 2,
		ttl:      time.Hour,
		ops: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
			c.Set("b", 2)
			c.Set("a", 10) // update a; a moves to front, b is now oldest
			c.Set("c", 3)  // should evict b
		},
		want:        map[string]any{"a": 10, "c": 3},
		wantMissing: []string{"b"},
		wantLen:     2,
	},
	{
		name:     "Get refreshes position",
		capacity: 3,
		ttl:      time.Hour,
		ops: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
			c.Set("b", 2)
			c.Set("c", 3)
			c.Get("a")    // a moves to front: a, c, b
			c.Get("b")    // b moves to front: b, a, c
			c.Set("d", 4) // evict c
		},
		want:        map[string]any{"a": 1, "b": 2, "d": 4},
		wantMissing: []string{"c"},
		wantLen:     3,
	},
	{
		name:     "Set evicts expired oldest at capacity",
		capacity: 2,
		ttl:      time.Second,
		ops: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
			c.Set("b", 2)
			clk.advance(2 * time.Second) // both expired
			c.Set("c", 3)                // should evict a (oldest) even though both expired
		},
		want:        map[string]any{"c": 3},
		wantMissing: []string{"a", "b"},
		wantLen:     1,
	},
	{
		name:     "capacity one eviction",
		capacity: 1,
		ttl:      time.Hour,
		ops: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
			c.Set("b", 2)
		},
		want:        map[string]any{"b": 2},
		wantMissing: []string{"a"},
		wantLen:     1,
	},
	{
		name:     "Set prefers expired over live LRU when at capacity",
		capacity: 2,
		ttl:      time.Second,
		ops: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1) // oldest, will expire
			clk.advance(600 * time.Millisecond)
			c.Set("b", 2)                       // newest, live
			c.Get("a")                          // move a to front without refreshing expiry (a now front, b back)
			clk.advance(600 * time.Millisecond) // a expired (1.2s total), b still live (0.6s total)
			// List order: front=a(expired), back=b(live)
			// Set(c) should evict expired a, not live b
			c.Set("c", 3)
		},
		want:        map[string]any{"b": 2, "c": 3},
		wantMissing: []string{"a"},
		wantLen:     2,
	},
}

func TestEviction(t *testing.T) {
	for _, tc := range evictionCases {
		t.Run(tc.name, func(t *testing.T) {
			c := New(tc.capacity, tc.ttl)
			clk := newFakeClock()
			c.SetClock(clk)

			tc.ops(c, clk)

			for k, v := range tc.want {
				if got, ok := c.Get(k); !ok || got != v {
					t.Errorf("%s: Get(%q) = (%v, %v), want (%v, true)", tc.name, k, got, ok, v)
				}
			}
			for _, k := range tc.wantMissing {
				if _, ok := c.Get(k); ok {
					t.Errorf("%s: expected %q to be missing, but it was present", tc.name, k)
				}
			}
			if tc.wantLen >= 0 && c.Len() != tc.wantLen {
				t.Errorf("%s: Len() = %d, want %d", tc.name, c.Len(), tc.wantLen)
			}
		})
	}
}

// expiryTestCase groups TTL expiry scenarios.
type expiryTestCase struct {
	name        string
	capacity    int
	ttl         time.Duration
	setup       func(c *Cache, clk *fakeClock)
	advance     time.Duration
	want        map[string]any // expected present keys and values
	wantMissing []string       // expected missing keys
	wantLen     int            // expected Len() at end (optional, -1 to skip)
}

var expiryCases = []expiryTestCase{
	{
		name:     "entry expires after TTL",
		capacity: 10,
		ttl:      time.Second,
		setup: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
		},
		advance:     1100 * time.Millisecond,
		want:        map[string]any{},
		wantMissing: []string{"a"},
		wantLen:     -1,
	},
	{
		name:     "entry not expired before TTL",
		capacity: 10,
		ttl:      time.Second,
		setup: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
		},
		advance:     500 * time.Millisecond,
		want:        map[string]any{"a": 1},
		wantMissing: []string{},
		wantLen:     -1,
	},
	{
		name:     "exact at TTL boundary means expired",
		capacity: 10,
		ttl:      time.Second,
		setup: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
		},
		advance:     time.Second,
		want:        map[string]any{},
		wantMissing: []string{"a"},
		wantLen:     -1,
	},
	{
		name:     "Set on existing key refreshes TTL",
		capacity: 10,
		ttl:      time.Second,
		setup: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
			clk.advance(900 * time.Millisecond)
			c.Set("a", 2) // refresh expiry
		},
		advance:     500 * time.Millisecond, // 1.4s since first set, 0.5s since refresh
		want:        map[string]any{"a": 2},
		wantMissing: []string{},
		wantLen:     -1,
	},
	{
		name:     "refreshed entry eventually expires",
		capacity: 10,
		ttl:      time.Second,
		setup: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
			clk.advance(900 * time.Millisecond)
			c.Set("a", 2) // refresh expiry
		},
		advance:     1100 * time.Millisecond, // 1.1s since refresh
		want:        map[string]any{},
		wantMissing: []string{"a"},
		wantLen:     -1,
	},
	{
		name:     "lazy expiry removal on Get",
		capacity: 2,
		ttl:      time.Second,
		setup: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
			c.Set("b", 2)
			clk.advance(2 * time.Second) // both expired
		},
		advance:     0, // no additional advance; check after Get("a")
		want:        map[string]any{},
		wantMissing: []string{"a"},
		wantLen:     1, // b still counted until accessed
	},
	{
		name:     "ttl zero means no expiry",
		capacity: 10,
		ttl:      0,
		setup: func(c *Cache, clk *fakeClock) {
			c.Set("a", 1)
		},
		advance:     24 * time.Hour,
		want:        map[string]any{"a": 1},
		wantMissing: []string{},
		wantLen:     -1,
	},
	{
		name:     "negative ttl means no expiry",
		capacity: 10,
		ttl:      -time.Second,
		setup: func(c *Cache, clk *fakeClock) {
			c.Set("b", 2)
		},
		advance:     24 * time.Hour,
		want:        map[string]any{"b": 2},
		wantMissing: []string{},
		wantLen:     -1,
	},
}

func TestExpiry(t *testing.T) {
	for _, tc := range expiryCases {
		t.Run(tc.name, func(t *testing.T) {
			c := New(tc.capacity, tc.ttl)
			clk := newFakeClock()
			c.SetClock(clk)

			tc.setup(c, clk)
			clk.advance(tc.advance)

			for k, v := range tc.want {
				if got, ok := c.Get(k); !ok || got != v {
					t.Errorf("%s: Get(%q) = (%v, %v), want (%v, true)", tc.name, k, got, ok, v)
				}
			}
			for _, k := range tc.wantMissing {
				if _, ok := c.Get(k); ok {
					t.Errorf("%s: expected %q to be missing, but it was present", tc.name, k)
				}
			}
			if tc.wantLen >= 0 && c.Len() != tc.wantLen {
				t.Errorf("%s: Len() = %d, want %d", tc.name, c.Len(), tc.wantLen)
			}
		})
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
	// After all operations, Len should not exceed capacity.
	if c.Len() > c.capacity {
		t.Errorf("Len() = %d, want <= %d", c.Len(), c.capacity)
	}
}
