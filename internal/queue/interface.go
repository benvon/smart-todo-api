package queue

import (
	"context"
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
	// Enqueue adds a job to the queue
	Enqueue(ctx context.Context, job *Job) error

	// Dequeue removes and returns a message from the queue
	// Returns nil if no message is available
	// The caller is responsible for acknowledging the message
	// DEPRECATED: Use Consume() for better performance and scalability
	Dequeue(ctx context.Context) (*Message, error)

	// Consume returns a channel of messages from the queue
	// Messages are delivered asynchronously as they arrive
	// The caller is responsible for acknowledging each message
	// Prefetch controls how many unacknowledged messages each consumer can hold
	// Returns a channel that will be closed when the context is cancelled or an error occurs
	Consume(ctx context.Context, prefetchCount int) (<-chan *Message, <-chan error, error)

	// Close closes the queue connection
	Close() error

	// HealthCheck verifies the queue connection is healthy
	HealthCheck(ctx context.Context) error
}
