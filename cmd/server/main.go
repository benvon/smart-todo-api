package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/benvon/smart-todo/internal/config"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/handlers"
	"github.com/benvon/smart-todo/internal/logger"
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/benvon/smart-todo/internal/services/ai"
	"github.com/benvon/smart-todo/internal/services/oidc"
	"github.com/benvon/smart-todo/internal/telemetry"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
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
	debugMode := cfg.ServerDebugMode || *debugFlag

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

	zapLogger.Info("starting_server",
		zap.Bool("debug_mode", debugMode),
		zap.String("server_port", cfg.ServerPort),
		zap.String("frontend_url", cfg.FrontendURL),
		zap.String("ai_provider", cfg.AIProvider),
		zap.String("ai_model", cfg.AIModel),
		zap.Bool("otel_enabled", cfg.OTELEnabled),
	)

	// Initialize OpenTelemetry if enabled
	var tracerProvider interface{ Shutdown(context.Context) error }
	if cfg.OTELEnabled {
		if cfg.OTELEndpoint == "" {
			zapLogger.Warn("otel_enabled_but_endpoint_not_configured")
		} else {
			tp, err := telemetry.InitTracer(context.Background(), "smart-todo-api", cfg.OTELEndpoint)
			if err != nil {
				zapLogger.Warn("failed_to_initialize_otel_tracer", zap.Error(err))
			} else {
				tracerProvider = tp
				zapLogger.Info("otel_tracer_initialized",
					zap.String("endpoint", cfg.OTELEndpoint),
				)
				defer func() {
					shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer shutdownCancel()
					if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
						zapLogger.Error("failed_to_shutdown_otel_tracer", zap.Error(err))
					}
				}()
			}
		}
	}

	// Connect to database
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		zapLogger.Fatal("failed_to_connect_to_database", zap.Error(err))
	}
	defer func() {
		if err := db.Close(); err != nil {
			zapLogger.Warn("failed_to_close_database_connection", zap.Error(err))
		}
	}()

	zapLogger.Info("connected_to_database")

	// Connect to Redis for rate limiting
	redisLimiter, err := middleware.NewRedisRateLimiter(cfg.RedisURL)
	if err != nil {
		zapLogger.Fatal("failed_to_connect_to_redis", zap.Error(err))
	}
	defer func() {
		if err := redisLimiter.Close(); err != nil {
			zapLogger.Warn("failed_to_close_redis_connection", zap.Error(err))
		}
	}()
	zapLogger.Info("connected_to_redis")

	// Connect to RabbitMQ for job queue (required)
	// Retry connection with exponential backoff to handle RabbitMQ startup delays
	const maxRetries = 10
	const initialDelay = 2 * time.Second
	var jobQueue queue.JobQueue
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		jobQueue, err = queue.NewRabbitMQQueue(cfg.RabbitMQURL)
		if err == nil {
			zapLogger.Info("connected_to_rabbitmq")
			defer func() {
				if err := jobQueue.Close(); err != nil {
					zapLogger.Warn("failed_to_close_rabbitmq_connection", zap.Error(err))
				}
			}()
			break
		}

		lastErr = err
		delay := initialDelay * time.Duration(1<<uint(attempt)) // Exponential backoff
		if delay > 30*time.Second {
			delay = 30 * time.Second // Cap at 30 seconds
		}
		zapLogger.Warn("failed_to_connect_to_rabbitmq_retrying",
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", maxRetries),
			zap.Error(err),
			zap.Duration("retry_delay", delay),
		)
		time.Sleep(delay)
	}

	if err != nil {
		zapLogger.Fatal("failed_to_connect_to_rabbitmq_after_retries",
			zap.Int("max_retries", maxRetries),
			zap.Error(lastErr),
		)
	}

	// Initialize repositories
	todoRepo := database.NewTodoRepository(db)
	todoRepo.SetLogger(zapLogger)
	oidcConfigRepo := database.NewOIDCConfigRepository(db)
	corsConfigRepo := database.NewCorsConfigRepository(db)
	ratelimitConfigRepo := database.NewRatelimitConfigRepository(db)
	contextRepo := database.NewAIContextRepository(db)
	activityRepo := database.NewUserActivityRepository(db)
	tagStatsRepo := database.NewTagStatisticsRepository(db)

	// Set up automatic tag change detection in todo repository
	todoRepo.SetTagStatsRepo(tagStatsRepo)
	todoRepo.SetTagChangeHandler(func(ctx context.Context, userID uuid.UUID) error {
		zapLogger.Debug("tag_change_handler_invoked",
			zap.String("user_id", userID.String()),
		)

		// Attempt to mark tag statistics as tainted (ensures stats will be refreshed)
		var markTaintedErr error
		_, err := tagStatsRepo.MarkTainted(ctx, userID)
		if err != nil {
			zapLogger.Warn("failed_to_mark_tag_statistics_tainted",
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
			markTaintedErr = err
			// Continue to enqueue the job despite this error to avoid inconsistent state
		}

		// Always enqueue tag analysis job when tags change, even if MarkTainted failed
		// The job will eventually fix the tainted state, allowing the system to self-heal
		// Multiple jobs are fine - the analyzer will process them and re-analyze all todos
		if jobQueue != nil {
			tagJob := queue.NewJob(queue.JobTypeTagAnalysis, userID, nil)
			debounceDelay := 5 * time.Second
			notBefore := time.Now().Add(debounceDelay)
			tagJob.NotBefore = &notBefore
			if err := jobQueue.Enqueue(ctx, tagJob); err != nil {
				zapLogger.Error("failed_to_enqueue_tag_analysis_job",
					zap.String("user_id", userID.String()),
					zap.Error(err),
				)
				// If both operations failed, return combined error
				if markTaintedErr != nil {
					return errors.Join(markTaintedErr, fmt.Errorf("failed to enqueue tag analysis job: %w", err))
				}
				return fmt.Errorf("failed to enqueue tag analysis job: %w", err)
			}
			zapLogger.Info("enqueued_tag_analysis_job",
				zap.String("user_id", userID.String()),
				zap.Duration("debounce_delay", debounceDelay),
			)
		} else {
			zapLogger.Warn("job_queue_not_available",
				zap.String("user_id", userID.String()),
			)
		}

		// If only MarkTainted failed but job was enqueued successfully, ignore the error
		// The enqueued job will eventually update statistics and fix the tainted state
		return nil
	})

	// Initialize services
	oidcProvider := oidc.NewProvider(oidcConfigRepo)
	jwksManager := oidc.NewJWKSManager()

	// Initialize AI provider
	aiProvider, err := createAIProvider(cfg, zapLogger, debugMode)
	if err != nil {
		zapLogger.Warn("failed_to_create_ai_provider_ai_features_disabled", zap.Error(err))
		aiProvider = nil
	}

	// Initialize AI services
	var chatService *ai.ChatService
	var contextService *ai.ContextService
	if aiProvider != nil {
		chatService = ai.NewChatService(aiProvider)
		contextService = ai.NewContextService(aiProvider, contextRepo)
	}

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(oidcProvider, cfg.OIDCProvider)
	todoHandler := handlers.NewTodoHandler(todoRepo, zapLogger, handlers.WithTodoTagStatsRepo(tagStatsRepo), handlers.WithTodoJobQueue(jobQueue))
	healthChecker := handlers.NewHealthCheckerWithDeps(db, redisLimiter, jobQueue)

	var chatHandler *handlers.ChatHandler
	if chatService != nil && contextService != nil {
		chatHandler = handlers.NewChatHandler(chatService, contextService, contextRepo, zapLogger)
	}

	// Setup router
	r := mux.NewRouter()

	// Apply middleware (order matters - executed in reverse order of registration)
	// Note: In gorilla/mux, middleware executes in reverse order of registration
	// Middleware registered LAST executes FIRST (outermost wrapper)
	zapLogger.Info("setting_up_middleware")

	// Outermost middleware (executes first):
	// 0. OpenTelemetry tracing (if enabled)
	if cfg.OTELEnabled && tracerProvider != nil {
		r.Use(otelmux.Middleware("smart-todo-api"))
		zapLogger.Info("otel_middleware_enabled")
	}
	// 1. Security headers (should be set on all responses)
	r.Use(middleware.SecurityHeaders(cfg.EnableHSTS))
	// 2. CORS (load from DB, hot-reload; fallback to FRONTEND_URL)
	corsReloader := middleware.NewCORSReloader(corsConfigRepo, cfg.FrontendURL, zapLogger, 1*time.Minute)
	r.Use(corsReloader.Middleware())
	// Rate limit middleware (applied selectively to specific routes, not globally)
	rateLimitReloader := middleware.NewRateLimitReloader(redisLimiter.Client(), ratelimitConfigRepo, "5-S", zapLogger, 1*time.Minute)
	if rateLimitReloader == nil {
		zapLogger.Fatal("failed_to_create_rate_limit_reloader")
	}
	rateLimitMW := rateLimitReloader.Middleware()
	// 3. Request size limits (protects against DoS)
	r.Use(middleware.MaxRequestSize(middleware.DefaultMaxRequestSize))
	// 4. Content-Type validation for POST/PATCH/PUT requests
	r.Use(middleware.ContentType)
	// 5. Request timeout (30 seconds default)
	r.Use(middleware.Timeout(30 * time.Second))
	// 6. Error handler (catches panics)
	r.Use(middleware.ErrorHandler(zapLogger))
	// 7. Audit logging (for security events)
	r.Use(middleware.Audit(zapLogger))
	// 8. Logging (innermost, executes last before handler)
	r.Use(middleware.Logging(zapLogger))
	// 9. Activity tracking (for authenticated requests)
	r.Use(middleware.ActivityTracking(activityRepo, zapLogger))
	r.Use(middleware.ActivityTracking(activityRepo, zapLogger))

	log.Println("Middleware setup complete")

	// Public routes (no rate limiting for health checks)
	r.HandleFunc("/healthz", healthChecker.HealthCheck).Methods("GET")
	r.HandleFunc("/health", healthCheck).Methods("GET") // Legacy endpoint
	r.HandleFunc("/version", versionInfo).Methods("GET")

	// OpenAPI spec (public)
	openAPIPath := filepath.Join("api", "openapi", "openapi.yaml")
	openAPIHandler := handlers.NewOpenAPIHandler(openAPIPath)
	openAPIHandler.RegisterRoutes(r)

	// API v1 routes
	apiRouter := r.PathPrefix("/api/v1").Subrouter()

	// Auth routes
	authRouter := apiRouter.PathPrefix("/auth").Subrouter()

	// Public auth routes with rate limiting (more restrictive for unauthenticated)
	loginRouter := authRouter.PathPrefix("/oidc").Subrouter()
	loginRouter.Use(rateLimitMW)
	loginRouter.HandleFunc("/login", authHandler.GetOIDCLogin).Methods("GET")

	// Protected auth routes
	protectedAuthRouter := authRouter.PathPrefix("").Subrouter()
	protectedAuthRouter.Use(middleware.Auth(db, oidcProvider, jwksManager, cfg.OIDCProvider, zapLogger))
	protectedAuthRouter.Use(rateLimitMW)
	protectedAuthRouter.HandleFunc("/me", authHandler.GetMe).Methods("GET")

	// Todo routes (protected)
	todosRouter := apiRouter.PathPrefix("/todos").Subrouter()
	todosRouter.Use(middleware.Auth(db, oidcProvider, jwksManager, cfg.OIDCProvider, zapLogger))
	todosRouter.Use(rateLimitMW)
	todoHandler.RegisterRoutes(todosRouter)

	// AI routes (protected)
	aiRouter := apiRouter.PathPrefix("/ai").Subrouter()
	aiRouter.Use(middleware.Auth(db, oidcProvider, jwksManager, cfg.OIDCProvider, zapLogger))
	aiRouter.Use(rateLimitMW)

	// AI Context routes
	aiContextHandler := handlers.NewAIContextHandler(contextRepo)
	contextRouter := aiRouter.PathPrefix("/context").Subrouter()
	aiContextHandler.RegisterRoutes(contextRouter)

	// Chat routes (if AI provider available)
	if chatHandler != nil {
		chatHandler.RegisterRoutes(aiRouter)
	}

	// Catch-all OPTIONS handler for preflight requests
	// This ensures OPTIONS requests are handled even if routes don't explicitly allow them
	// The CORS middleware will handle setting headers before this is called
	r.Methods("OPTIONS").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS middleware should have already set headers, just return 204
		w.WriteHeader(http.StatusNoContent)
	})

	// Setup server
	srv := &http.Server{
		Addr:           ":" + cfg.ServerPort,
		Handler:        r,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB max header size
	}

	// CORS and rate limit hot-reload loops
	reloadCtx, reloadCancel := context.WithCancel(context.Background())
	defer reloadCancel()
	go corsReloader.Start(reloadCtx)
	go rateLimitReloader.Start(reloadCtx)

	// Start DLQ garbage collector if the queue implementation supports it
	// Run every hour, retain messages for 24 hours
	if dlqPurger, ok := jobQueue.(queue.DLQPurger); ok {
		dlqGC := queue.NewGarbageCollector(dlqPurger, 1*time.Hour, 24*time.Hour)
		go func() {
			if err := dlqGC.Start(reloadCtx); err != nil && err != context.Canceled {
				zapLogger.Error("dlq_garbage_collector_stopped_with_error", zap.Error(err))
			}
		}()
		zapLogger.Info("started_dlq_garbage_collector",
			zap.Duration("interval", 1*time.Hour),
			zap.Duration("retention", 24*time.Hour),
		)
	}

	// Start server in a goroutine
	go func() {
		zapLogger.Info("server_starting",
			zap.String("port", cfg.ServerPort),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zapLogger.Fatal("server_failed_to_start", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zapLogger.Info("server_shutting_down")
	reloadCancel()

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		zapLogger.Fatal("server_forced_to_shutdown", zap.Error(err))
	}

	zapLogger.Info("server_exited")
}

// createAIProvider creates an AI provider based on configuration
func createAIProvider(cfg *config.Config, logger *zap.Logger, debugMode bool) (ai.AIProvider, error) {
	if cfg.OpenAIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Get provider type
	providerType := cfg.AIProvider
	if providerType == "" {
		providerType = "openai"
	}

	// Create provider directly with logger support
	if providerType == "openai" {
		return ai.NewOpenAIProviderWithLogger(
			cfg.OpenAIKey,
			cfg.AIBaseURL,
			cfg.AIModel,
			logger,
			debugMode,
		), nil
	}

	// Fallback to registry for other providers (without logger)
	registry := ai.NewProviderRegistry()
	ai.RegisterOpenAI(registry)

	config := map[string]string{
		"api_key":  cfg.OpenAIKey,
		"model":    cfg.AIModel,
		"base_url": cfg.AIBaseURL,
	}

	return registry.GetProvider(providerType, config)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		// Use standard log here since we don't have logger in this context
		// This is a fallback for a simple health check endpoint
		_ = err
	}
}

func versionInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Only expose minimal version info (sanitized for security)
	if _, err := fmt.Fprintf(w, `{"version":"1.0.0","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		// Use standard log here since we don't have logger in this context
		// This is a fallback for a simple version endpoint
		_ = err
	}
}
