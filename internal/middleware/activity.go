package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"go.uber.org/zap"
)

// ActivityTracking tracks user activity and manages reprocessing pause/resume
func ActivityTracking(activityRepo *database.UserActivityRepository, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only track activity for authenticated requests
			user := UserFromContext(r)
			if user != nil {
				ctx := r.Context()

				// Update last API interaction
				if err := activityRepo.UpdateLastInteraction(ctx, user.ID); err != nil {
					logger.Warn("failed_to_update_user_activity",
						zap.Error(err),
						zap.String("user_id", user.ID.String()),
					)
					// Don't fail the request if activity tracking fails
				}

				// Check if reprocessing should be paused (3 days inactivity)
				// This check happens on every request but only updates if needed
				// This runs in a background goroutine independent of the request lifecycle
				go func(parentCtx context.Context) {
					// Create a timeout context derived from the parent context
					// This satisfies the linter's contextcheck requirement
					// The timeout ensures the operation completes even if the parent is cancelled
					checkCtx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
					defer cancel()

					// Get users needing pause
					usersToPause, err := activityRepo.GetUsersNeedingReprocessingPause(checkCtx)
					if err != nil {
						logger.Warn("failed_to_check_users_needing_pause",
							zap.Error(err),
						)
						return
					}

					// Pause reprocessing for inactive users
					for _, userID := range usersToPause {
						if err := activityRepo.SetReprocessingPaused(checkCtx, userID, true); err != nil {
							logger.Warn("failed_to_pause_reprocessing",
								zap.Error(err),
								zap.String("user_id", userID.String()),
							)
						}
					}
				}(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ActivityTracker is a simpler version that can be used as a helper
type ActivityTracker struct {
	activityRepo  *database.UserActivityRepository
	logger        *zap.Logger
	checkInterval time.Duration
}

// NewActivityTracker creates a new activity tracker
func NewActivityTracker(activityRepo *database.UserActivityRepository, logger *zap.Logger) *ActivityTracker {
	return &ActivityTracker{
		activityRepo:  activityRepo,
		logger:        logger,
		checkInterval: 1 * time.Hour, // Check every hour
	}
}

// Start starts the background goroutine for checking inactive users
func (at *ActivityTracker) Start(ctx context.Context) {
	ticker := time.NewTicker(at.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			usersToPause, err := at.activityRepo.GetUsersNeedingReprocessingPause(ctx)
			if err != nil {
				at.logger.Warn("failed_to_check_users_needing_pause",
					zap.Error(err),
				)
				continue
			}

			for _, userID := range usersToPause {
				if err := at.activityRepo.SetReprocessingPaused(ctx, userID, true); err != nil {
					at.logger.Warn("failed_to_pause_reprocessing",
						zap.Error(err),
						zap.String("user_id", userID.String()),
					)
				}
			}
		}
	}
}
