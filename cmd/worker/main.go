package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/benvon/smart-todo/internal/config"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/logger"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/benvon/smart-todo/internal/services/ai"
	"github.com/benvon/smart-todo/internal/workers"
	"go.uber.org/zap"
)

func main() {
	// Parse command-line flags
	debugFlag := flag.Bool("debug", false, "Enable debug mode for LLM API logging")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override debug mode if flag is set
	debugMode := cfg.WorkerDebugMode || *debugFlag

	// Initialize logger
	zapLogger, err := logger.NewProductionLogger(debugMode)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if syncErr := zapLogger.Sync(); syncErr != nil {
			// Ignore sync errors in production
			_ = syncErr
		}
	}()

	zapLogger.Info("Starting worker",
		zap.Bool("debug_mode", debugMode),
		zap.String("ai_provider", cfg.AIProvider),
		zap.String("ai_model", cfg.AIModel),
	)

	// Initialize database connection
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		zapLogger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer func() {
		if err := db.Close(); err != nil {
			zapLogger.Warn("Failed to close database connection", zap.Error(err))
		}
	}()

	zapLogger.Info("Connected to database")

	// Initialize repositories
	todoRepo := database.NewTodoRepository(db)
	contextRepo := database.NewAIContextRepository(db)
	activityRepo := database.NewUserActivityRepository(db)
	tagStatsRepo := database.NewTagStatisticsRepository(db)

	// Initialize RabbitMQ queue
	jobQueue, err := queue.NewRabbitMQQueue(cfg.RabbitMQURL)
	if err != nil {
		zapLogger.Fatal("Failed to connect to RabbitMQ", zap.Error(err))
	}
	defer func() {
		if err := jobQueue.Close(); err != nil {
			zapLogger.Warn("Failed to close RabbitMQ connection", zap.Error(err))
		}
	}()

	zapLogger.Info("Connected to RabbitMQ",
		zap.Int("prefetch", cfg.RabbitMQPrefetch),
	)

	// Create AI provider with logger
	var aiProvider ai.AIProvider
	if cfg.AIProvider == "openai" {
		aiProvider = ai.NewOpenAIProviderWithLogger(
			cfg.OpenAIKey,
			cfg.AIBaseURL,
			cfg.AIModel,
			zapLogger,
			debugMode,
		)
	} else {
		zapLogger.Fatal("Unsupported AI provider", zap.String("provider", cfg.AIProvider))
	}

	zapLogger.Info("Initialized AI provider",
		zap.String("provider", cfg.AIProvider),
		zap.String("model", cfg.AIModel),
	)

	// Create task analyzer
	analyzer := workers.NewTaskAnalyzer(
		aiProvider,
		todoRepo,
		contextRepo,
		activityRepo,
		tagStatsRepo,
		jobQueue,
		zapLogger,
	)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming messages
	msgChan, errChan, err := jobQueue.Consume(ctx, cfg.RabbitMQPrefetch)
	if err != nil {
		zapLogger.Fatal("Failed to start consuming messages", zap.Error(err))
	}

	zapLogger.Info("Worker started, consuming messages from queue")

	// Process messages
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgChan:
				if !ok {
					zapLogger.Info("Message channel closed")
					return
				}

				// Process job
				if err := analyzer.ProcessJob(ctx, msg); err != nil {
					zapLogger.Error("Failed to process job",
						zap.Error(err),
						zap.String("job_id", msg.GetJob().ID.String()),
						zap.String("job_type", string(msg.GetJob().Type)),
					)
				}
			}
		}
	}()

	// Handle errors
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errChan:
				if !ok {
					return
				}
				zapLogger.Error("Queue error", zap.Error(err))
			}
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	zapLogger.Info("Shutdown signal received, stopping worker...")

	// Cancel context to stop processing
	cancel()

	zapLogger.Info("Worker stopped")
}
