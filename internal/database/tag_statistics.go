package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
)

// TagStatisticsRepository handles tag statistics database operations
type TagStatisticsRepository struct {
	db *DB
}

// NewTagStatisticsRepository creates a new tag statistics repository
func NewTagStatisticsRepository(db *DB) *TagStatisticsRepository {
	return &TagStatisticsRepository{db: db}
}

// GetByUserID retrieves tag statistics by user ID
func (r *TagStatisticsRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error) {
	stats := &models.TagStatistics{}
	var tagStatsJSON []byte
	var lastAnalyzedAt sql.NullTime

	query := `
		SELECT user_id, tag_stats, tainted, last_analyzed_at, analysis_version, created_at, updated_at
		FROM tag_statistics
		WHERE user_id = $1
	`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&stats.UserID,
		&tagStatsJSON,
		&stats.Tainted,
		&lastAnalyzedAt,
		&stats.AnalysisVersion,
		&stats.CreatedAt,
		&stats.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tag statistics not found for user %s", userID)
		}
		return nil, fmt.Errorf("failed to get tag statistics: %w", err)
	}

	if len(tagStatsJSON) > 0 {
		if err := json.Unmarshal(tagStatsJSON, &stats.TagStats); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tag_stats: %w", err)
		}
	} else {
		stats.TagStats = make(map[string]models.TagStats)
	}

	if lastAnalyzedAt.Valid {
		stats.LastAnalyzedAt = &lastAnalyzedAt.Time
	}

	return stats, nil
}

// GetByUserIDOrCreate retrieves tag statistics or creates a new record if not found
func (r *TagStatisticsRepository) GetByUserIDOrCreate(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error) {
	stats, err := r.GetByUserID(ctx, userID)
	if err == nil {
		return stats, nil
	}

	// Create new record if not found
	stats = &models.TagStatistics{
		UserID:          userID,
		TagStats:        make(map[string]models.TagStats),
		Tainted:         true,
		AnalysisVersion: 0,
	}

	// Use Upsert to handle race condition where record might be created between GetByUserID and Create
	if err := r.Upsert(ctx, stats); err != nil {
		return nil, fmt.Errorf("failed to create tag statistics: %w", err)
	}

	// Re-fetch to get the created record with timestamps
	return r.GetByUserID(ctx, userID)
}

// Create creates a new tag statistics record
func (r *TagStatisticsRepository) Create(ctx context.Context, stats *models.TagStatistics) error {
	query := `
		INSERT INTO tag_statistics (user_id, tag_stats, tainted, last_analyzed_at, analysis_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`

	tagStatsJSON, err := json.Marshal(stats.TagStats)
	if err != nil {
		return fmt.Errorf("failed to marshal tag_stats: %w", err)
	}

	var lastAnalyzedAt sql.NullTime
	if stats.LastAnalyzedAt != nil {
		lastAnalyzedAt = sql.NullTime{Time: *stats.LastAnalyzedAt, Valid: true}
	}

	now := time.Now()
	err = r.db.QueryRowContext(ctx, query,
		stats.UserID,
		tagStatsJSON,
		stats.Tainted,
		lastAnalyzedAt,
		stats.AnalysisVersion,
		now,
		now,
	).Scan(&stats.CreatedAt, &stats.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create tag statistics: %w", err)
	}

	return nil
}

// UpdateStatistics atomically updates tag statistics with version check
// Returns true if update succeeded, false if version conflict occurred
func (r *TagStatisticsRepository) UpdateStatistics(ctx context.Context, stats *models.TagStatistics) (bool, error) {
	query := `
		UPDATE tag_statistics
		SET tag_stats = $1, tainted = false, last_analyzed_at = $2, analysis_version = analysis_version + 1, updated_at = $3
		WHERE user_id = $4 AND analysis_version = $5
		RETURNING analysis_version, created_at, updated_at
	`

	tagStatsJSON, err := json.Marshal(stats.TagStats)
	if err != nil {
		return false, fmt.Errorf("failed to marshal tag_stats: %w", err)
	}

	now := time.Now()
	var lastAnalyzedAt sql.NullTime
	if stats.LastAnalyzedAt != nil {
		lastAnalyzedAt = sql.NullTime{Time: *stats.LastAnalyzedAt, Valid: true}
	} else {
		lastAnalyzedAt = sql.NullTime{Time: now, Valid: true}
	}

	var newVersion int
	err = r.db.QueryRowContext(ctx, query,
		tagStatsJSON,
		lastAnalyzedAt,
		now,
		stats.UserID,
		stats.AnalysisVersion,
	).Scan(&newVersion, &stats.CreatedAt, &stats.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			// Version conflict - another update occurred
			return false, nil
		}
		return false, fmt.Errorf("failed to update tag statistics: %w", err)
	}

	stats.AnalysisVersion = newVersion
	stats.Tainted = false
	if lastAnalyzedAt.Valid {
		stats.LastAnalyzedAt = &lastAnalyzedAt.Time
	}

	return true, nil
}

// MarkTainted atomically marks statistics as tainted if currently not tainted
// Returns true if transition occurred (false->true), false if already tainted
func (r *TagStatisticsRepository) MarkTainted(ctx context.Context, userID uuid.UUID) (bool, error) {
	query := `
		UPDATE tag_statistics
		SET tainted = true, updated_at = $1
		WHERE user_id = $2 AND tainted = false
		RETURNING user_id
	`

	var resultID uuid.UUID
	err := r.db.QueryRowContext(ctx, query, time.Now(), userID).Scan(&resultID)

	if err != nil {
		if err == sql.ErrNoRows {
			// Already tainted or record doesn't exist - create/update record
			// Use upsert to ensure record exists
			upsertQuery := `
				INSERT INTO tag_statistics (user_id, tag_stats, tainted, analysis_version, created_at, updated_at)
				VALUES ($1, '{}', true, 0, $2, $2)
				ON CONFLICT (user_id) DO UPDATE
				SET tainted = true, updated_at = $2
				WHERE tag_statistics.tainted = false
				RETURNING user_id
			`
			err = r.db.QueryRowContext(ctx, upsertQuery, userID, time.Now()).Scan(&resultID)
			if err != nil {
				if err == sql.ErrNoRows {
					// Already tainted, no transition
					return false, nil
				}
				return false, fmt.Errorf("failed to mark tainted: %w", err)
			}
			// Transition occurred via upsert
			return true, nil
		}
		return false, fmt.Errorf("failed to mark tainted: %w", err)
	}

	// Transition occurred
	return true, nil
}

// Upsert creates or updates tag statistics
func (r *TagStatisticsRepository) Upsert(ctx context.Context, stats *models.TagStatistics) error {
	query := `
		INSERT INTO tag_statistics (user_id, tag_stats, tainted, last_analyzed_at, analysis_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE
		SET tag_stats = EXCLUDED.tag_stats,
		    tainted = EXCLUDED.tainted,
		    last_analyzed_at = EXCLUDED.last_analyzed_at,
		    analysis_version = EXCLUDED.analysis_version,
		    updated_at = EXCLUDED.updated_at
		RETURNING created_at, updated_at
	`

	tagStatsJSON, err := json.Marshal(stats.TagStats)
	if err != nil {
		return fmt.Errorf("failed to marshal tag_stats: %w", err)
	}

	var lastAnalyzedAt sql.NullTime
	if stats.LastAnalyzedAt != nil {
		lastAnalyzedAt = sql.NullTime{Time: *stats.LastAnalyzedAt, Valid: true}
	}

	now := time.Now()
	err = r.db.QueryRowContext(ctx, query,
		stats.UserID,
		tagStatsJSON,
		stats.Tainted,
		lastAnalyzedAt,
		stats.AnalysisVersion,
		now,
		now,
	).Scan(&stats.CreatedAt, &stats.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert tag statistics: %w", err)
	}

	return nil
}
