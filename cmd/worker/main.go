package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Create tag analyzer
	tagAnalyzer := workers.NewTagAnalyzer(
		todoRepo,
		tagStatsRepo,
		zapLogger,
	)

	// Create reprocessor for scheduled reprocessing
	reprocessor := workers.NewReprocessor(
		jobQueue,
		activityRepo,
		zapLogger,
	)

	// Create garbage collector for job cleanup
	// Run every hour, retain jobs for 24 hours
	gc := queue.NewGarbageCollector(jobQueue, 1*time.Hour, 24*time.Hour)

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

	// Start garbage collector
	go func() {
		if err := gc.Start(ctx); err != nil && err != context.Canceled {
			zapLogger.Error("Garbage collector stopped with error", zap.Error(err))
		}
	}()
	zapLogger.Info("Started garbage collector",
		zap.Duration("interval", 1*time.Hour),
		zap.Duration("retention", 24*time.Hour),
	)

	// Start reprocessor scheduler (runs every 12 hours)
	go func() {
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()

		// Run once at startup
		if err := reprocessor.ScheduleReprocessingJobs(ctx); err != nil {
			zapLogger.Error("Failed to schedule initial reprocessing jobs", zap.Error(err))
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := reprocessor.ScheduleReprocessingJobs(ctx); err != nil {
					zapLogger.Error("Failed to schedule reprocessing jobs", zap.Error(err))
				}
			}
		}
	}()
	zapLogger.Info("Started reprocessing scheduler",
		zap.Duration("interval", 12*time.Hour),
	)

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

				job := msg.GetJob()
				
				// Route job to appropriate analyzer based on type
				var err error
				switch job.Type {
				case queue.JobTypeTagAnalysis:
					err = tagAnalyzer.ProcessJob(ctx, msg)
				case queue.JobTypeTaskAnalysis, queue.JobTypeReprocessUser:
					err = analyzer.ProcessJob(ctx, msg)
				default:
					zapLogger.Error("Unknown job type",
						zap.String("job_id", job.ID.String()),
						zap.String("job_type", string(job.Type)),
					)
					// Nack unknown job types
					if nackErr := msg.Nack(false); nackErr != nil {
						zapLogger.Error("Failed to nack unknown job type",
							zap.String("job_id", job.ID.String()),
							zap.Error(nackErr),
						)
					}
					continue
				}

				if err != nil {
					zapLogger.Error("Failed to process job",
						zap.Error(err),
						zap.String("job_id", job.ID.String()),
						zap.String("job_type", string(job.Type)),
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
