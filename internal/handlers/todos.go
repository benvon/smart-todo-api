package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	logpkg "github.com/benvon/smart-todo/internal/logger"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/benvon/smart-todo/internal/request"
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
	jobQueue     queue.JobQueue
	logger       *zap.Logger
}

// TodoHandlerOption configures a TodoHandler.
type TodoHandlerOption func(*TodoHandler)

// WithTodoJobQueue sets the job queue for enqueueing analysis jobs.
func WithTodoJobQueue(q queue.JobQueue) TodoHandlerOption {
	return func(h *TodoHandler) { h.jobQueue = q }
}

// WithTodoTagStatsRepo sets the tag statistics repository for /tags/stats.
func WithTodoTagStatsRepo(r database.TagStatisticsRepositoryInterface) TodoHandlerOption {
	return func(h *TodoHandler) { h.tagStatsRepo = r }
}

// NewTodoHandler creates a new todo handler. Options add job queue and/or tag stats support.
func NewTodoHandler(todoRepo *database.TodoRepository, logger *zap.Logger, opts ...TodoHandlerOption) *TodoHandler {
	h := &TodoHandler{todoRepo: todoRepo, logger: logger}
	for _, o := range opts {
		o(h)
	}
	return h
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

// listParams holds parsed list query parameters.
type listParams struct {
	page        int
	pageSize    int
	timeHorizon *models.TimeHorizon
	status      *models.TodoStatus
}

// parseListParams parses and validates list query params from r. Returns an error for invalid values.
func parseListParams(r *http.Request) (listParams, error) {
	out := listParams{
		page:     parsePage(r.URL.Query().Get("page")),
		pageSize: parsePageSize(r.URL.Query().Get("page_size")),
	}
	th, err := parseTimeHorizon(r.URL.Query().Get("time_horizon"))
	if err != nil {
		return listParams{}, err
	}
	out.timeHorizon = th
	st, err := parseStatus(r.URL.Query().Get("status"))
	if err != nil {
		return listParams{}, err
	}
	out.status = st
	return out, nil
}

func parsePage(p string) int {
	if p == "" {
		return 1
	}
	parsed, err := strconv.Atoi(p)
	if err != nil || parsed <= 0 {
		return 1
	}
	return parsed
}

func parsePageSize(ps string) int {
	if ps == "" {
		return DefaultPageSize
	}
	parsed, err := strconv.Atoi(ps)
	if err != nil || parsed <= 0 {
		return DefaultPageSize
	}
	if parsed > MaxPageSize {
		return MaxPageSize
	}
	return parsed
}

func parseTimeHorizon(th string) (*models.TimeHorizon, error) {
	if th == "" {
		return nil, nil
	}
	if err := validation.ValidateTimeHorizon(th); err != nil {
		return nil, err
	}
	h := models.TimeHorizon(th)
	return &h, nil
}

func parseStatus(s string) (*models.TodoStatus, error) {
	if s == "" {
		return nil, nil
	}
	if err := validation.ValidateTodoStatus(s); err != nil {
		return nil, err
	}
	st := models.TodoStatus(s)
	return &st, nil
}

// ListTodos lists todos for the authenticated user with pagination
func (h *TodoHandler) ListTodos(w http.ResponseWriter, r *http.Request) {
	user := request.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}
	params, err := parseListParams(r)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	ctx := r.Context()
	todos, total, err := h.todoRepo.GetByUserIDPaginated(ctx, user.ID, params.timeHorizon, params.status, params.page, params.pageSize)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve todos")
		return
	}
	totalPages := (total + params.pageSize - 1) / params.pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	respondJSON(w, http.StatusOK, ListTodosResponse{
		Todos:      todos,
		Page:       params.page,
		PageSize:   params.pageSize,
		Total:      total,
		TotalPages: totalPages,
	})
}

// CreateTodo creates a new todo
func (h *TodoHandler) CreateTodo(w http.ResponseWriter, r *http.Request) {
	user := request.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}
	req, err := decodeCreateTodoRequest(r)
	if err != nil {
		respondCreateTodoDecodeError(w, err)
		return
	}
	if err := validateCreateTodoRequest(w, &req); err != nil {
		return
	}
	todo, err := buildTodoFromCreateRequest(&req, user)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if err := h.todoRepo.Create(r.Context(), todo); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to create todo")
		return
	}
	h.enqueueCreateTodoJob(r.Context(), user, todo)
	respondJSON(w, http.StatusCreated, todo)
}

func decodeCreateTodoRequest(r *http.Request) (CreateTodoRequest, error) {
	var req CreateTodoRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	return req, err
}

func respondCreateTodoDecodeError(w http.ResponseWriter, err error) {
	if maxBytesErr, ok := err.(*http.MaxBytesError); ok {
		respondJSONError(w, http.StatusRequestEntityTooLarge, "Request Entity Too Large", fmt.Sprintf("Request body exceeds maximum size of %d bytes", maxBytesErr.Limit))
		return
	}
	respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid request body")
}

func validateCreateTodoRequest(w http.ResponseWriter, req *CreateTodoRequest) error {
	if err := validation.Validate.Struct(req); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldError := range validationErrors {
				respondJSONError(w, http.StatusBadRequest, "Bad Request", fmt.Sprintf("Validation failed: %s", fieldError.Error()))
				return err
			}
		}
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Validation failed")
		return err
	}
	req.Text = validation.SanitizeText(req.Text)
	if req.Text == "" {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Text is required and cannot be empty after sanitization")
		return fmt.Errorf("empty text")
	}
	if len(req.Text) > MaxTodoTextLength {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", fmt.Sprintf("Text exceeds maximum length of %d characters", MaxTodoTextLength))
		return fmt.Errorf("text too long")
	}
	return nil
}

func buildTodoFromCreateRequest(req *CreateTodoRequest, user *models.User) (*models.Todo, error) {
	now := time.Now()
	timeEntered := now.Format(time.RFC3339)
	todo := &models.Todo{
		ID:          uuid.New(),
		UserID:      user.ID,
		Text:        req.Text,
		TimeHorizon: models.TimeHorizonSoon,
		Status:      models.TodoStatusPending,
		Metadata: models.Metadata{
			TagSources:  make(map[string]models.TagSource),
			TimeEntered: &timeEntered,
		},
	}
	if req.DueDate != nil && *req.DueDate != "" {
		dueDate, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			return nil, fmt.Errorf("invalid due_date format. Expected RFC3339 format (e.g., 2024-03-15T14:30:00Z): %v", err)
		}
		todo.DueDate = &dueDate
	}
	return todo, nil
}

func (h *TodoHandler) enqueueCreateTodoJob(ctx context.Context, user *models.User, todo *models.Todo) {
	if h.jobQueue == nil {
		h.logger.Debug("job_queue_not_available",
			zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
			zap.String("user_id", logpkg.SanitizeUserID(user.ID.String())),
		)
		return
	}
	job := queue.NewJob(queue.JobTypeTaskAnalysis, user.ID, &todo.ID)
	if err := h.jobQueue.Enqueue(ctx, job); err != nil {
		h.logger.Warn("failed_to_enqueue_ai_analysis_job",
			zap.String("operation", "create_todo"),
			zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
			zap.String("user_id", logpkg.SanitizeUserID(user.ID.String())),
			zap.String("error", logpkg.SanitizeError(err)),
		)
		return
	}
	h.logger.Info("enqueued_ai_analysis_job",
		zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
		zap.String("user_id", logpkg.SanitizeUserID(user.ID.String())),
	)
}

// GetTodo retrieves a todo by ID
func (h *TodoHandler) GetTodo(w http.ResponseWriter, r *http.Request) {
	user := request.UserFromContext(r)
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

// parseAndValidateUpdateRequest decodes the JSON body into UpdateTodoRequest.
func parseAndValidateUpdateRequest(r *http.Request) (UpdateTodoRequest, error) {
	var req UpdateTodoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return UpdateTodoRequest{}, err
	}
	return req, nil
}

// applyUpdatesToTodo applies req fields to todo. Validates and returns an error on invalid values.
func applyUpdatesToTodo(todo *models.Todo, req *UpdateTodoRequest) error {
	if todo.Metadata.TagSources == nil {
		todo.Metadata.TagSources = make(map[string]models.TagSource)
	}
	if err := applyTextUpdate(todo, req.Text); err != nil {
		return err
	}
	if err := applyTimeHorizonUpdate(todo, req.TimeHorizon); err != nil {
		return err
	}
	if err := applyStatusUpdate(todo, req.Status); err != nil {
		return err
	}
	if req.Tags != nil {
		todo.Metadata.SetUserTags(*req.Tags)
	}
	return applyDueDateUpdate(todo, req.DueDate)
}

func applyTextUpdate(todo *models.Todo, text *string) error {
	if text == nil {
		return nil
	}
	sanitized := validation.SanitizeText(*text)
	if sanitized == "" {
		return fmt.Errorf("text cannot be empty after sanitization")
	}
	if len(sanitized) > MaxTodoTextLength {
		return fmt.Errorf("text exceeds maximum length of %d characters", MaxTodoTextLength)
	}
	todo.Text = sanitized
	return nil
}

func applyTimeHorizonUpdate(todo *models.Todo, th *string) error {
	if th == nil {
		return nil
	}
	if *th == "" {
		override := false
		todo.Metadata.TimeHorizonUserOverride = &override
		return nil
	}
	if err := validation.ValidateTimeHorizon(*th); err != nil {
		return err
	}
	todo.TimeHorizon = models.TimeHorizon(*th)
	override := true
	todo.Metadata.TimeHorizonUserOverride = &override
	return nil
}

func applyStatusUpdate(todo *models.Todo, status *models.TodoStatus) error {
	if status == nil {
		return nil
	}
	if err := validation.ValidateTodoStatus(string(*status)); err != nil {
		return err
	}
	todo.Status = *status
	return nil
}

func applyDueDateUpdate(todo *models.Todo, dueDate *string) error {
	if dueDate == nil {
		return nil
	}
	if *dueDate == "" {
		todo.DueDate = nil
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, *dueDate)
	if err != nil {
		return fmt.Errorf("invalid due_date format, expected RFC3339 (e.g. 2024-03-15T14:30:00Z): %w", err)
	}
	todo.DueDate = &parsed
	return nil
}

// UpdateTodo updates an existing todo
func (h *TodoHandler) UpdateTodo(w http.ResponseWriter, r *http.Request) {
	user := request.UserFromContext(r)
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
	if todo.UserID != user.ID {
		respondJSONError(w, http.StatusForbidden, "Forbidden", "Todo does not belong to user")
		return
	}
	oldTags := todo.Metadata.CategoryTags
	req, err := parseAndValidateUpdateRequest(r)
	if err != nil {
		if maxBytesErr, ok := err.(*http.MaxBytesError); ok {
			respondJSONError(w, http.StatusRequestEntityTooLarge, "Request Entity Too Large", fmt.Sprintf("Request body exceeds maximum size of %d bytes", maxBytesErr.Limit))
			return
		}
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid request body")
		return
	}
	if err := applyUpdatesToTodo(todo, &req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	if err := h.todoRepo.Update(ctx, todo, oldTags); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to update todo")
		return
	}
	respondJSON(w, http.StatusOK, todo)
}

// DeleteTodo deletes a todo
func (h *TodoHandler) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	user := request.UserFromContext(r)
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
	user := request.UserFromContext(r)
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
	user := request.UserFromContext(r)
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
				zap.String("operation", "analyze_todo"),
				zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
				zap.String("user_id", logpkg.SanitizeUserID(user.ID.String())),
				zap.String("error", logpkg.SanitizeError(err)),
			)
			respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to enqueue analysis job")
			return
		}

		h.logger.Info("enqueued_ai_analysis_job_manual",
			zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
			zap.String("user_id", logpkg.SanitizeUserID(user.ID.String())),
		)
		respondJSON(w, http.StatusAccepted, map[string]string{
			"message": "Analysis job enqueued",
			"todo_id": todo.ID.String(),
		})
		return
	}

	// Job queue not available
	h.logger.Warn("job_queue_not_available_manual_analysis",
		zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
		zap.String("user_id", logpkg.SanitizeUserID(user.ID.String())),
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
	user := request.UserFromContext(r)
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
