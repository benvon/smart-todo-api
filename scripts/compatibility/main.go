// This fixture is compiled against BOTH release revisions. It uses their actual
// HTTP handlers, repositories and task processor; only external AI is a stub.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/handlers"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/benvon/smart-todo/internal/request"
	"github.com/benvon/smart-todo/internal/services/ai"
	"github.com/benvon/smart-todo/internal/workers"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type fixedAI struct{ label string }

func (a fixedAI) AnalyzeTask(context.Context, string, *models.AIContext) ([]string, models.TimeHorizon, error) {
	return []string{"compat-" + a.label}, models.TimeHorizonNext, nil
}
func (fixedAI) Chat(context.Context, []ai.ChatMessage, *models.AIContext) (*ai.ChatResponse, error) {
	return nil, errors.New("not used by compatibility fixture")
}
func (fixedAI) SummarizeContext(context.Context, []ai.ChatMessage) (string, error) {
	return "", errors.New("not used by compatibility fixture")
}

func createTodo(ctx context.Context, handler *handlers.TodoHandler, user *models.User, label string) error {
	body, err := json.Marshal(map[string]string{"text": "compat-" + label})
	if err != nil {
		return err
	}
	req := httptest.NewRequest("POST", "/todos", bytes.NewReader(body)).WithContext(request.WithUser(ctx, user))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.CreateTodo(response, req)
	if response.Code != 201 {
		return fmt.Errorf("API CreateTodo returned %d", response.Code)
	}
	read := httptest.NewRequest("GET", "/todos", nil).WithContext(request.WithUser(ctx, user))
	listed := httptest.NewRecorder()
	handler.ListTodos(listed, read)
	if listed.Code != 200 {
		return fmt.Errorf("API ListTodos returned %d", listed.Code)
	}
	return nil
}

func processPeer(ctx context.Context, db *database.DB, analyzer *workers.TaskAnalyzer, user *models.User, label string) (bool, error) {
	var todoID uuid.UUID
	var text string
	// Prefer the other revision's writes, proving bidirectional data compatibility.
	err := db.QueryRowContext(ctx, `SELECT id,text FROM todos WHERE user_id=$1 AND status='pending'
		ORDER BY (text=$2), created_at LIMIT 1`, user.ID, "compat-"+label).Scan(&todoID, &text)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	job := queue.NewJob(queue.JobTypeTaskAnalysis, user.ID, &todoID)
	// Serialize through the real queue wire format before processing.
	wire, err := json.Marshal(job)
	if err != nil {
		return false, err
	}
	var decoded queue.Job
	if err := json.Unmarshal(wire, &decoded); err != nil {
		return false, err
	}
	if err := analyzer.ProcessTaskAnalysisJob(ctx, &decoded); err != nil {
		return false, err
	}
	todo, err := database.NewTodoRepository(db).GetByID(ctx, todoID)
	if err != nil {
		return false, err
	}
	if todo.Status != models.TodoStatusProcessed || todo.TimeHorizon != models.TimeHorizonNext {
		return false, errors.New("worker result was not preserved")
	}
	return text != "compat-"+label, nil
}

func run() error {
	if len(os.Args) != 3 {
		return errors.New("usage: compatibility LABEL STATE_DIRECTORY")
	}
	label, state := os.Args[1], os.Args[2]
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, err := database.New(os.Getenv("DATABASE_URL"))
	if err != nil {
		return errors.New("compatibility database connection failed")
	}
	defer func() {
		if err := db.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "close compatibility database failed")
		}
	}()
	user := &models.User{ID: uuid.MustParse("10000000-0000-4000-8000-000000000001"), Email: "compat@example.test"}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id,email) VALUES ($1,$2) ON CONFLICT (id) DO NOTHING", user.ID, user.Email); err != nil {
		return err
	}
	repo := database.NewTodoRepository(db)
	handler := handlers.NewTodoHandler(repo, zap.NewNop())
	analyzer := workers.NewTaskAnalyzer(fixedAI{label}, repo, database.NewAIContextRepository(db),
		database.NewUserActivityRepository(db), nil, nil, zap.NewNop())
	return exercise(ctx, db, handler, analyzer, user, label, state)
}

// Leave a backlog for peer consumption while the old worker also processes
// its own writes before the new revision starts.
func createBacklog(ctx context.Context, handler *handlers.TodoHandler, user *models.User, label string) error {
	for range 2 {
		if err := createTodo(ctx, handler, user, label); err != nil {
			return err
		}
	}
	return nil
}

func exercise(ctx context.Context, db *database.DB, handler *handlers.TodoHandler, analyzer *workers.TaskAnalyzer, user *models.User, label, state string) error {
	cycles, peerJobs := 0, 0
	for {
		if err := createBacklog(ctx, handler, user, label); err != nil {
			return err
		}
		processed, err := processPeer(ctx, db, analyzer, user, label)
		if err != nil {
			return err
		}
		if processed {
			peerJobs++
		}
		cycles += 2
		if err := os.WriteFile(filepath.Join(state, label+".ready"), []byte("ready"), 0600); err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(state, "stop")); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	return writeResults(state, label, cycles, peerJobs)
}

func writeResults(state, label string, cycles, peerJobs int) error {
	if peerJobs == 0 {
		return errors.New("no peer API writes processed by worker")
	}
	result, err := json.Marshal(map[string]int{"api_writes": cycles, "peer_jobs_processed": peerJobs})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(state, label+".json"), result, 0600)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "compatibility fixture:", err)
		os.Exit(1)
	}
}
