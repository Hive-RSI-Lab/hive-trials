package lru

import (
	"sync"
	"time"
)

// Clock is an interface for getting the current time.
// This allows injecting a fake clock for testing.
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using the real time.
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

// Cache is a thread-safe LRU cache with TTL support.
type Cache struct {
	mu       sync.RWMutex
	capacity int
	ttl      time.Duration
	clock    Clock
	// doubly linked list for LRU ordering
	head *node
	tail *node
	// map for O(1) lookup
	items map[string]*node
}

type node struct {
	key       string
	value     any
	expiresAt time.Time
	prev      *node
	next      *node
}

// New creates a new LRU cache with the given capacity and TTL.
// If capacity <= 0, there is no capacity limit.
// If ttl <= 0, entries do not expire.
func New(capacity int, ttl time.Duration) *Cache {
	c := &Cache{
		capacity: capacity,
		ttl:      ttl,
		clock:    RealClock{},
		items:    make(map[string]*node),
	}
	// Initialize sentinel nodes for the doubly linked list
	c.head = &node{}
	c.tail = &node{}
	c.head.next = c.tail
	c.tail.prev = c.head
	return c
}

// setClockForTest sets the clock for testing purposes.
// This is an unexported method used by in-package tests.
func (c *Cache) setClockForTest(clock Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock = clock
}

// Get retrieves a value from the cache.
// Returns the value and true if found and not expired, otherwise nil and false.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// Check expiration
	if c.ttl > 0 && c.clock.Now().After(n.expiresAt) {
		c.removeNode(n)
		delete(c.items, key)
		return nil, false
	}

	// Move to front (most recently used)
	c.moveToFront(n)
	return n.value, true
}

// Set adds or updates a value in the cache.
func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()

	if n, ok := c.items[key]; ok {
		// Update existing entry
		n.value = value
		if c.ttl > 0 {
			n.expiresAt = now.Add(c.ttl)
		}
		c.moveToFront(n)
		return
	}

	// Create new entry
	n := &node{
		key:       key,
		value:     value,
		expiresAt: now.Add(c.ttl),
	}

	// Add to front of list
	c.addToFront(n)
	c.items[key] = n

	// Evict if over capacity
	if c.capacity > 0 && len(c.items) > c.capacity {
		c.evictLRUOrExpired(now)
	}
}

// Len returns the number of items in the cache (including expired entries
// that have not yet been accessed or evicted).
// Expired entries remain in the cache until they are accessed (via Get)
// or until they become the least-recently-used entry and are evicted
// during a Set operation when the cache is at capacity.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// evictLRU removes the least recently used item (the one before tail).
func (c *Cache) evictLRU() {
	lru := c.tail.prev
	if lru == c.head {
		return // empty
	}
	c.removeNode(lru)
	delete(c.items, lru.key)
}

// evictLRUOrExpired first removes all expired entries, then evicts the LRU
// if still over capacity.
func (c *Cache) evictLRUOrExpired(now time.Time) {
	// First, remove expired entries
	if c.ttl > 0 {
		for n := c.tail.prev; n != c.head; {
			prev := n.prev
			if now.After(n.expiresAt) {
				c.removeNode(n)
				delete(c.items, n.key)
			}
			n = prev
		}
	}

	// If still over capacity, evict LRU
	if c.capacity > 0 && len(c.items) > c.capacity {
		c.evictLRU()
	}
}

// addToFront adds a node right after head (most recently used position).
func (c *Cache) addToFront(n *node) {
	n.prev = c.head
	n.next = c.head.next
	c.head.next.prev = n
	c.head.next = n
}

// removeNode removes a node from the doubly linked list.
func (c *Cache) removeNode(n *node) {
	n.prev.next = n.next
	n.next.prev = n.prev
	n.prev = nil
	n.next = nil
}

// moveToFront moves an existing node to the front (most recently used).
func (c *Cache) moveToFront(n *node) {
	c.removeNode(n)
	c.addToFront(n)
}
