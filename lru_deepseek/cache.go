// Package lru_deepseek provides a concurrent-safe LRU cache with per-entry TTL.
package lru_deepseek

import (
	"container/list"
	"sync"
	"time"
)

// Clock is an injectable time source for testing.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type entry struct {
	key    string
	value  any
	expiry time.Time
}

// Cache is a concurrent-safe LRU cache with per-entry TTL.
type Cache struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	clock    Clock
	ll       *list.List
	items    map[string]*list.Element
}

// New creates a Cache with the given capacity and TTL.
// capacity is clamped to at least 1.
// ttl <= 0 means entries never expire.
func New(capacity int, ttl time.Duration) *Cache {
	if capacity < 1 {
		capacity = 1
	}
	return &Cache{
		capacity: capacity,
		ttl:      ttl,
		clock:    realClock{},
		ll:       list.New(),
		items:    make(map[string]*list.Element),
	}
}

// SetClock injects a clock for testing.
func (c *Cache) SetClock(clock Clock) {
	c.mu.Lock()
	c.clock = clock
	c.mu.Unlock()
}

// Get returns the value for key if present and not expired.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}
	ent := elem.Value.(*entry)
	if c.ttl > 0 && !c.clock.Now().Before(ent.expiry) {
		c.removeElement(elem)
		return nil, false
	}
	c.ll.MoveToFront(elem)
	return ent.value, true
}

// Set adds or updates the value for key.
func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()
	if elem, ok := c.items[key]; ok {
		ent := elem.Value.(*entry)
		ent.value = value
		if c.ttl > 0 {
			ent.expiry = now.Add(c.ttl)
		}
		c.ll.MoveToFront(elem)
		return
	}

	ent := &entry{key: key, value: value}
	if c.ttl > 0 {
		ent.expiry = now.Add(c.ttl)
	}
	elem := c.ll.PushFront(ent)
	c.items[key] = elem

	// When over capacity, sweep for an expired entry to evict first.
	// This scans from the back (oldest) toward the front and removes
	// the first expired entry found. If no expired entry exists,
	// removeOldest evicts the LRU live entry.
	if c.ll.Len() > c.capacity {
		c.sweepExpired(now)
		if c.ll.Len() > c.capacity {
			c.removeOldest()
		}
	}
}

// sweepExpired removes the oldest expired entry from the list if one exists,
// preferring to evict expired entries over live ones when at capacity.
// It scans from the back (oldest) toward the front and removes the first
// expired entry found. If no expired entry exists, it does nothing and
// the caller (Set) will evict the LRU live entry via removeOldest.
func (c *Cache) sweepExpired(now time.Time) {
	if c.ttl <= 0 {
		return
	}
	for elem := c.ll.Back(); elem != nil; elem = elem.Prev() {
		ent := elem.Value.(*entry)
		if now.Before(ent.expiry) {
			continue // this entry is still live, check older ones
		}
		// Found an expired entry; remove it and stop.
		c.removeElement(elem)
		return
	}
}

// Len returns the number of entries in the cache, including expired ones
// that haven't been evicted by a Get/Set yet.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}

func (c *Cache) removeOldest() {
	elem := c.ll.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

func (c *Cache) removeElement(elem *list.Element) {
	c.ll.Remove(elem)
	ent := elem.Value.(*entry)
	delete(c.items, ent.key)
}
