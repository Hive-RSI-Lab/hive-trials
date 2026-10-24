package lru

import (
	"sync"
	"testing"
	"time"
)

// fakeClock is a controllable clock for testing.
type fakeClock struct {
	mu   sync.Mutex
	now  time.Time
	ch   chan time.Time
	done chan struct{}
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{
		now:  start,
		ch:   make(chan time.Time, 1),
		done: make(chan struct{}),
	}
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) After(d time.Duration) <-chan time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	target := f.now.Add(d)
	select {
	case f.ch <- target:
	case <-f.done:
	}
	return f.ch
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
	select {
	case f.ch <- f.now:
	default:
	}
}

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
	c := New(3, time.Hour)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Access 'a' to make it most recently used
	c.Get("a")

	// Add 'd', should evict 'b' (least recently used)
	c.Set("d", 4)

	if c.Len() != 3 {
		t.Errorf("expected Len() == 3, got %d", c.Len())
	}

	// 'b' should be evicted
	_, ok := c.Get("b")
	if ok {
		t.Error("'b' should have been evicted")
	}

	// 'a', 'c', 'd' should exist
	for _, key := range []string{"a", "c", "d"} {
		_, ok := c.Get(key)
		if !ok {
			t.Errorf("'%s' should exist", key)
		}
	}
}

func TestLRUEvictionOrder(t *testing.T) {
	c := New(3, time.Hour)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Order should be: c (MRU), b, a (LRU)
	// Add d, evicts a
	c.Set("d", 4)

	_, ok := c.Get("a")
	if ok {
		t.Error("'a' should be evicted first")
	}

	// Now order: d (MRU), c, b (LRU)
	// Add e, evicts b
	c.Set("e", 5)

	_, ok = c.Get("b")
	if ok {
		t.Error("'b' should be evicted second")
	}

	// c, d, e should remain
	for _, key := range []string{"c", "d", "e"} {
		_, ok := c.Get(key)
		if !ok {
			t.Errorf("'%s' should exist", key)
		}
	}
}

func TestUpdateExistingKey(t *testing.T) {
	c := New(3, time.Hour)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Update 'a' - should move to front
	c.Set("a", 100)

	// Add 'd', should evict 'b' (now LRU)
	c.Set("d", 4)

	_, ok := c.Get("b")
	if ok {
		t.Error("'b' should be evicted after 'a' update")
	}

	val, ok := c.Get("a")
	if !ok || val != 100 {
		t.Errorf("'a' should have updated value 100, got %v, ok=%v", val, ok)
	}
}

func TestTTLExpiration(t *testing.T) {
	clock := newFakeClock(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	c := New(10, time.Hour, clock)

	c.Set("key1", "value1")

	// Before expiry
	val, ok := c.Get("key1")
	if !ok || val != "value1" {
		t.Errorf("before expiry: expected value1, got %v, ok=%v", val, ok)
	}

	// Advance past TTL
	clock.Advance(time.Hour + time.Minute)

	// After expiry
	_, ok = c.Get("key1")
	if ok {
		t.Error("key1 should be expired after TTL")
	}
}

func TestTTLNoExpirationWhenZero(t *testing.T) {
	clock := newFakeClock(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	c := New(10, 0, clock) // TTL = 0 means no expiration

	c.Set("key1", "value1")

	// Advance far past any reasonable TTL
	clock.Advance(24 * time.Hour)

	val, ok := c.Get("key1")
	if !ok || val != "value1" {
		t.Errorf("with TTL=0: expected value1, got %v, ok=%v", val, ok)
	}
}

func TestTTLUpdateResetsExpiry(t *testing.T) {
	clock := newFakeClock(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	c := New(10, time.Hour, clock)

	c.Set("key1", "value1")

	// Advance 30 minutes
	clock.Advance(30 * time.Minute)

	// Update the key - should reset expiry
	c.Set("key1", "value2")

	// Advance another 45 minutes (total 75 min, still within TTL from update)
	clock.Advance(45 * time.Minute)

	val, ok := c.Get("key1")
	if !ok || val != "value2" {
		t.Errorf("after update and 75 min: expected value2, got %v, ok=%v", val, ok)
	}

	// Advance another 30 minutes (total 105 min from update, past TTL)
	clock.Advance(30 * time.Minute)

	_, ok = c.Get("key1")
	if ok {
		t.Error("key1 should be expired after TTL from last update")
	}
}

func TestTTLGetDoesNotExtendExpiry(t *testing.T) {
	clock := newFakeClock(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	c := New(10, time.Hour, clock)

	c.Set("key1", "value1")

	// Advance 30 minutes
	clock.Advance(30 * time.Minute)

	// Get the key - should NOT reset expiry
	_, ok := c.Get("key1")
	if !ok {
		t.Fatal("key1 should still be valid after 30 min")
	}

	// Advance another 20 minutes (total 50 min) - still within TTL
	clock.Advance(20 * time.Minute)

	// Still within TTL (50 min < 60 min)
	_, ok = c.Get("key1")
	if !ok {
		t.Error("key1 should still be valid at 50 min")
	}

	// Advance another 20 minutes (total 70 min) - past TTL
	clock.Advance(20 * time.Minute)

	// Now expired
	_, ok = c.Get("key1")
	if ok {
		t.Error("key1 should be expired at 70 min")
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New(100, time.Hour)
	var wg sync.WaitGroup
	const numGoroutines = 10
	const numOps = 1000

	// Writers
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

	// Readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				c.Get("key")
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
	clock := newFakeClock(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	c := New(3, time.Hour, clock)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Expire 'a' and 'b' but not 'c' (advance 50 min, not full hour)
	clock.Advance(50 * time.Minute)

	// Add 'd' - should not evict anything since we're at capacity but 'c' is not expired
	// and 'a' and 'b' are not expired yet either
	c.Set("d", 4)

	// All four should exist (capacity is 3, but we only added 3 before, then 1 more = 4, so LRU 'a' evicted)
	if c.Len() != 3 {
		t.Errorf("expected Len() == 3, got %d", c.Len())
	}

	// 'a' should be evicted (LRU)
	_, ok := c.Get("a")
	if ok {
		t.Error("'a' should be evicted as LRU")
	}

	// 'b', 'c', 'd' should exist
	for _, key := range []string{"b", "c", "d"} {
		_, ok := c.Get(key)
		if !ok {
			t.Errorf("'%s' should exist", key)
		}
	}

	// Now expire 'b' and 'c'
	clock.Advance(20 * time.Minute) // total 70 min, past TTL

	// Add 'e' - should clean up expired entries first
	c.Set("e", 5)

	// 'b' and 'c' should be gone (expired)
	_, ok = c.Get("b")
	if ok {
		t.Error("expired 'b' should be gone")
	}
	_, ok = c.Get("c")
	if ok {
		t.Error("expired 'c' should be gone")
	}

	// 'd' and 'e' should exist
	for _, key := range []string{"d", "e"} {
		_, ok := c.Get(key)
		if !ok {
			t.Errorf("'%s' should exist", key)
		}
	}
}