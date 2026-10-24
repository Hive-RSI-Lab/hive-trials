package lru

import (
    "container/list"
    "sync"
    "time"
)

// Clock is a function returning the current time. It can be overridden in tests
// to control time flow.
type Clock func() time.Time

// Cache implements an LRU (least-recently-used) cache with a TTL (time‑to‑live).
// It is safe for concurrent use.
//
// New creates a cache that can hold at most capacity entries. When the cache is
// full, the least‑recently‑used entry is evicted. Each entry expires ttl after
// being set; a zero ttl disables expiry.
type Cache struct {
    capacity int
    ttl      time.Duration
    now      Clock

    mu    sync.RWMutex
    ll    *list.List               // *entry, front = most recent
    items map[string]*entry        // key -> entry
}

type entry struct {
    key        string
    value      any
    expiration time.Time
    element    *list.Element // back‑reference for O(1) removal
}

// New returns a new Cache.
func New(capacity int, ttl time.Duration) *Cache {
    if capacity <= 0 {
        capacity = 0 // treat as unlimited, but LRU still works
    }
    return &Cache{
        capacity: capacity,
        ttl:      ttl,
        now:      time.Now,
        ll:       list.New(),
        items:    make(map[string]*entry),
    }
}

// Set inserts or updates the value associated with key. The entry's expiration
// time is reset to now+ttl. If the cache exceeds its capacity the least‑recently
// used entry is evicted.
func (c *Cache) Set(key string, value any) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if e, ok := c.items[key]; ok {
        // Update existing entry.
        e.value = value
        if c.ttl > 0 {
            e.expiration = c.now().Add(c.ttl)
        }
        c.ll.MoveToFront(e.element)
        return
    }

    // New entry.
    e := &entry{key: key, value: value}
    if c.ttl > 0 {
        e.expiration = c.now().Add(c.ttl)
    }
    elem := c.ll.PushFront(e)
    e.element = elem
    c.items[key] = e

    // Enforce capacity.
    if c.capacity > 0 && c.ll.Len() > c.capacity {
        c.evictOldest()
    }
}

// Get returns the value for the given key, or (nil, false) if the key is not
// present or the entry has expired. A successful Get marks the entry as most
// recently used.
func (c *Cache) Get(key string) (any, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()

    e, ok := c.items[key]
    if !ok {
        return nil, false
    }
    if c.ttl > 0 && c.now().After(e.expiration) {
        c.removeElement(e.element)
        return nil, false
    }
    // Move to front to mark as recently used.
    c.ll.MoveToFront(e.element)
    return e.value, true
}

// Len returns the number of items currently stored in the cache (including any
// that may be expired but not yet accessed). It is primarily useful for tests.
func (c *Cache) Len() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.ll.Len()
}

// evictOldest removes the least‑recently‑used item from the cache.
func (c *Cache) evictOldest() {
    if back := c.ll.Back(); back != nil {
        c.removeElement(back)
    }
}

// removeElement removes the list element and updates the map.
func (c *Cache) removeElement(el *list.Element) {
    if el == nil {
        return
    }
    e := el.Value.(*entry)
    delete(c.items, e.key)
    c.ll.Remove(el)
}

// SetClock overrides the clock used to compute expiration. This is intended for
// tests only.
func (c *Cache) SetClock(clock Clock) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.now = clock
}
