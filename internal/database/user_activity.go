package database

import (
	"context"
	"fmt"
	"time"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
)

// UserActivityRepository handles user activity database operations
type UserActivityRepository struct {
	db *DB
}

// NewUserActivityRepository creates a new user activity repository
func NewUserActivityRepository(db *DB) *UserActivityRepository {
	return &UserActivityRepository{db: db}
}

// GetByUserID retrieves user activity by user ID
func (r *UserActivityRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.UserActivity, error) {
	activity := &models.UserActivity{}

	query := `
		SELECT user_id, last_api_interaction, reprocessing_paused, created_at, updated_at
		FROM user_activity
		WHERE user_id = $1
	`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&activity.UserID,
		&activity.LastAPIInteraction,
		&activity.ReprocessingPaused,
		&activity.CreatedAt,
		&activity.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get user activity: %w", err)
	}

	return activity, nil
}

// Upsert creates or updates user activity
func (r *UserActivityRepository) Upsert(ctx context.Context, activity *models.UserActivity) error {
	query := `
		INSERT INTO user_activity (user_id, last_api_interaction, reprocessing_paused, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET last_api_interaction = EXCLUDED.last_api_interaction,
		    reprocessing_paused = EXCLUDED.reprocessing_paused,
		    updated_at = EXCLUDED.updated_at
		RETURNING created_at, updated_at
	`

	now := time.Now()
	err := r.db.QueryRowContext(ctx, query,
		activity.UserID,
		activity.LastAPIInteraction,
		activity.ReprocessingPaused,
		now,
		now,
	).Scan(&activity.CreatedAt, &activity.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert user activity: %w", err)
	}

	return nil
}

// UpdateLastInteraction updates the last API interaction timestamp
func (r *UserActivityRepository) UpdateLastInteraction(ctx context.Context, userID uuid.UUID) error {
	query := `
		INSERT INTO user_activity (user_id, last_api_interaction, reprocessing_paused, created_at, updated_at)
		VALUES ($1, $2, false, $3, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET last_api_interaction = EXCLUDED.last_api_interaction,
		    reprocessing_paused = false,
		    updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, userID, now, now)
	if err != nil {
		return fmt.Errorf("failed to update last interaction: %w", err)
	}

	return nil
}

// SetReprocessingPaused sets the reprocessing paused flag
func (r *UserActivityRepository) SetReprocessingPaused(ctx context.Context, userID uuid.UUID, paused bool) error {
	query := `
		UPDATE user_activity
		SET reprocessing_paused = $1, updated_at = $2
		WHERE user_id = $3
	`

	_, err := r.db.ExecContext(ctx, query, paused, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to set reprocessing paused: %w", err)
	}

	return nil
}

// GetEligibleUsersForReprocessing returns users who are eligible for reprocessing
// (not paused, within activity window)
func (r *UserActivityRepository) GetEligibleUsersForReprocessing(ctx context.Context) ([]uuid.UUID, error) {
	query := `
		SELECT user_id
		FROM user_activity
		WHERE reprocessing_paused = false
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query eligible users: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Log error but continue - rows may already be closed
			_ = err
		}
	}()

	var userIDs []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan user ID: %w", err)
		}
		userIDs = append(userIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return userIDs, nil
}

// GetUsersNeedingReprocessingPause returns users who need reprocessing paused (3 days inactive)
func (r *UserActivityRepository) GetUsersNeedingReprocessingPause(ctx context.Context) ([]uuid.UUID, error) {
	query := `
		SELECT user_id
		FROM user_activity
		WHERE last_api_interaction < NOW() - INTERVAL '3 days'
		  AND reprocessing_paused = false
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users needing pause: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Log error but continue - rows may already be closed
			_ = err
		}
	}()

	var userIDs []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan user ID: %w", err)
		}
		userIDs = append(userIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return userIDs, nil
}
