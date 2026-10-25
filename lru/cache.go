package lru

import (
	"container/list"
	"sync"
	"time"
)

// Clock is an interface for time operations, allowing injection for testing.
type Clock interface {
	Now() time.Time
}

// RealClock is the default implementation using time.Time.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// entry holds a key-value pair with expiration time.
type entry struct {
	key       string
	value     any
	expiresAt time.Time
}

// Cache is a thread-safe LRU cache with TTL expiration.
type Cache struct {
	capacity int
	ttl      time.Duration
	clock    Clock

	mu    sync.RWMutex
	list  *list.List
	index map[string]*list.Element
}

// New creates a new LRU cache with the given capacity and TTL.
// If ttl <= 0, entries never expire.
// The clock parameter allows injecting a fake clock for testing.
func New(capacity int, ttl time.Duration, clock ...Clock) *Cache {
	c := &Cache{
		capacity: capacity,
		ttl:      ttl,
		list:     list.New(),
		index:    make(map[string]*list.Element),
	}
	if len(clock) > 0 && clock[0] != nil {
		c.clock = clock[0]
	} else {
		c.clock = RealClock{}
	}
	return c
}

// Get retrieves a value from the cache.
// Returns the value and true if found and not expired, otherwise nil and false.
func (c *Cache) Get(key string) (value any, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, found := c.index[key]
	if !found {
		return nil, false
	}

	ent := elem.Value.(*entry)
	if c.ttl > 0 && c.clock.Now().After(ent.expiresAt) {
		c.removeElement(elem)
		return nil, false
	}

	// Move to front (most recently used)
	c.list.MoveToFront(elem)
	return ent.value, true
}

// Set adds or updates a value in the cache.
// If the key exists, it updates the value and moves to front.
// If at capacity, evicts expired entries first, then the least recently used entry.
func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, found := c.index[key]; found {
		// Update existing entry
		ent := elem.Value.(*entry)
		ent.value = value
		if c.ttl > 0 {
			ent.expiresAt = c.clock.Now().Add(c.ttl)
		}
		c.list.MoveToFront(elem)
		return
	}

	// Add new entry
	ent := &entry{
		key:   key,
		value: value,
	}
	if c.ttl > 0 {
		ent.expiresAt = c.clock.Now().Add(c.ttl)
	}

	elem := c.list.PushFront(ent)
	c.index[key] = elem

	// Evict if over capacity (clean up expired first, then LRU)
	if c.list.Len() > c.capacity {
		c.evictIfNeededLocked()
	}
}

// evictIfNeededLocked removes expired entries first, then LRU if still over capacity.
// Must be called with c.mu held.
func (c *Cache) evictIfNeededLocked() {
	// First, remove expired entries
	if c.ttl > 0 {
		now := c.clock.Now()
		for elem := c.list.Back(); elem != nil; {
			ent := elem.Value.(*entry)
			next := elem.Prev()
			if now.After(ent.expiresAt) {
				c.removeElement(elem)
			}
			elem = next
		}
	}

	// If still over capacity, evict LRU
	if c.list.Len() > c.capacity {
		c.evictLRU()
	}
}

// Len returns the number of entries in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.list.Len()
}

// evictLRU removes the least recently used entry (back of the list).
func (c *Cache) evictLRU() {
	elem := c.list.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

// removeElement removes an element from both the list and the index.
func (c *Cache) removeElement(elem *list.Element) {
	ent := elem.Value.(*entry)
	delete(c.index, ent.key)
	c.list.Remove(elem)
}