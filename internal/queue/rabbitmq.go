package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// DefaultQueueName is the default queue name
	DefaultQueueName = "todo_analysis_jobs"
	// DefaultDLQName is the default dead letter queue name
	DefaultDLQName = "todo_analysis_jobs_dlq"
	// DefaultExchangeName is the default exchange name
	DefaultExchangeName = "todo_jobs"
	// DefaultDelayedExchangeName is the default delayed exchange name (requires plugin)
	DefaultDelayedExchangeName = "todo_jobs_delayed"
)

// RabbitMQQueue implements JobQueue using RabbitMQ
type RabbitMQQueue struct {
	conn                *amqp.Connection
	channel             *amqp.Channel
	queueName           string
	dlqName             string
	exchangeName        string
	delayedExchangeName string
}

// NewRabbitMQQueue creates a new RabbitMQ queue
func NewRabbitMQQueue(amqpURL string) (*RabbitMQQueue, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			// Log but don't return the close error
			_ = closeErr
		}
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	queue := &RabbitMQQueue{
		conn:                conn,
		channel:             ch,
		queueName:           DefaultQueueName,
		dlqName:             DefaultDLQName,
		exchangeName:        DefaultExchangeName,
		delayedExchangeName: DefaultDelayedExchangeName,
	}

	// Setup exchanges and queues
	if err := queue.setup(); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			// Log but don't return the close error
			_ = closeErr
		}
		return nil, fmt.Errorf("failed to setup queues: %w", err)
	}

	return queue, nil
}

// setup configures exchanges and queues
func (q *RabbitMQQueue) setup() error {
	// Declare delayed exchange (requires rabbitmq_delayed_message_exchange plugin)
	delayedArgs := amqp.Table{
		"x-delayed-type": "direct",
	}
	err := q.channel.ExchangeDeclare(
		q.delayedExchangeName,
		"x-delayed-message",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		delayedArgs,
	)
	if err != nil {
		// If plugin is not available, the channel might be closed
		// Try to reopen it if necessary
		if q.channel.IsClosed() {
			newCh, openErr := q.conn.Channel()
			if openErr != nil {
				return fmt.Errorf("failed to reopen channel after delayed exchange error: %w", openErr)
			}
			q.channel = newCh
		}
		// Log warning but continue without delayed exchange
		fmt.Printf("Warning: delayed message exchange not available (plugin may not be installed): %v\n", err)
	}

	// Declare regular exchange
	err = q.channel.ExchangeDeclare(
		q.exchangeName,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare dead letter queue
	dlqArgs := amqp.Table{}
	_, err = q.channel.QueueDeclare(
		q.dlqName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		dlqArgs,
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	// Bind DLQ to exchange
	err = q.channel.QueueBind(
		q.dlqName,
		"dlq", // routing key
		q.exchangeName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind DLQ: %w", err)
	}

	// Declare main queue with DLQ
	queueArgs := amqp.Table{
		"x-dead-letter-exchange":    q.exchangeName,
		"x-dead-letter-routing-key": "dlq",
	}
	_, err = q.channel.QueueDeclare(
		q.queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		queueArgs,
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind main queue to both exchanges
	err = q.channel.QueueBind(
		q.queueName,
		"jobs", // routing key
		q.exchangeName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue to exchange: %w", err)
	}

	// Bind to delayed exchange if available (ignore error if plugin not installed)
	_ = q.channel.QueueBind(
		q.queueName,
		"jobs", // routing key
		q.delayedExchangeName,
		false,
		nil,
	)

	return nil
}

// Enqueue adds a job to the queue
func (q *RabbitMQQueue) Enqueue(ctx context.Context, job *Job) error {
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         jobJSON,
		DeliveryMode: amqp.Persistent, // Make message persistent
		MessageId:    job.ID.String(),
		Timestamp:    job.CreatedAt,
	}

	// Calculate TTL from NotAfter if set
	if job.NotAfter != nil {
		ttl := time.Until(*job.NotAfter)
		if ttl > 0 {
			publishing.Expiration = fmt.Sprintf("%d", int(ttl.Milliseconds()))
		}
	}

	var exchangeName string
	var routingKey string
	var headers amqp.Table

	// Use delayed exchange if NotBefore is set
	if job.NotBefore != nil {
		delay := time.Until(*job.NotBefore)
		if delay > 0 {
			exchangeName = q.delayedExchangeName
			headers = amqp.Table{
				"x-delay": int(delay.Milliseconds()),
			}
			publishing.Headers = headers
		} else {
			exchangeName = q.exchangeName
		}
	} else {
		exchangeName = q.exchangeName
	}

	routingKey = "jobs"

	err = q.channel.PublishWithContext(
		ctx,
		exchangeName,
		routingKey,
		false, // mandatory
		false, // immediate
		publishing,
	)
	if err != nil {
		return fmt.Errorf("failed to publish job: %w", err)
	}

	return nil
}

// Consume returns a channel of messages from the queue using async delivery
// This is the recommended approach for production as it eliminates polling delays
// and provides better load balancing across multiple worker instances
func (q *RabbitMQQueue) Consume(ctx context.Context, prefetchCount int) (<-chan *Message, <-chan error, error) {
	// Create a dedicated channel for consuming (best practice: separate channel for consumers)
	consumeCh, err := q.conn.Channel()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create consumer channel: %w", err)
	}

	// Set QoS/prefetch to control how many unacknowledged messages this consumer can hold
	// This ensures fair distribution across multiple workers
	// prefetchCount=1 means each worker gets one message at a time (fair dispatch)
	// Higher values allow workers to prefetch multiple messages (better throughput but less fair)
	if err := consumeCh.Qos(prefetchCount, 0, false); err != nil {
		if closeErr := consumeCh.Close(); closeErr != nil {
			// Log error but continue with original error
			_ = closeErr
		}
		return nil, nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	// Start consuming messages
	deliveries, err := consumeCh.Consume(
		q.queueName,
		"",    // consumer tag (empty = auto-generate)
		false, // auto-ack (false = manual ack required)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		if closeErr := consumeCh.Close(); closeErr != nil {
			// Log error but continue with original error
			_ = closeErr
		}
		return nil, nil, fmt.Errorf("failed to start consuming: %w", err)
	}

	// Create channels for messages and errors
	msgChan := make(chan *Message, prefetchCount)
	errChan := make(chan error, 1)

	// Start goroutine to process deliveries
	go func() {
		defer close(msgChan)
		defer close(errChan)
		defer func() {
			if err := consumeCh.Close(); err != nil {
				// Log error but continue - channel may already be closed
				_ = err
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case delivery, ok := <-deliveries:
				if !ok {
					// Channel closed (connection lost)
					errChan <- fmt.Errorf("delivery channel closed")
					return
				}

				// Check if message has expired
				if delivery.Expiration != "" {
					// Message expired, don't requeue
					_ = delivery.Nack(false, false)
					continue
				}

				// Unmarshal job
				var job Job
				if err := json.Unmarshal(delivery.Body, &job); err != nil {
					// Invalid message, send to DLQ
					_ = delivery.Nack(false, false)
					errChan <- fmt.Errorf("failed to unmarshal job: %w", err)
					continue
				}

				// Check if job should be processed now (respect NotBefore)
				if !job.ShouldProcess() {
					// Not ready yet, requeue for later
					_ = delivery.Nack(false, true)
					continue
				}

				// Create message wrapper
				msg := &Message{
					Job:         &job,
					DeliveryTag: delivery.DeliveryTag,
					Channel:     consumeCh,
				}

				// Send message (non-blocking)
				select {
				case <-ctx.Done():
					// Context cancelled, requeue the message
					_ = delivery.Nack(false, true)
					return
				case msgChan <- msg:
					// Message sent successfully
				}
			}
		}
	}()

	return msgChan, errChan, nil
}

// Dequeue removes and returns a message from the queue
// DEPRECATED: Use Consume() for better performance and scalability
func (q *RabbitMQQueue) Dequeue(ctx context.Context) (*Message, error) {
	msg, ok, err := q.channel.Get(
		q.queueName,
		false, // manual ack
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	if !ok {
		// No message available
		return nil, nil
	}

	// Check if message has expired (RabbitMQ should handle this, but check anyway)
	if msg.Expiration != "" {
		// Message has expired, don't requeue
		_ = msg.Nack(false, false)
		return nil, nil
	}

	var job Job
	if err := json.Unmarshal(msg.Body, &job); err != nil {
		// Invalid message, send to DLQ by nacking without requeue
		_ = msg.Nack(false, false)
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	// Check if job should be processed now (respect NotBefore)
	if !job.ShouldProcess() {
		// Requeue the job if it's not time yet
		_ = msg.Nack(false, true)
		return nil, nil
	}

	return &Message{
		Job:         &job,
		DeliveryTag: msg.DeliveryTag,
		Channel:     q.channel,
	}, nil
}

// Close closes the queue connection
func (q *RabbitMQQueue) Close() error {
	var err error
	if q.channel != nil {
		err = q.channel.Close()
	}
	if q.conn != nil {
		if closeErr := q.conn.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}
