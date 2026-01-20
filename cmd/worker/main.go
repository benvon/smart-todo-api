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

	// Configure prefetch count for fair distribution across workers
	// prefetchCount=1 ensures fair dispatch (one message per worker at a time)
	// Higher values improve throughput but can lead to uneven distribution
	// For AI processing jobs, 1-3 is typically optimal
	// Default is 1 if RABBITMQ_PREFETCH is not set
	prefetchCount := cfg.RabbitMQPrefetch
	if prefetchCount < 1 {
		prefetchCount = 1
		log.Printf("Warning: RABBITMQ_PREFETCH must be >= 1, using default of 1")
	}
	log.Printf("Using RabbitMQ prefetch count: %d", prefetchCount)

	// Start consuming messages asynchronously
	msgChan, errChan, err := jobQueue.Consume(ctx, prefetchCount)
	if err != nil {
		log.Fatalf("Failed to start consuming jobs: %v", err)
	}

	// Worker loop - processes messages as they arrive
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-errChan:
				if err != nil {
					log.Printf("Error from message consumer: %v", err)
					// Wait a bit before retrying (the consumer will attempt to reconnect)
					time.Sleep(5 * time.Second)
				}
			case msg, ok := <-msgChan:
				if !ok {
					// Channel closed
					log.Println("Message channel closed, stopping worker")
					return
				}

				// Process job
				if err := analyzer.ProcessJob(ctx, msg); err != nil {
					log.Printf("Failed to process job: %v", err)
					// Error handling (including retries) is done in ProcessJob
					// For rate limit errors, ProcessJob handles re-enqueueing with delays
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
