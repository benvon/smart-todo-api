package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/benvon/smart-todo/internal/config"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/benvon/smart-todo/internal/services/ai"
	"github.com/benvon/smart-todo/internal/workers"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database connection: %v", err)
		}
	}()

	// Connect to RabbitMQ
	jobQueue, err := queue.NewRabbitMQQueue(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer func() {
		if err := jobQueue.Close(); err != nil {
			log.Printf("Failed to close RabbitMQ connection: %v", err)
		}
	}()

	// Initialize repositories
	todoRepo := database.NewTodoRepository(db)
	contextRepo := database.NewAIContextRepository(db)
	activityRepo := database.NewUserActivityRepository(db)

	// Initialize AI provider
	aiProvider, err := createAIProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to create AI provider: %v", err)
	}

	// Initialize workers
	analyzer := workers.NewTaskAnalyzer(aiProvider, todoRepo, contextRepo, activityRepo, jobQueue)
	reprocessor := workers.NewReprocessor(jobQueue, activityRepo)

	// Start garbage collector
	gc := queue.NewGarbageCollector(jobQueue, 1*time.Hour, 7*24*time.Hour)
	gcCtx, gcCancel := context.WithCancel(context.Background())
	defer gcCancel()
	go func() {
		if err := gc.Start(gcCtx); err != nil && err != context.Canceled {
			log.Printf("Garbage collector error: %v", err)
		}
	}()

	// Start reprocessing scheduler (every 12 hours)
	reprocTicker := time.NewTicker(12 * time.Hour)
	defer reprocTicker.Stop()
	go func() {
		for range reprocTicker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			if err := reprocessor.ScheduleReprocessingJobs(ctx); err != nil {
				log.Printf("Failed to schedule reprocessing jobs: %v", err)
			}
			cancel()
		}
	}()

	// Start activity tracker (checks every hour)
	activityTracker := middleware.NewActivityTracker(activityRepo)
	activityCtx, activityCancel := context.WithCancel(context.Background())
	defer activityCancel()
	go activityTracker.Start(activityCtx)

	// Main worker loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Worker started, processing jobs...")

	// Worker loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Dequeue job
				msg, err := jobQueue.Dequeue(ctx)
				if err != nil {
					log.Printf("Failed to dequeue job: %v", err)
					time.Sleep(5 * time.Second)
					continue
				}

				if msg == nil {
					// No job available, wait a bit
					time.Sleep(1 * time.Second)
					continue
				}

				// Check if job should be processed now (respect NotBefore)
				if !msg.Job.ShouldProcess() {
					log.Printf("Job %s scheduled for later (NotBefore: %v), returning to queue",
						msg.Job.ID, msg.Job.NotBefore)
					// Ack to return to queue so it can be picked up later
					if ackErr := msg.Ack(); ackErr != nil {
						log.Printf("Failed to ack delayed job: %v", ackErr)
					}
					time.Sleep(5 * time.Second) // Wait before checking again
					continue
				}

				// Process job
				if err := analyzer.ProcessJob(ctx, msg); err != nil {
					log.Printf("Failed to process job: %v", err)
					// For rate limit errors, wait before trying next job
					// (This is handled in handleJobError, but we add a small delay here too)
					if msg.Job.RetryCount > 0 {
						time.Sleep(2 * time.Second) // Brief pause between retries
					}
				}
			}
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down worker...")

	// Cancel contexts
	cancel()
	gcCancel()
	activityCancel()

	log.Println("Worker stopped")
}

// createAIProvider creates an AI provider based on configuration
func createAIProvider(cfg *config.Config) (ai.AIProvider, error) {
	// Create provider registry
	registry := ai.NewProviderRegistry()
	ai.RegisterOpenAI(registry)

	// Get provider config
	providerType := cfg.AIProvider
	if providerType == "" {
		providerType = "openai"
	}

	config := map[string]string{
		"api_key":  cfg.OpenAIKey,
		"model":    cfg.AIModel,
		"base_url": cfg.AIBaseURL,
	}

	return registry.GetProvider(providerType, config)
}
