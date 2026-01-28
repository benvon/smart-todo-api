package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/benvon/smart-todo/internal/validation"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// TodoHandler handles todo-related requests
type TodoHandler struct {
	todoRepo     *database.TodoRepository
	tagStatsRepo database.TagStatisticsRepositoryInterface
	jobQueue     queue.JobQueue // Optional - if nil, job enqueueing is disabled
	logger       *zap.Logger
}

// NewTodoHandler creates a new todo handler
func NewTodoHandler(todoRepo *database.TodoRepository, logger *zap.Logger) *TodoHandler {
	return &TodoHandler{todoRepo: todoRepo, logger: logger}
}

// NewTodoHandlerWithQueue creates a new todo handler with job queue support
func NewTodoHandlerWithQueue(todoRepo *database.TodoRepository, jobQueue queue.JobQueue, logger *zap.Logger) *TodoHandler {
	return &TodoHandler{
		todoRepo: todoRepo,
		jobQueue: jobQueue,
		logger:   logger,
	}
}

// NewTodoHandlerWithQueueAndTagStats creates a new todo handler with job queue and tag statistics support
func NewTodoHandlerWithQueueAndTagStats(todoRepo *database.TodoRepository, tagStatsRepo database.TagStatisticsRepositoryInterface, jobQueue queue.JobQueue, logger *zap.Logger) *TodoHandler {
	return &TodoHandler{
		todoRepo:     todoRepo,
		tagStatsRepo: tagStatsRepo,
		jobQueue:     jobQueue,
		logger:       logger,
	}
}

// RegisterRoutes registers todo routes on the given router
// The router should already have the /todos prefix (e.g., from apiRouter.PathPrefix("/todos"))
func (h *TodoHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("", h.ListTodos).Methods("GET")
	r.HandleFunc("", h.CreateTodo).Methods("POST")
	// Only register tag stats route if tagStatsRepo is available
	if h.tagStatsRepo != nil {
		r.HandleFunc("/tags/stats", h.GetTagStats).Methods("GET")
	}
	r.HandleFunc("/{id}", h.GetTodo).Methods("GET")
	r.HandleFunc("/{id}", h.UpdateTodo).Methods("PATCH")
	r.HandleFunc("/{id}", h.DeleteTodo).Methods("DELETE")
	r.HandleFunc("/{id}/complete", h.CompleteTodo).Methods("POST")
	r.HandleFunc("/{id}/analyze", h.AnalyzeTodo).Methods("POST")
}

const (
	// MaxTodoTextLength is the maximum length for todo text
	MaxTodoTextLength = 10000
	// MinTodoTextLength is the minimum length for todo text
	MinTodoTextLength = 1
	// DefaultPageSize is the default page size for pagination
	DefaultPageSize = 100
	// MaxPageSize is the maximum page size for pagination
	MaxPageSize = 500
)

// CreateTodoRequest represents a create todo request
type CreateTodoRequest struct {
	Text    string  `json:"text" validate:"required,min=1,max=10000"`
	DueDate *string `json:"due_date,omitempty"` // ISO 8601 (RFC3339) format, e.g., "2024-03-15T14:30:00Z"
}

// UpdateTodoRequest represents an update todo request
type UpdateTodoRequest struct {
	Text        *string            `json:"text,omitempty"`
	TimeHorizon *string            `json:"time_horizon,omitempty"` // Empty string to clear user override and let AI manage
	Status      *models.TodoStatus `json:"status,omitempty"`
	Tags        *[]string          `json:"tags,omitempty"`     // User-defined tags (overrides AI tags)
	DueDate     *string            `json:"due_date,omitempty"` // ISO 8601 (RFC3339) format, e.g., "2024-03-15T14:30:00Z", empty string to clear
}

// ListTodosResponse represents the paginated response for listing todos
type ListTodosResponse struct {
	Todos      []*models.Todo `json:"todos"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	Total      int            `json:"total"`
	TotalPages int            `json:"total_pages"`
}

// ListTodos lists todos for the authenticated user with pagination
func (h *TodoHandler) ListTodos(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	ctx := r.Context()

	// Parse pagination parameters
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := DefaultPageSize
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
			if parsed > MaxPageSize {
				pageSize = MaxPageSize
			} else {
				pageSize = parsed
			}
		}
	}

	// Parse and validate query parameters
	var timeHorizon *models.TimeHorizon
	if th := r.URL.Query().Get("time_horizon"); th != "" {
		if err := validation.ValidateTimeHorizon(th); err != nil {
			respondJSONError(w, http.StatusBadRequest, "Bad Request", err.Error())
			return
		}
		thEnum := models.TimeHorizon(th)
		timeHorizon = &thEnum
	}

	var status *models.TodoStatus
	if s := r.URL.Query().Get("status"); s != "" {
		if err := validation.ValidateTodoStatus(s); err != nil {
			respondJSONError(w, http.StatusBadRequest, "Bad Request", err.Error())
			return
		}
		sEnum := models.TodoStatus(s)
		status = &sEnum
	}

	// Get todos with pagination
	todos, total, err := h.todoRepo.GetByUserIDPaginated(ctx, user.ID, timeHorizon, status, page, pageSize)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve todos")
		return
	}

	// Calculate total pages
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	response := ListTodosResponse{
		Todos:      todos,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}

	respondJSON(w, http.StatusOK, response)
}

// CreateTodo creates a new todo
func (h *TodoHandler) CreateTodo(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	var req CreateTodoRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		// Check if error is due to request size limit
		if maxBytesErr, ok := err.(*http.MaxBytesError); ok {
			respondJSONError(w, http.StatusRequestEntityTooLarge, "Request Entity Too Large", fmt.Sprintf("Request body exceeds maximum size of %d bytes", maxBytesErr.Limit))
			return
		}
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid request body")
		return
	}

	// Validate request
	if err := validation.Validate.Struct(req); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldError := range validationErrors {
				respondJSONError(w, http.StatusBadRequest, "Bad Request", fmt.Sprintf("Validation failed: %s", fieldError.Error()))
				return
			}
		}
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Validation failed")
		return
	}

	// Sanitize text input
	req.Text = validation.SanitizeText(req.Text)
	if req.Text == "" {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Text is required and cannot be empty after sanitization")
		return
	}

	// Validate length after sanitization
	if len(req.Text) > MaxTodoTextLength {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", fmt.Sprintf("Text exceeds maximum length of %d characters", MaxTodoTextLength))
		return
	}

	ctx := r.Context()
	now := time.Now()
	timeEntered := now.Format(time.RFC3339)
	todo := &models.Todo{
		ID:          uuid.New(),
		UserID:      user.ID,
		Text:        req.Text,
		TimeHorizon: models.TimeHorizonSoon, // Default to 'soon'
		Status:      models.TodoStatusPending,
		Metadata: models.Metadata{
			TagSources:  make(map[string]models.TagSource),
			TimeEntered: &timeEntered,
		},
	}

	// Parse due_date if provided
	if req.DueDate != nil && *req.DueDate != "" {
		dueDate, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, "Bad Request", fmt.Sprintf("Invalid due_date format. Expected RFC3339 format (e.g., 2024-03-15T14:30:00Z): %v", err))
			return
		}
		todo.DueDate = &dueDate
	}

	if err := h.todoRepo.Create(ctx, todo); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to create todo")
		return
	}

	// Enqueue AI analysis job if job queue is available
	// Note: Tag change detection is handled automatically by the repository
	if h.jobQueue != nil {
		job := queue.NewJob(queue.JobTypeTaskAnalysis, user.ID, &todo.ID)
		if err := h.jobQueue.Enqueue(ctx, job); err != nil {
			// Log error but don't fail the request
			// The todo was created successfully, analysis can be retried later
			h.logger.Warn("failed_to_enqueue_ai_analysis_job",
				zap.String("todo_id", todo.ID.String()),
				zap.String("user_id", user.ID.String()),
				zap.Error(err),
			)
		} else {
			h.logger.Info("enqueued_ai_analysis_job",
				zap.String("todo_id", todo.ID.String()),
				zap.String("user_id", user.ID.String()),
			)
		}
	} else {
		h.logger.Debug("job_queue_not_available",
			zap.String("todo_id", todo.ID.String()),
			zap.String("user_id", user.ID.String()),
		)
	}

	respondJSON(w, http.StatusCreated, todo)
}

// GetTodo retrieves a todo by ID
func (h *TodoHandler) GetTodo(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid todo ID")
		return
	}

	ctx := r.Context()
	todo, err := h.todoRepo.GetByID(ctx, id)
	if err != nil {
		respondJSONError(w, http.StatusNotFound, "Not Found", "Todo not found")
		return
	}

	// Verify todo belongs to user
	if todo.UserID != user.ID {
		respondJSONError(w, http.StatusForbidden, "Forbidden", "Todo does not belong to user")
		return
	}

	respondJSON(w, http.StatusOK, todo)
}

// UpdateTodo updates an existing todo
func (h *TodoHandler) UpdateTodo(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid todo ID")
		return
	}

	ctx := r.Context()
	todo, err := h.todoRepo.GetByID(ctx, id)
	if err != nil {
		respondJSONError(w, http.StatusNotFound, "Not Found", "Todo not found")
		return
	}

	// Verify todo belongs to user
	if todo.UserID != user.ID {
		respondJSONError(w, http.StatusForbidden, "Forbidden", "Todo does not belong to user")
		return
	}

	// Save old tags for tag change detection
	oldTags := todo.Metadata.CategoryTags

	var req UpdateTodoRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		// Check if error is due to request size limit
		if maxBytesErr, ok := err.(*http.MaxBytesError); ok {
			respondJSONError(w, http.StatusRequestEntityTooLarge, "Request Entity Too Large", fmt.Sprintf("Request body exceeds maximum size of %d bytes", maxBytesErr.Limit))
			return
		}
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid request body")
		return
	}

	// Initialize tag sources if nil
	if todo.Metadata.TagSources == nil {
		todo.Metadata.TagSources = make(map[string]models.TagSource)
	}

	// Update fields if provided with validation
	// Note: Tag change detection is handled automatically by the repository
	if req.Text != nil {
		// Sanitize text input
		sanitized := validation.SanitizeText(*req.Text)
		if sanitized == "" {
			respondJSONError(w, http.StatusBadRequest, "Bad Request", "Text cannot be empty after sanitization")
			return
		}
		if len(sanitized) > MaxTodoTextLength {
			respondJSONError(w, http.StatusBadRequest, "Bad Request", fmt.Sprintf("Text exceeds maximum length of %d characters", MaxTodoTextLength))
			return
		}
		todo.Text = sanitized
	}
	if req.TimeHorizon != nil {
		// Empty string means clear the user override and let AI manage time horizon
		if *req.TimeHorizon == "" {
			// Clear the user override flag to allow AI to manage time horizon again
			override := false
			todo.Metadata.TimeHorizonUserOverride = &override
		} else {
			// Validate enum value
			if err := validation.ValidateTimeHorizon(*req.TimeHorizon); err != nil {
				respondJSONError(w, http.StatusBadRequest, "Bad Request", err.Error())
				return
			}
			// User explicitly setting time horizon - mark as user override
			todo.TimeHorizon = models.TimeHorizon(*req.TimeHorizon)
			// Mark that user has manually set the time horizon
			override := true
			todo.Metadata.TimeHorizonUserOverride = &override
		}
	}
	if req.Status != nil {
		// Validate enum value
		if err := validation.ValidateTodoStatus(string(*req.Status)); err != nil {
			respondJSONError(w, http.StatusBadRequest, "Bad Request", err.Error())
			return
		}
		todo.Status = *req.Status
	}
	if req.Tags != nil {
		// User explicitly setting tags - mark all as user-defined
		// This overrides any AI-generated tags
		todo.Metadata.SetUserTags(*req.Tags)
	}
	if req.DueDate != nil {
		// Empty string means clear the due date
		if *req.DueDate == "" {
			todo.DueDate = nil
		} else {
			dueDate, err := time.Parse(time.RFC3339, *req.DueDate)
			if err != nil {
				respondJSONError(w, http.StatusBadRequest, "Bad Request", fmt.Sprintf("Invalid due_date format. Expected RFC3339 format (e.g., 2024-03-15T14:30:00Z): %v", err))
				return
			}
			todo.DueDate = &dueDate
		}
	}

	if err := h.todoRepo.Update(ctx, todo, oldTags); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to update todo")
		return
	}

	// Note: Tag change detection is handled automatically by the repository

	respondJSON(w, http.StatusOK, todo)
}

// DeleteTodo deletes a todo
func (h *TodoHandler) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid todo ID")
		return
	}

	ctx := r.Context()
	todo, err := h.todoRepo.GetByID(ctx, id)
	if err != nil {
		respondJSONError(w, http.StatusNotFound, "Not Found", "Todo not found")
		return
	}

	// Verify todo belongs to user
	if todo.UserID != user.ID {
		respondJSONError(w, http.StatusForbidden, "Forbidden", "Todo does not belong to user")
		return
	}

	if err := h.todoRepo.Delete(ctx, id); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to delete todo")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CompleteTodo marks a todo as completed
func (h *TodoHandler) CompleteTodo(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid todo ID")
		return
	}

	ctx := r.Context()
	todo, err := h.todoRepo.GetByID(ctx, id)
	if err != nil {
		respondJSONError(w, http.StatusNotFound, "Not Found", "Todo not found")
		return
	}

	// Verify todo belongs to user
	if todo.UserID != user.ID {
		respondJSONError(w, http.StatusForbidden, "Forbidden", "Todo does not belong to user")
		return
	}

	// Save old tags for tag change detection
	oldTags := todo.Metadata.CategoryTags

	// Mark as completed
	now := time.Now()
	todo.Status = models.TodoStatusCompleted
	todo.CompletedAt = &now

	if err := h.todoRepo.Update(ctx, todo, oldTags); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to complete todo")
		return
	}

	respondJSON(w, http.StatusOK, todo)
}

// AnalyzeTodo manually triggers AI analysis for a todo
func (h *TodoHandler) AnalyzeTodo(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid todo ID")
		return
	}

	ctx := r.Context()
	todo, err := h.todoRepo.GetByID(ctx, id)
	if err != nil {
		respondJSONError(w, http.StatusNotFound, "Not Found", "Todo not found")
		return
	}

	// Verify todo belongs to user
	if todo.UserID != user.ID {
		respondJSONError(w, http.StatusForbidden, "Forbidden", "Todo does not belong to user")
		return
	}

	// Enqueue AI analysis job if job queue is available
	if h.jobQueue != nil {
		job := queue.NewJob(queue.JobTypeTaskAnalysis, user.ID, &todo.ID)
		if err := h.jobQueue.Enqueue(ctx, job); err != nil {
			h.logger.Error("failed_to_enqueue_ai_analysis_job_manual",
				zap.String("todo_id", todo.ID.String()),
				zap.String("user_id", user.ID.String()),
				zap.Error(err),
			)
			respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to enqueue analysis job")
			return
		}

		h.logger.Info("enqueued_ai_analysis_job_manual",
			zap.String("todo_id", todo.ID.String()),
			zap.String("user_id", user.ID.String()),
		)
		respondJSON(w, http.StatusAccepted, map[string]string{
			"message": "Analysis job enqueued",
			"todo_id": todo.ID.String(),
		})
		return
	}

	// Job queue not available
	h.logger.Warn("job_queue_not_available_manual_analysis",
		zap.String("todo_id", todo.ID.String()),
		zap.String("user_id", user.ID.String()),
	)
	respondJSONError(w, http.StatusServiceUnavailable, "Service Unavailable", "AI analysis is not available")
}

// TagStatsResponse represents the response for tag statistics
type TagStatsResponse struct {
	TagStats       map[string]models.TagStats `json:"tag_stats"`
	Tainted        bool                       `json:"tainted"`
	LastAnalyzedAt *time.Time                 `json:"last_analyzed_at,omitempty"`
}

// GetTagStats returns tag statistics for the authenticated user
func (h *TodoHandler) GetTagStats(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	// Defensive check: tagStatsRepo should not be nil if route is registered
	// but check anyway to prevent panic
	if h.tagStatsRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "Service Unavailable", "Tag statistics are not available")
		return
	}

	ctx := r.Context()

	// Get tag statistics (or create if doesn't exist)
	stats, err := h.tagStatsRepo.GetByUserIDOrCreate(ctx, user.ID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve tag statistics")
		return
	}

	response := TagStatsResponse{
		TagStats:       stats.TagStats,
		Tainted:        stats.Tainted,
		LastAnalyzedAt: stats.LastAnalyzedAt,
	}

	respondJSON(w, http.StatusOK, response)
}
