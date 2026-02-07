// Package store — ttl.go implements a background TTL (Time-To-Live) purger
// that periodically cleans up expired inbox entries to prevent unbounded
// memory growth.
//
// LEARNING: This file demonstrates a common Go pattern for background workers:
//   - A struct holds configuration and dependencies
//   - A Start method runs an infinite loop in a goroutine
//   - context.Context controls the lifecycle (start/stop)
//   - time.Ticker drives periodic execution
//
// This pattern is used in many Go applications: cache eviction, metrics
// collection, health checks, database connection pool maintenance, etc.
package store

import (
	"context"
	"log"
	"time"
)

// LEARNING: Package-level constants provide default configuration values.
// Exporting them (uppercase names) allows other packages to reference them,
// which is useful for documentation and testing. The caller can override
// these when constructing the purger.
//
// time.Duration arithmetic is clean in Go: 30 * 24 * time.Hour reads as
// "30 days" without needing helper functions.
const (
	// DefaultTTL is the default time-to-live for inbox entries (30 days)
	DefaultTTL = 30 * 24 * time.Hour
	// DefaultPurgeInterval is how often the purger runs (60 seconds)
	DefaultPurgeInterval = 60 * time.Second
)

// TTLPurger manages the background purging of expired inbox entries.
//
// LEARNING: This struct encapsulates the purger's configuration and
// dependencies. It receives a *Store via dependency injection rather than
// accessing a global variable. This makes the purger testable — you can
// pass in a mock store for unit tests.
type TTLPurger struct {
	store    *Store
	ttl      time.Duration
	interval time.Duration
}

// NewTTLPurger creates a new TTL purger.
func NewTTLPurger(store *Store, ttl, interval time.Duration) *TTLPurger {
	return &TTLPurger{
		store:    store,
		ttl:      ttl,
		interval: interval,
	}
}

// Start begins the background purging process. It runs until the context
// is cancelled (typically during server shutdown).
//
// LEARNING: This function demonstrates the "select loop" pattern — Go's
// primary way to handle multiple concurrent events:
//
//	for {
//	    select {
//	    case <-ctx.Done():    // context was cancelled → stop
//	    case <-ticker.C:      // ticker fired → do work
//	    }
//	}
//
// select blocks until one of its cases is ready. If multiple cases are
// ready simultaneously, Go picks one at random (to prevent starvation).
//
// time.NewTicker creates a channel that receives a value at regular intervals.
// Unlike time.After (which fires once), Ticker repeats until stopped.
// Always defer ticker.Stop() to release the underlying timer resources.
//
// This function is designed to run as a goroutine: go purger.Start(ctx)
// The ctx.Done() case ensures clean shutdown — when the parent calls
// cancel() on the context, this goroutine exits its loop and returns.
func (p *TTLPurger) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	log.Printf("TTL purger started (TTL: %v, interval: %v)", p.ttl, p.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("TTL purger stopped")
			return
		case <-ticker.C:
			purged := p.store.PurgeExpiredInboxEntries(p.ttl)
			if purged > 0 {
				log.Printf("Purged %d expired inbox entries", purged)
			}
		}
	}
}
