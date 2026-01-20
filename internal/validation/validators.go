package validation

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/go-playground/validator/v10"
)

var (
	// Validate is a shared validator instance
	Validate *validator.Validate
)

func init() {
	Validate = validator.New()

	// Register custom validators for enums
	// These should never fail in normal operation, but log if they do
	if err := Validate.RegisterValidation("time_horizon", validateTimeHorizon); err != nil {
		panic(fmt.Sprintf("failed to register time_horizon validator: %v", err))
	}
	if err := Validate.RegisterValidation("todo_status", validateTodoStatus); err != nil {
		panic(fmt.Sprintf("failed to register todo_status validator: %v", err))
	}
}

// validateTimeHorizon validates that a string is a valid TimeHorizon enum value
func validateTimeHorizon(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	switch models.TimeHorizon(value) {
	case models.TimeHorizonNext, models.TimeHorizonSoon, models.TimeHorizonLater:
		return true
	default:
		return false
	}
}

// validateTodoStatus validates that a string is a valid TodoStatus enum value
func validateTodoStatus(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	switch models.TodoStatus(value) {
	case models.TodoStatusPending, models.TodoStatusProcessing, models.TodoStatusProcessed, models.TodoStatusCompleted:
		return true
	default:
		return false
	}
}

// SanitizeText sanitizes text input by trimming whitespace and removing control characters
func SanitizeText(text string) string {
	// Trim whitespace
	text = strings.TrimSpace(text)

	// Remove control characters except newline and tab
	var sanitized strings.Builder
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			continue
		}
		sanitized.WriteRune(r)
	}

	return sanitized.String()
}

// ValidateTimeHorizon validates a TimeHorizon string value
func ValidateTimeHorizon(value string) error {
	th := models.TimeHorizon(value)
	switch th {
	case models.TimeHorizonNext, models.TimeHorizonSoon, models.TimeHorizonLater:
		return nil
	default:
		return fmt.Errorf("invalid time_horizon: %s (must be 'next', 'soon', or 'later')", value)
	}
}

// ValidateTodoStatus validates a TodoStatus string value
func ValidateTodoStatus(value string) error {
	status := models.TodoStatus(value)
	switch status {
	case models.TodoStatusPending, models.TodoStatusProcessing, models.TodoStatusProcessed, models.TodoStatusCompleted:
		return nil
	default:
		return fmt.Errorf("invalid status: %s (must be 'pending', 'processing', 'processed', or 'completed')", value)
	}
}
