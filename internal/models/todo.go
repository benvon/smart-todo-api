package models

import (
	"time"

	"github.com/google/uuid"
)

// TimeHorizon represents when a todo should be done
type TimeHorizon string

const (
	TimeHorizonNow   TimeHorizon = "now"
	TimeHorizonSoon  TimeHorizon = "soon"
	TimeHorizonLater TimeHorizon = "later"
)

// TodoStatus represents the status of a todo
type TodoStatus string

const (
	TodoStatusPending    TodoStatus = "pending"
	TodoStatusProcessing TodoStatus = "processing"
	TodoStatusCompleted  TodoStatus = "completed"
)

// Todo represents a todo item
type Todo struct {
	ID          uuid.UUID   `json:"id"`
	UserID      uuid.UUID   `json:"user_id"`
	Text        string      `json:"text"`
	TimeHorizon TimeHorizon `json:"time_horizon"`
	Status      TodoStatus  `json:"status"`
	Metadata    Metadata    `json:"metadata"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
}
