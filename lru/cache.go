package lru

import (
    "container/list"
    "sync"
    "time"
)

type Clock interface {
    Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Cache is a concurrency‑safe LRU cache with a TTL for each entry.
// The zero value is not usable; use New to create one.
type Cache struct {
    mu       sync.RWMutex
    capacity int
    ttl      time.Duration
    clock    Clock
    // map key to list element
    items map[string]*list.Element
    // order: front = most recently used, back = least recently used
    order *list.List
}

type entry struct {
    key       string
    value     any
    expiresAt time.Time
}

// New creates a new cache with the given capacity and TTL.
// It uses the real time clock.
func New(capacity int, ttl time.Duration) *Cache {
    if capacity <= 0 {
        panic("capacity must be > 0")
    }
    return &Cache{
        capacity: capacity,
        ttl:      ttl,
        clock:    realClock{},
        items:    make(map[string]*list.Element),
        order:    list.New(),
    }
}

// Get returns the value associated with the key, if present and not expired.
func (c *Cache) Get(key string) (any, bool) {
    // Use write lock for simplicity: we may need to delete expired entries and update LRU order.
    c.mu.Lock()
    defer c.mu.Unlock()
    el, ok := c.items[key]
    if !ok {
        return nil, false
    }
    ent := el.Value.(*entry)
    if c.clock.Now().After(ent.expiresAt) {
        c.removeElement(el)
        return nil, false
    }
    // Move to front (most recent)
    c.order.MoveToFront(el)
    return ent.value, true
}

// Set stores the value under key, updating expiration and LRU order.
func (c *Cache) Set(key string, v any) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if el, ok := c.items[key]; ok {
        // update existing entry
        ent := el.Value.(*entry)
        ent.value = v
        ent.expiresAt = c.clock.Now().Add(c.ttl)
        c.order.MoveToFront(el)
        return
    }
    // new entry
    if c.order.Len() >= c.capacity {
        // evict least recently used (back)
        back := c.order.Back()
        if back != nil {
            c.removeElement(back)
        }
    }
    ent := &entry{key: key, value: v, expiresAt: c.clock.Now().Add(c.ttl)}
    el := c.order.PushFront(ent)
    c.items[key] = el
}

// Len returns the number of non‑expired items currently stored.
func (c *Cache) Len() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    // Count items that are not expired.
    count := 0
    now := c.clock.Now()
    for _, el := range c.items {
        if now.Before(el.Value.(*entry).expiresAt) {
            count++
        }
    }
    return count
}

func (c *Cache) removeElement(el *list.Element) {
    ent := el.Value.(*entry)
    delete(c.items, ent.key)
    c.order.Remove(el)
}
