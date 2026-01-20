package queue

import (
	"context"
)

// JobQueue is the interface for job queues
type JobQueue interface {
	// Enqueue adds a job to the queue
	Enqueue(ctx context.Context, job *Job) error
	
	// Dequeue removes and returns a message from the queue
	// Returns nil if no message is available
	// The caller is responsible for acknowledging the message
	Dequeue(ctx context.Context) (*Message, error)
	
	// Close closes the queue connection
	Close() error
}
