package lru

import (
	"sync"
	"time"
)

// Cache is a thread-safe LRU cache with optional TTL.
type Cache struct {
	mu       sync.RWMutex
	capacity int
	ttl      time.Duration
	items    map[string]*node
	head     *node // dummy head (most recent)
	tail     *node // dummy tail (least recent)
	clock    Clock
}

// Clock is an interface for getting the current time.
type Clock interface {
	Now() time.Time
}

// realClock implements Clock using time.Now.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

var _ Clock = realClock{}

type node struct {
	key       string
	value     any
	expiresAt time.Time // zero means no expiry
	prev, next *node
}

// New creates a new LRU cache.
// capacity <= 0 means no limit.
// ttl <= 0 means no expiry.
func New(capacity int, ttl time.Duration) *Cache {
	c := &Cache{
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[string]*node),
		clock:    realClock{},
	}
	// Initialize doubly-linked list with dummy head and tail
	c.head = &node{}
	c.tail = &node{}
	c.head.next = c.tail
	c.tail.prev = c.head
	return c
}

// setClockForTest allows tests to inject a fake clock.
func (c *Cache) setClockForTest(clk Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock = clk
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Get retrieves a value from the cache.
// Returns the value and true if found and not expired, otherwise nil and false.
// Access updates recency (moves to front).
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// Check expiry
	if n.expiresAt != (time.Time{}) && !c.clock.Now().Before(n.expiresAt) {
		c.removeNode(n)
		delete(c.items, key)
		return nil, false
	}

	// Move to front (most recent)
	c.moveToFront(n)
	return n.value, true
}

// Set adds or updates a value in the cache.
// If the key exists, it updates the value and resets TTL.
// If at capacity, evicts the least recently used item (or expired items first).
func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()

	// If key exists, update it
	if n, ok := c.items[key]; ok {
		n.value = value
		if c.ttl > 0 {
			n.expiresAt = now.Add(c.ttl)
		} else {
			n.expiresAt = time.Time{}
		}
		c.moveToFront(n)
		return
	}

	// Evict expired entries first if at capacity
	if c.capacity > 0 && len(c.items) >= c.capacity {
		c.evictExpired(now)
	}

	// If still at capacity, evict LRU
	if c.capacity > 0 && len(c.items) >= c.capacity {
		c.evictLRU()
	}

	// Create new node
	var expiresAt time.Time
	if c.ttl > 0 {
		expiresAt = now.Add(c.ttl)
	}
	n := &node{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}

	// Add to front of list
	c.addToFront(n)
	c.items[key] = n
}

// addToFront adds a node right after head (most recent position).
func (c *Cache) addToFront(n *node) {
	n.prev = c.head
	n.next = c.head.next
	c.head.next.prev = n
	c.head.next = n
}

// moveToFront moves an existing node to the front (most recent).
func (c *Cache) moveToFront(n *node) {
	c.removeNode(n)
	c.addToFront(n)
}

// removeNode removes a node from the linked list.
func (c *Cache) removeNode(n *node) {
	n.prev.next = n.next
	n.next.prev = n.prev
}

// evictExpired removes all expired entries.
func (c *Cache) evictExpired(now time.Time) {
	for key, n := range c.items {
		if n.expiresAt != (time.Time{}) && !now.Before(n.expiresAt) {
			c.removeNode(n)
			delete(c.items, key)
		}
	}
}

// evictLRU removes the least recently used item (node before tail).
func (c *Cache) evictLRU() {
	lru := c.tail.prev
	if lru == c.head {
		return // empty
	}
	c.removeNode(lru)
	delete(c.items, lru.key)
}