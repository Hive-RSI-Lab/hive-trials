package lru

import (
	"sync"
	"time"
)

// FakeClock is a controllable clock for testing.
// It implements the Clock interface.
type FakeClock struct {
	mu sync.Mutex
	now time.Time
}

// NewFakeClock creates a new fake clock starting at the given time.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start}
}

// Now returns the current fake time.
func (f *FakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the fake clock forward by the given duration.
func (f *FakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}