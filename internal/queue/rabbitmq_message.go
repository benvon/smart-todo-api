package queue

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// Message wraps a Job with its RabbitMQ delivery information
type Message struct {
	Job         *Job
	DeliveryTag uint64
	Channel     *amqp.Channel
}

// Ack acknowledges the message
func (m *Message) Ack() error {
	return m.Channel.Ack(m.DeliveryTag, false)
}

// Nack negatively acknowledges the message
func (m *Message) Nack(requeue bool) error {
	return m.Channel.Nack(m.DeliveryTag, false, requeue)
}
