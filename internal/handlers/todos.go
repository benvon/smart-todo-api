package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/validation"
)

// TodoHandler handles todo-related requests
type TodoHandler struct {
	todoRepo *database.TodoRepository
}

// NewTodoHandler creates a new todo handler
func NewTodoHandler(todoRepo *database.TodoRepository) *TodoHandler {
	return &TodoHandler{todoRepo: todoRepo}
}

// RegisterRoutes registers todo routes on the given router
// The router should already have the /todos prefix (e.g., from apiRouter.PathPrefix("/todos"))
func (h *TodoHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("", h.ListTodos).Methods("GET")
	r.HandleFunc("", h.CreateTodo).Methods("POST")
	r.HandleFunc("/{id}", h.GetTodo).Methods("GET")
	r.HandleFunc("/{id}", h.UpdateTodo).Methods("PATCH")
	r.HandleFunc("/{id}", h.DeleteTodo).Methods("DELETE")
	r.HandleFunc("/{id}/complete", h.CompleteTodo).Methods("POST")
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
	Text string `json:"text" validate:"required,min=1,max=10000"`
}

// UpdateTodoRequest represents an update todo request
type UpdateTodoRequest struct {
	Text        *string              `json:"text,omitempty"`
	TimeHorizon *models.TimeHorizon  `json:"time_horizon,omitempty"`
	Status      *models.TodoStatus   `json:"status,omitempty"`
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
	todo := &models.Todo{
		ID:          uuid.New(),
		UserID:      user.ID,
		Text:        req.Text,
		TimeHorizon: models.TimeHorizonSoon, // Default to 'soon'
		Status:      models.TodoStatusPending,
		Metadata:    models.Metadata{},
	}

	if err := h.todoRepo.Create(ctx, todo); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to create todo")
		return
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

	// Update fields if provided with validation
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
		// Validate enum value
		if err := validation.ValidateTimeHorizon(string(*req.TimeHorizon)); err != nil {
			respondJSONError(w, http.StatusBadRequest, "Bad Request", err.Error())
			return
		}
		todo.TimeHorizon = *req.TimeHorizon
	}
	if req.Status != nil {
		// Validate enum value
		if err := validation.ValidateTodoStatus(string(*req.Status)); err != nil {
			respondJSONError(w, http.StatusBadRequest, "Bad Request", err.Error())
			return
		}
		todo.Status = *req.Status
	}

	if err := h.todoRepo.Update(ctx, todo); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to update todo")
		return
	}

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

	// Mark as completed
	now := time.Now()
	todo.Status = models.TodoStatusCompleted
	todo.CompletedAt = &now

	if err := h.todoRepo.Update(ctx, todo); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to complete todo")
		return
	}

	respondJSON(w, http.StatusOK, todo)
}
