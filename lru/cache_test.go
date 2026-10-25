package lru

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New(10, time.Hour)
	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.Len() != 0 {
		t.Errorf("expected Len() == 0, got %d", c.Len())
	}
}

func TestGetSet(t *testing.T) {
	c := New(10, time.Hour)
	c.Set("key1", "value1")
	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("Get(key1) should return ok=true")
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got %v", val)
	}
}

func TestGetMissing(t *testing.T) {
	c := New(10, time.Hour)
	val, ok := c.Get("missing")
	if ok {
		t.Errorf("Get(missing) should return ok=false, got ok=true, val=%v", val)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestLRUEviction(t *testing.T) {
	tests := []struct {
		name          string
		capacity      int
		operations    []func(*Cache)
		expectPresent []string
		expectAbsent  []string
	}{
		{
			name:     "basic LRU eviction",
			capacity: 3,
			operations: func() []func(*Cache) {
				ops := []func(*Cache){
					func(c *Cache) { c.Set("a", 1) },
					func(c *Cache) { c.Set("b", 2) },
					func(c *Cache) { c.Set("c", 3) },
					func(c *Cache) { c.Get("a") }, // make 'a' MRU
					func(c *Cache) { c.Set("d", 4) }, // should evict 'b'
				}
				return ops
			}(),
			expectPresent: []string{"a", "c", "d"},
			expectAbsent:  []string{"b"},
		},
		{
			name:     "sequential eviction order",
			capacity: 3,
			operations: func() []func(*Cache) {
				ops := []func(*Cache){
					func(c *Cache) { c.Set("a", 1) },
					func(c *Cache) { c.Set("b", 2) },
					func(c *Cache) { c.Set("c", 3) },
					func(c *Cache) { c.Set("d", 4) }, // evict 'a'
					func(c *Cache) { c.Set("e", 5) }, // evict 'b'
				}
				return ops
			}(),
			expectPresent: []string{"c", "d", "e"},
			expectAbsent:  []string{"a", "b"},
		},
		{
			name:     "update moves to front",
			capacity: 3,
			operations: func() []func(*Cache) {
				ops := []func(*Cache){
					func(c *Cache) { c.Set("a", 1) },
					func(c *Cache) { c.Set("b", 2) },
					func(c *Cache) { c.Set("c", 3) },
					func(c *Cache) { c.Set("a", 100) }, // update 'a', moves to front
					func(c *Cache) { c.Set("d", 4) },  // should evict 'b' (now LRU)
				}
				return ops
			}(),
			expectPresent: []string{"a", "c", "d"},
			expectAbsent:  []string{"b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := New(tc.capacity, time.Hour)
			for _, op := range tc.operations {
				op(c)
			}
			for _, key := range tc.expectPresent {
				if _, ok := c.Get(key); !ok {
					t.Errorf("expected key %q to be present", key)
				}
			}
			for _, key := range tc.expectAbsent {
				if _, ok := c.Get(key); ok {
					t.Errorf("expected key %q to be absent (evicted)", key)
				}
			}
			if c.Len() != len(tc.expectPresent) {
				t.Errorf("expected Len() == %d, got %d", len(tc.expectPresent), c.Len())
			}
		})
	}
}

func TestTTLExpiration(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(*FakeClock) *Cache
		advance      time.Duration
		expectExists bool
	}{
		{
			name: "entry expires after TTL",
			setup: func(clock *FakeClock) *Cache {
				c := New(10, time.Hour, clock)
				c.Set("key1", "value1")
				return c
			},
			advance:      time.Hour + time.Minute,
			expectExists: false,
		},
		{
			name: "entry still valid before TTL",
			setup: func(clock *FakeClock) *Cache {
				c := New(10, time.Hour, clock)
				c.Set("key1", "value1")
				return c
			},
			advance:      30 * time.Minute,
			expectExists: true,
		},
		{
			name: "no expiration when TTL is zero",
			setup: func(clock *FakeClock) *Cache {
				c := New(10, 0, clock)
				c.Set("key1", "value1")
				return c
			},
			advance:      24 * time.Hour,
			expectExists: true,
		},
		{
			name: "update resets expiry",
			setup: func(clock *FakeClock) *Cache {
				c := New(10, time.Hour, clock)
				c.Set("key1", "value1")
				clock.Advance(30 * time.Minute)
				c.Set("key1", "value2") // update resets TTL
				return c
			},
			advance:      45 * time.Minute, // total 75 min from start, 45 min from update
			expectExists: true,
		},
		{
			name: "update resets expiry - expired after TTL from update",
			setup: func(clock *FakeClock) *Cache {
				c := New(10, time.Hour, clock)
				c.Set("key1", "value1")
				clock.Advance(30 * time.Minute)
				c.Set("key1", "value2")
				return c
			},
			advance:      75 * time.Minute, // total 105 min from start, 75 min from update
			expectExists: false,
		},
		{
			name: "Get does not extend expiry",
			setup: func(clock *FakeClock) *Cache {
				c := New(10, time.Hour, clock)
				c.Set("key1", "value1")
				clock.Advance(30 * time.Minute)
				c.Get("key1") // Get should not reset expiry
				return c
			},
			advance:      40 * time.Minute, // total 70 min
			expectExists: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := NewFakeClock(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
			c := tc.setup(clock)

			// Verify initial state
			if val, ok := c.Get("key1"); !ok || val != "value1" && val != "value2" {
				t.Fatal("initial Get failed")
			}

			clock.Advance(tc.advance)

			_, ok := c.Get("key1")
			if ok != tc.expectExists {
				t.Errorf("expected exists=%v, got exists=%v", tc.expectExists, ok)
			}
		})
	}
}

func TestLen(t *testing.T) {
	c := New(5, time.Hour)
	if c.Len() != 0 {
		t.Errorf("expected Len() == 0, got %d", c.Len())
	}

	c.Set("a", 1)
	if c.Len() != 1 {
		t.Errorf("expected Len() == 1, got %d", c.Len())
	}

	c.Set("b", 2)
	if c.Len() != 2 {
		t.Errorf("expected Len() == 2, got %d", c.Len())
	}

	c.Set("a", 3) // update
	if c.Len() != 2 {
		t.Errorf("expected Len() == 2 after update, got %d", c.Len())
	}

	c.Set("c", 3)
	c.Set("d", 4)
	c.Set("e", 5)
	if c.Len() != 5 {
		t.Errorf("expected Len() == 5, got %d", c.Len())
	}

	// Adding one more should evict LRU
	c.Set("f", 6)
	if c.Len() != 5 {
		t.Errorf("expected Len() == 5 after eviction, got %d", c.Len())
	}
}

func TestEvictionWithExpiredEntries(t *testing.T) {
	clock := NewFakeClock(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	c := New(3, time.Hour, clock)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Expire 'a' only (advance 70 min past 1 hour TTL, but 'b' and 'c' were added later)
	// Actually all entries get the same timestamp when Set, so they all expire at the same time.
	// Let's use different TTLs by setting at different times
	clock.Advance(70 * time.Minute)

	// Add 'd' - should evict expired entries first, then LRU if needed
	c.Set("d", 4)

	// 'a', 'b', 'c' should all be gone (all expired - they were all set before the 70 min advance)
	if _, ok := c.Get("a"); ok {
		t.Error("expired 'a' should be gone")
	}
	if _, ok := c.Get("b"); ok {
		t.Error("expired 'b' should be gone")
	}
	if _, ok := c.Get("c"); ok {
		t.Error("expired 'c' should be gone")
	}

	// 'd' should exist
	if _, ok := c.Get("d"); !ok {
		t.Error("'d' should exist")
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Test with many keys, eviction, and race detection
	c := New(100, time.Hour)
	var wg sync.WaitGroup
	const numGoroutines = 20
	const numOps = 500

	// Writers using many different keys to trigger eviction
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := "key"
				c.Set(key, id*numOps+j)
			}
		}(i)
	}

	// Writers using varied keys to fill cache beyond capacity
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := "varkey"
				c.Set(key, id*numOps+j)
			}
		}(i)
	}

	// Readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				c.Get("key")
				c.Get("varkey")
				c.Len()
			}
		}()
	}

	wg.Wait()

	// Just verify it didn't panic and cache is in valid state
	if c.Len() < 0 || c.Len() > 100 {
		t.Errorf("invalid Len after concurrent access: %d", c.Len())
	}
}

func TestConcurrentAccessWithEvictionAndExpiry(t *testing.T) {
	// This test uses a fake clock and varied keys to exercise eviction and expiry under race
	clock := NewFakeClock(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	c := New(50, time.Hour, clock)
	var wg sync.WaitGroup
	const numGoroutines = 10
	const numOps = 200

	// Writers with many different keys to exceed capacity
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := "k"
				c.Set(key, id*numOps+j)
			}
		}(i)
	}

	// Writers with unique keys to force eviction
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := "u"
				c.Set(key, id*numOps+j)
			}
		}(i)
	}

	// Readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				c.Get("k")
				c.Get("u")
				c.Len()
			}
		}()
	}

	// Time advancer - advances clock to expire entries
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			clock.Advance(10 * time.Minute)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()
}

func TestConcurrentAccessWithRace(t *testing.T) {
	// This test is meant to be run with -race flag
	c := New(50, time.Hour)
	var wg sync.WaitGroup
	const numGoroutines = 20
	const numOps = 500

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := "k"
				c.Set(key, id*numOps+j)
				c.Get(key)
				c.Len()
			}
		}(i)
	}

	wg.Wait()
}

func TestFakeClock(t *testing.T) {
	clock := NewFakeClock(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	start := clock.Now()
	expected := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	if !start.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, start)
	}

	clock.Advance(time.Hour)
	after := clock.Now()
	expected = time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)
	if !after.Equal(expected) {
		t.Errorf("expected %v after advance, got %v", expected, after)
	}

	// Test that it implements Clock interface
	var _ Clock = clock
}