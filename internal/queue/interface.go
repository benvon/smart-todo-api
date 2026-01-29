package queue

import (
	"context"
	"time"
)

// MessageInterface defines the interface for queue messages
// This enables better testability by allowing mock implementations
type MessageInterface interface {
	Ack() error
	Nack(requeue bool) error
	GetJob() *Job
}

// JobQueue is the interface for job queues
type JobQueue interface {
	Enqueue(ctx context.Context, job *Job) error
	Dequeue(ctx context.Context) (*Message, error)
	Consume(ctx context.Context, prefetchCount int) (<-chan *Message, <-chan error, error)
	Close() error
	HealthCheck(ctx context.Context) error
}

// DLQPurger removes dead-lettered messages older than a retention period.
// Implementations consume from the DLQ, ack (discard) old messages, and nack without requeue for recent ones so the purge completes.
type DLQPurger interface {
	PurgeOlderThan(ctx context.Context, retention time.Duration) (purged int, err error)
}
