package store

import (
	"context"
	"log"
	"time"
)

const (
	// DefaultTTL is the default time-to-live for inbox entries (30 days)
	DefaultTTL = 30 * 24 * time.Hour
	// DefaultPurgeInterval is how often the purger runs (60 seconds)
	DefaultPurgeInterval = 60 * time.Second
)

// TTLPurger manages the background purging of expired inbox entries
type TTLPurger struct {
	store    *Store
	ttl      time.Duration
	interval time.Duration
}

// NewTTLPurger creates a new TTL purger
func NewTTLPurger(store *Store, ttl, interval time.Duration) *TTLPurger {
	return &TTLPurger{
		store:    store,
		ttl:      ttl,
		interval: interval,
	}
}

// Start begins the background purging process
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
