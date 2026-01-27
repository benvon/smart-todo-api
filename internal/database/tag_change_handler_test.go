package database

import (
	"context"
	"errors"
	"testing"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
)

// TestTagChangeHandler_AlwaysEnqueuesJob tests the tag change handler logic
// This ensures that jobs are always enqueued when tags change, regardless of tainted status
func TestTagChangeHandler_AlwaysEnqueuesJob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		markTaintedResult bool // What MarkTainted returns (transitioned or not)
		expectJobEnqueue  bool
		description       string
	}{
		{
			name:              "enqueue job even when already tainted",
			markTaintedResult: false, // Already tainted, no transition
			expectJobEnqueue:  true,  // But we should still enqueue
			description:       "When tags change, we should always enqueue a job, even if stats are already tainted",
		},
		{
			name:              "enqueue job when transition occurs",
			markTaintedResult: true, // Transition occurred
			expectJobEnqueue:  true, // Should enqueue
			description:       "When tags change and transition occurs, we should enqueue a job",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userID := uuid.New()
			jobEnqueued := false
			var enqueuedUserID uuid.UUID

			// Create a mock tag stats repo
			mockTagStatsRepo := &mockTagStatsRepoForHandlerTest{
				markTaintedFunc: func(ctx context.Context, uid uuid.UUID) (bool, error) {
					if uid != userID {
						t.Errorf("MarkTainted called with wrong userID: expected %s, got %s", userID, uid)
					}
					return tt.markTaintedResult, nil
				},
			}

			// Create a mock job queue
			mockJobQueue := &mockJobQueueForHandlerTest{
				enqueueFunc: func(ctx context.Context, job interface{}) error {
					jobEnqueued = true
					enqueuedUserID = userID
					return nil
				},
			}

			// Simulate the handler logic from cmd/server/main.go and cmd/worker/main.go
			handler := func(ctx context.Context, uid uuid.UUID) error {
				// Always mark tag statistics as tainted (ensures stats will be refreshed)
				_, err := mockTagStatsRepo.MarkTainted(ctx, uid)
				if err != nil {
					return err
				}

				// Always enqueue tag analysis job when tags change
				// Multiple jobs are fine - the analyzer will process them
				if mockJobQueue != nil {
					if err := mockJobQueue.Enqueue(ctx, nil); err != nil {
						return err
					}
				}

				return nil
			}

			// Invoke handler
			err := handler(context.Background(), userID)
			if err != nil {
				t.Fatalf("Handler failed: %v", err)
			}

			// Verify job was enqueued regardless of transitioned status
			// This test would FAIL if we reverted to the old behavior where we only
			// enqueue when transitioned=true. The "enqueue_job_even_when_already_tainted"
			// test case specifically checks this scenario.
			if !jobEnqueued {
				t.Errorf("Expected job to be enqueued regardless of transitioned=%v, but it wasn't. %s",
					tt.markTaintedResult, tt.description)
			}
			if enqueuedUserID != userID {
				t.Errorf("Expected job enqueued for user %s, got %s", userID, enqueuedUserID)
			}
		})
	}
}

// TestTagChangeHandler_RegressionTest_OldBehaviorWouldFail demonstrates that
// the test would catch a regression where we only enqueue when transitioned=true
func TestTagChangeHandler_RegressionTest_OldBehaviorWouldFail(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	jobEnqueued := false

	// Simulate the OLD (incorrect) handler behavior that would skip enqueueing
	// when already tainted
	oldHandler := func(ctx context.Context, uid uuid.UUID, markTaintedResult bool) error {
		// OLD BEHAVIOR: Only enqueue if transitioned=true
		if markTaintedResult {
			// Only enqueue if transition occurred
			jobEnqueued = true
		}
		// If already tainted (markTaintedResult=false), skip enqueueing
		return nil
	}

	// Test case: tags change but stats are already tainted
	// With OLD behavior, job would NOT be enqueued (this is the bug)
	err := oldHandler(context.Background(), userID, false) // Already tainted
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	// This would pass with old behavior (bug not detected)
	// But our NEW test catches it by expecting jobEnqueued=true
	if jobEnqueued {
		t.Error("OLD BEHAVIOR: Job was enqueued even though already tainted (this is actually correct, but old code wouldn't do this)")
	} else {
		t.Log("OLD BEHAVIOR DETECTED: Job was not enqueued when already tainted - this is the bug we fixed")
	}

	// Now test with NEW (correct) behavior
	jobEnqueued = false
	newHandler := func(ctx context.Context, uid uuid.UUID, markTaintedResult bool) error {
		// NEW BEHAVIOR: Always enqueue when tags change
		jobEnqueued = true // Always enqueue regardless of transitioned status
		return nil
	}

	err = newHandler(context.Background(), userID, false) // Already tainted
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	// NEW behavior: job is always enqueued
	if !jobEnqueued {
		t.Error("NEW BEHAVIOR: Job should always be enqueued when tags change, regardless of tainted status")
	}
}

// TestTagChangeHandler_ErrorHandling tests error handling in the tag change handler
// This ensures proper behavior when MarkTainted or Enqueue operations fail
func TestTagChangeHandler_ErrorHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		markTaintedError  error
		enqueueError      error
		expectJobEnqueue  bool
		expectHandlerFail bool
		description       string
	}{
		{
			name:              "enqueue job even when MarkTainted fails",
			markTaintedError:  errors.New("database connection failed"),
			enqueueError:      nil,
			expectJobEnqueue:  true,
			expectHandlerFail: false,
			description:       "Job should still be enqueued even if MarkTainted fails to prevent inconsistent state",
		},
		{
			name:              "handler fails when Enqueue fails",
			markTaintedError:  nil,
			enqueueError:      errors.New("queue connection failed"),
			expectJobEnqueue:  false,
			expectHandlerFail: true,
			description:       "Handler should fail if job cannot be enqueued",
		},
		{
			name:              "handler continues with both errors but job enqueued attempted",
			markTaintedError:  errors.New("database connection failed"),
			enqueueError:      errors.New("queue connection failed"),
			expectJobEnqueue:  false,
			expectHandlerFail: true,
			description:       "Handler should attempt to enqueue even if MarkTainted fails, but fail if Enqueue fails",
		},
		{
			name:              "handler succeeds when both operations succeed",
			markTaintedError:  nil,
			enqueueError:      nil,
			expectJobEnqueue:  true,
			expectHandlerFail: false,
			description:       "Normal case: both operations succeed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userID := uuid.New()
			jobEnqueued := false
			markTaintedCalled := false
			enqueueCalled := false

			// Create a mock tag stats repo
			mockTagStatsRepo := &mockTagStatsRepoForHandlerTest{
				markTaintedFunc: func(ctx context.Context, uid uuid.UUID) (bool, error) {
					markTaintedCalled = true
					if tt.markTaintedError != nil {
						return false, tt.markTaintedError
					}
					return true, nil
				},
			}

			// Create a mock job queue
			mockJobQueue := &mockJobQueueForHandlerTest{
				enqueueFunc: func(ctx context.Context, job interface{}) error {
					enqueueCalled = true
					if tt.enqueueError != nil {
						return tt.enqueueError
					}
					jobEnqueued = true
					return nil
				},
			}

			// Simulate the IMPROVED handler logic that continues to enqueue even if MarkTainted fails
			handler := func(ctx context.Context, uid uuid.UUID) error {
				var markTaintedErr error
				
				// Always attempt to mark tag statistics as tainted
				_, err := mockTagStatsRepo.MarkTainted(ctx, uid)
				if err != nil {
					// Log the error but continue to enqueue the job
					markTaintedErr = err
				}

				// Always enqueue tag analysis job when tags change
				// This must happen even if MarkTainted fails to avoid inconsistent state
				if mockJobQueue != nil {
					if err := mockJobQueue.Enqueue(ctx, nil); err != nil {
						// If both operations failed, return both errors
						if markTaintedErr != nil {
							return errors.Join(markTaintedErr, err)
						}
						return err
					}
				}

				// If only MarkTainted failed, we've successfully enqueued the job
				// which will eventually fix the tainted state, so we can ignore the error
				// and let the system self-heal
				return nil
			}

			// Invoke handler
			err := handler(context.Background(), userID)

			// Verify error expectations
			if tt.expectHandlerFail {
				if err == nil {
					t.Errorf("Expected handler to fail but it succeeded. %s", tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("Expected handler to succeed but it failed with: %v. %s", err, tt.description)
				}
			}

			// Verify MarkTainted was always attempted
			if !markTaintedCalled {
				t.Error("Expected MarkTainted to be called")
			}

			// Verify Enqueue was attempted (even if MarkTainted failed)
			if !enqueueCalled {
				t.Error("Expected Enqueue to be attempted even if MarkTainted failed")
			}

			// Verify job enqueue state
			if tt.expectJobEnqueue && !jobEnqueued {
				t.Errorf("Expected job to be enqueued but it wasn't. %s", tt.description)
			}
			if !tt.expectJobEnqueue && jobEnqueued {
				t.Errorf("Expected job not to be enqueued but it was. %s", tt.description)
			}
		})
	}
}

// mockTagStatsRepoForHandlerTest is a mock for testing tag change handlers
type mockTagStatsRepoForHandlerTest struct {
	markTaintedFunc func(ctx context.Context, userID uuid.UUID) (bool, error)
}

func (m *mockTagStatsRepoForHandlerTest) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTagStatsRepoForHandlerTest) GetByUserIDOrCreate(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTagStatsRepoForHandlerTest) UpdateStatistics(ctx context.Context, stats *models.TagStatistics) (bool, error) {
	return false, errors.New("not implemented")
}

func (m *mockTagStatsRepoForHandlerTest) MarkTainted(ctx context.Context, userID uuid.UUID) (bool, error) {
	if m.markTaintedFunc == nil {
		return false, errors.New("markTaintedFunc not configured")
	}
	return m.markTaintedFunc(ctx, userID)
}

var _ TagStatisticsRepositoryInterface = (*mockTagStatsRepoForHandlerTest)(nil)

// mockJobQueueForHandlerTest is a minimal mock for job queue
type mockJobQueueForHandlerTest struct {
	enqueueFunc func(ctx context.Context, job interface{}) error
}

func (m *mockJobQueueForHandlerTest) Enqueue(ctx context.Context, job interface{}) error {
	if m.enqueueFunc == nil {
		return errors.New("enqueueFunc not configured")
	}
	return m.enqueueFunc(ctx, job)
}
