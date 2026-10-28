// Package lru is a thread-safe LRU cache with per-entry TTL support.
//
// It uses a doubly-linked list for O(1) LRU ordering and a map for O(1)
// lookup. Expired entries are removed lazily: on Get, and on Set when the
// cache is at capacity.
package lru

import (
	"sync"
	"time"
)

// Clock is a source of time, used by the cache to decide whether entries have
// expired. The acceptance tests swap in a fake clock via setClockForTest.
type Clock interface {
	Now() time.Time
}

// defaultClock is a real-time Clock.
type defaultClock struct{}

func (defaultClock) Now() time.Time { return time.Now() }

// node is an entry in the cache and a node in the doubly-linked list. A zero
// expiry means the entry never expires (only set when ttl > 0).
type node struct {
	key    string
	value  any
	expiry time.Time
	prev   *node
	next   *node
}

// Cache is a thread-safe LRU cache with TTL support.
//
//   - capacity <= 0 means no capacity limit
//   - ttl <= 0 means entries do not expire
type Cache struct {
	mu       sync.RWMutex
	items    map[string]*node
	head     *node // sentinel before the most-recently-used entry
	tail     *node // sentinel after the least-recently-used entry
	capacity int
	ttl      time.Duration
	clock    Clock
}

// New returns a Cache with the given capacity and TTL.
func New(capacity int, ttl time.Duration) *Cache {
	c := &Cache{
		items:    make(map[string]*node),
		capacity: capacity,
		ttl:      ttl,
		clock:    defaultClock{},
	}
	c.head = &node{}
	c.tail = &node{}
	c.head.next = c.tail
	c.tail.prev = c.head
	return c
}

// setClockForTest swaps in a controllable clock. It is used by the acceptance
// tests to exercise TTL behaviour without sleeping.
func (c *Cache) setClockForTest(clock Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock = clock
}

// addToFront inserts n right after the head sentinel: most-recently-used.
func (c *Cache) addToFront(n *node) {
	n.prev = c.head
	n.next = c.head.next
	c.head.next.prev = n
	c.head.next = n
}

// removeNode detaches n from the list.
func (c *Cache) removeNode(n *node) {
	n.prev.next = n.next
	n.next.prev = n.prev
	n.prev = nil
	n.next = nil
}

// moveToFront moves an existing node to the front (most-recently-used).
func (c *Cache) moveToFront(n *node) {
	c.removeNode(n)
	c.addToFront(n)
}

// expired reports whether n has passed its expiry time.
func (c *Cache) expired(n *node) bool {
	if n.expiry.IsZero() {
		return false
	}
	return c.clock.Now().After(n.expiry)
}

// removeExpired walks the list and removes every expired entry, freeing
// capacity for new entries before any LRU eviction.
func (c *Cache) removeExpired() {
	n := c.tail.prev
	for n != c.head {
		prev := n.prev
		if c.expired(n) {
			c.removeNode(n)
			delete(c.items, n.key)
		}
		n = prev
	}
}

// evict removes expired entries first, then evicts least-recently-used
// entries until the cache is within capacity.
func (c *Cache) evict() {
	c.removeExpired()
	for c.capacity > 0 && len(c.items) > c.capacity {
		n := c.tail.prev // least-recently-used
		c.removeNode(n)
		delete(c.items, n.key)
	}
}

// Get returns the value for key, moving the entry to the front
// (most-recently-used). It returns false if the key is absent or expired.
// Expired entries are removed from the cache as a side effect.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if c.expired(n) {
		c.removeNode(n)
		delete(c.items, n.key)
		return nil, false
	}
	c.moveToFront(n)
	return n.value, true
}

// Set adds or updates key, moving it to the front (most-recently-used). On an
// existing key it resets the TTL. When at capacity it evicts expired entries
// first, then least-recently-used entries.
func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n, ok := c.items[key]; ok {
		c.removeNode(n)
		delete(c.items, key)
	}

	n := &node{key: key, value: value}
	if c.ttl > 0 {
		n.expiry = c.clock.Now().Add(c.ttl)
	}
	c.items[key] = n
	c.addToFront(n)

	c.evict()
}

// Len returns the number of entries currently in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}
