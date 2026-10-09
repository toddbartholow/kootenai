// Package wazuh provides Wazuh integration for the Kootenai platform
package wazuh

import (
	"sync"
	"time"
)

// EventDeduplicator prevents replay attacks by rejecting duplicate alert IDs
// within a configurable time window.
type EventDeduplicator struct {
	seen   sync.Map // map[string]time.Time
	window time.Duration
	stopCh chan struct{}
}

// NewEventDeduplicator creates a new deduplicator with the given window.
// Events with the same alert ID within the window are considered duplicates.
func NewEventDeduplicator(window time.Duration) *EventDeduplicator {
	d := &EventDeduplicator{
		window: window,
		stopCh: make(chan struct{}),
	}
	go d.cleanupLoop()
	return d
}

// IsDuplicate returns true if the alertID has been seen within the dedup window.
// If not a duplicate, it records the alertID for future checks.
func (d *EventDeduplicator) IsDuplicate(alertID string) bool {
	if alertID == "" {
		return false
	}

	now := time.Now()

	// Check if we've seen this ID recently
	if val, ok := d.seen.Load(alertID); ok {
		seenAt := val.(time.Time)
		if now.Sub(seenAt) < d.window {
			return true
		}
	}

	// Record this ID
	d.seen.Store(alertID, now)
	return false
}

// Stop stops the cleanup goroutine.
func (d *EventDeduplicator) Stop() {
	close(d.stopCh)
}

// cleanupLoop periodically purges expired entries from the dedup map.
func (d *EventDeduplicator) cleanupLoop() {
	ticker := time.NewTicker(d.window)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-d.window)
			d.seen.Range(func(key, value any) bool {
				if value.(time.Time).Before(cutoff) {
					d.seen.Delete(key)
				}
				return true
			})
		}
	}
}
