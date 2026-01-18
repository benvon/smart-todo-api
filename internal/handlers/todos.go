package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/models"
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

// CreateTodoRequest represents a create todo request
type CreateTodoRequest struct {
	Text string `json:"text"`
}

// UpdateTodoRequest represents an update todo request
type UpdateTodoRequest struct {
	Text        *string              `json:"text,omitempty"`
	TimeHorizon *models.TimeHorizon  `json:"time_horizon,omitempty"`
	Status      *models.TodoStatus   `json:"status,omitempty"`
}

// ListTodos lists todos for the authenticated user
func (h *TodoHandler) ListTodos(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	ctx := r.Context()
	
	// Parse query parameters
	var timeHorizon *models.TimeHorizon
	if th := r.URL.Query().Get("time_horizon"); th != "" {
		thEnum := models.TimeHorizon(th)
		timeHorizon = &thEnum
	}

	var status *models.TodoStatus
	if s := r.URL.Query().Get("status"); s != "" {
		sEnum := models.TodoStatus(s)
		status = &sEnum
	}

	todos, err := h.todoRepo.GetByUserID(ctx, user.ID, timeHorizon, status)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve todos")
		return
	}

	respondJSON(w, http.StatusOK, todos)
}

// CreateTodo creates a new todo
func (h *TodoHandler) CreateTodo(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	var req CreateTodoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid request body")
		return
	}

	if req.Text == "" {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Text is required")
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

	w.WriteHeader(http.StatusCreated)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid request body")
		return
	}

	// Update fields if provided
	if req.Text != nil {
		todo.Text = *req.Text
	}
	if req.TimeHorizon != nil {
		todo.TimeHorizon = *req.TimeHorizon
	}
	if req.Status != nil {
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
