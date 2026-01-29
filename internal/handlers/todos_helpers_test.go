package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
)

func TestParseListParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		query      string
		wantPage   int
		wantSize   int
		wantErr    bool
		wantTH     string
		wantStatus string
	}{
		{"defaults", "", 1, DefaultPageSize, false, "", ""},
		{"page", "page=2", 2, DefaultPageSize, false, "", ""},
		{"page_size", "page_size=50", 1, 50, false, "", ""},
		{"page_size capped", "page_size=9999", 1, MaxPageSize, false, "", ""},
		{"time_horizon", "time_horizon=soon", 1, DefaultPageSize, false, "soon", ""},
		{"status", "status=pending", 1, DefaultPageSize, false, "", "pending"},
		{"invalid time_horizon", "time_horizon=invalid", 0, 0, true, "", ""},
		{"invalid status", "status=invalid", 0, 0, true, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest("GET", "http://test/?"+tt.query, nil)
			got, err := parseListParams(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseListParams() err = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.page != tt.wantPage {
				t.Errorf("page = %d, want %d", got.page, tt.wantPage)
			}
			if got.pageSize != tt.wantSize {
				t.Errorf("pageSize = %d, want %d", got.pageSize, tt.wantSize)
			}
			if tt.wantTH != "" && (got.timeHorizon == nil || string(*got.timeHorizon) != tt.wantTH) {
				t.Errorf("timeHorizon = %v, want %s", got.timeHorizon, tt.wantTH)
			}
			if tt.wantStatus != "" && (got.status == nil || string(*got.status) != tt.wantStatus) {
				t.Errorf("status = %v, want %s", got.status, tt.wantStatus)
			}
		})
	}
}

func TestApplyUpdatesToTodo(t *testing.T) {
	t.Parallel()
	todo := &models.Todo{
		ID:          uuid.New(),
		Text:        "original",
		TimeHorizon: models.TimeHorizonSoon,
		Status:      models.TodoStatusPending,
		Metadata:    models.Metadata{},
	}
	req := &UpdateTodoRequest{
		TimeHorizon: stringPtr("next"),
	}
	err := applyUpdatesToTodo(todo, req)
	if err != nil {
		t.Fatalf("applyUpdatesToTodo: %v", err)
	}
	if todo.TimeHorizon != models.TimeHorizonNext {
		t.Errorf("TimeHorizon = %s, want next", todo.TimeHorizon)
	}
	if todo.Metadata.TimeHorizonUserOverride == nil || !*todo.Metadata.TimeHorizonUserOverride {
		t.Error("expected TimeHorizonUserOverride true")
	}
}

func TestApplyUpdatesToTodo_EmptyTimeHorizonClearsOverride(t *testing.T) {
	t.Parallel()
	override := true
	todo := &models.Todo{
		TimeHorizon: models.TimeHorizonNext,
		Metadata:    models.Metadata{TimeHorizonUserOverride: &override},
	}
	req := &UpdateTodoRequest{TimeHorizon: stringPtr("")}
	err := applyUpdatesToTodo(todo, req)
	if err != nil {
		t.Fatalf("applyUpdatesToTodo: %v", err)
	}
	if todo.Metadata.TimeHorizonUserOverride == nil || *todo.Metadata.TimeHorizonUserOverride {
		t.Error("expected TimeHorizonUserOverride false when clearing")
	}
}

func TestApplyUpdatesToTodo_InvalidTimeHorizon(t *testing.T) {
	t.Parallel()
	todo := &models.Todo{Metadata: models.Metadata{}}
	req := &UpdateTodoRequest{TimeHorizon: stringPtr("invalid")}
	err := applyUpdatesToTodo(todo, req)
	if err == nil {
		t.Error("expected validation error for invalid time_horizon")
	}
}
