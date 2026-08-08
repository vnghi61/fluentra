package clock

import (
	"sync"
	"time"
)

// Clock supplies time to code that needs deterministic tests.
type Clock interface{ Now() time.Time }

// Real is the production clock.
type Real struct{}

// Now returns the current UTC time.
func (Real) Now() time.Time { return time.Now().UTC() }

// Fake is a thread-safe manually controlled clock.
type Fake struct {
	mu  sync.RWMutex
	now time.Time
}

// NewFake creates a fake clock at now.
func NewFake(now time.Time) *Fake { return &Fake{now: now.UTC()} }

// Now returns the fake time.
func (f *Fake) Now() time.Time { f.mu.RLock(); defer f.mu.RUnlock(); return f.now }

// Set changes the fake time.
func (f *Fake) Set(now time.Time) { f.mu.Lock(); defer f.mu.Unlock(); f.now = now.UTC() }

// Advance moves the fake clock by duration.
func (f *Fake) Advance(duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(duration)
}
