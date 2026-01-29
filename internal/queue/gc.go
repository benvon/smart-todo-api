package queue

import (
	"context"
	"fmt"
	"time"
)

// GarbageCollector runs periodic DLQ purges, removing messages older than retention.
type GarbageCollector struct {
	dlqPurger DLQPurger
	interval  time.Duration
	retention time.Duration
}

// NewGarbageCollector creates a new garbage collector. purger is used to purge DLQ messages
// older than retention; pass a RabbitMQ queue (implements DLQPurger) or another implementation.
func NewGarbageCollector(purger DLQPurger, interval time.Duration, retention time.Duration) *GarbageCollector {
	return &GarbageCollector{
		dlqPurger: purger,
		interval:  interval,
		retention: retention,
	}
}

// Start runs the GC loop until ctx is cancelled.
func (gc *GarbageCollector) Start(ctx context.Context) error {
	ticker := time.NewTicker(gc.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := gc.collect(ctx); err != nil {
				fmt.Printf("DLQ GC error: %v\n", err)
			}
		}
	}
}

// collect purges DLQ messages older than retention.
func (gc *GarbageCollector) collect(ctx context.Context) error {
	if gc.dlqPurger == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	n, err := gc.dlqPurger.PurgeOlderThan(ctx, gc.retention)
	if err != nil {
		return fmt.Errorf("DLQ purge: %w", err)
	}
	if n > 0 {
		fmt.Printf("DLQ GC purged %d message(s) older than %v\n", n, gc.retention)
	}
	return nil
}
