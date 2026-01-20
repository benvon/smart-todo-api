package main

import (
	"context"
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
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/benvon/smart-todo/internal/services/ai"
	"github.com/benvon/smart-todo/internal/services/oidc"
	"github.com/gorilla/mux"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("Configuration loaded - FrontendURL: '%s', ServerPort: '%s'", cfg.FrontendURL, cfg.ServerPort)

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

	// Connect to Redis for rate limiting
	redisLimiter, err := middleware.NewRedisRateLimiter(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer func() {
		if err := redisLimiter.Close(); err != nil {
			log.Printf("Failed to close Redis connection: %v", err)
		}
	}()
	log.Println("Connected to Redis for rate limiting")

	// Connect to RabbitMQ for job queue (optional)
	// Retry connection with exponential backoff to handle RabbitMQ startup delays
	var jobQueue queue.JobQueue
	if cfg.RabbitMQURL != "" {
		const maxRetries = 10
		const initialDelay = 2 * time.Second
		var lastErr error

		for attempt := 0; attempt < maxRetries; attempt++ {
			jobQueue, err = queue.NewRabbitMQQueue(cfg.RabbitMQURL)
			if err == nil {
				log.Println("Connected to RabbitMQ for job queue")
				defer func() {
					if err := jobQueue.Close(); err != nil {
						log.Printf("Failed to close RabbitMQ connection: %v", err)
					}
				}()
				break
			}

			lastErr = err
			delay := initialDelay * time.Duration(1<<uint(attempt)) // Exponential backoff
			if delay > 30*time.Second {
				delay = 30 * time.Second // Cap at 30 seconds
			}
			log.Printf("Failed to connect to RabbitMQ (attempt %d/%d): %v, retrying in %v...",
				attempt+1, maxRetries, err, delay)
			time.Sleep(delay)
		}

		if err != nil {
			log.Printf("Warning: Failed to connect to RabbitMQ after %d attempts: %v (job queue disabled)", maxRetries, lastErr)
			jobQueue = nil
		}
	} else {
		log.Println("RABBITMQ_URL not configured - job queue disabled")
	}

	// Initialize repositories
	todoRepo := database.NewTodoRepository(db)
	oidcConfigRepo := database.NewOIDCConfigRepository(db)
	contextRepo := database.NewAIContextRepository(db)
	activityRepo := database.NewUserActivityRepository(db)

	// Initialize services
	oidcProvider := oidc.NewProvider(oidcConfigRepo)
	jwksManager := oidc.NewJWKSManager()

	// Initialize AI provider
	aiProvider, err := createAIProvider(cfg)
	if err != nil {
		log.Printf("Warning: Failed to create AI provider: %v (AI features will be disabled)", err)
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
	var todoHandler *handlers.TodoHandler
	if jobQueue != nil {
		todoHandler = handlers.NewTodoHandlerWithQueue(todoRepo, jobQueue)
	} else {
		todoHandler = handlers.NewTodoHandler(todoRepo)
	}
	healthChecker := handlers.NewHealthChecker(db)

	var chatHandler *handlers.ChatHandler
	if chatService != nil && contextService != nil {
		chatHandler = handlers.NewChatHandler(chatService, contextService, contextRepo)
	}

	// Setup router
	r := mux.NewRouter()

	// Apply middleware (order matters - executed in reverse order of registration)
	// Note: In gorilla/mux, middleware executes in reverse order of registration
	// Middleware registered LAST executes FIRST (outermost wrapper)
	log.Println("Setting up middleware...")

	// Outermost middleware (executes first):
	// 1. Security headers (should be set on all responses)
	r.Use(middleware.SecurityHeaders(cfg.EnableHSTS))
	// 2. CORS (handles preflight requests)
	corsMW := middleware.CORSFromEnv(cfg.FrontendURL)
	r.Use(corsMW)
	// 3. Request size limits (protects against DoS)
	r.Use(middleware.MaxRequestSize(middleware.DefaultMaxRequestSize))
	// 4. Content-Type validation for POST/PATCH/PUT requests
	r.Use(middleware.ContentType)
	// 5. Request timeout (30 seconds default)
	r.Use(middleware.Timeout(30 * time.Second))
	// 6. Error handler (catches panics)
	r.Use(middleware.ErrorHandler)
	// 7. Audit logging (for security events)
	r.Use(middleware.Audit)
	// 8. Logging (innermost, executes last before handler)
	r.Use(middleware.Logging)
	// 9. Activity tracking (for authenticated requests)
	r.Use(middleware.ActivityTracking(activityRepo))

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
	loginRouter.Use(middleware.RateLimitUnauthenticated(redisLimiter))
	loginRouter.HandleFunc("/login", authHandler.GetOIDCLogin).Methods("GET")

	// Protected auth routes
	protectedAuthRouter := authRouter.PathPrefix("").Subrouter()
	protectedAuthRouter.Use(middleware.Auth(db, oidcProvider, jwksManager, cfg.OIDCProvider))
	protectedAuthRouter.Use(middleware.RateLimitAuthenticated(redisLimiter))
	protectedAuthRouter.HandleFunc("/me", authHandler.GetMe).Methods("GET")

	// Todo routes (protected)
	todosRouter := apiRouter.PathPrefix("/todos").Subrouter()
	todosRouter.Use(middleware.Auth(db, oidcProvider, jwksManager, cfg.OIDCProvider))
	todosRouter.Use(middleware.RateLimitAuthenticated(redisLimiter))
	todoHandler.RegisterRoutes(todosRouter)

	// AI routes (protected)
	if chatHandler != nil {
		aiRouter := apiRouter.PathPrefix("/ai").Subrouter()
		aiRouter.Use(middleware.Auth(db, oidcProvider, jwksManager, cfg.OIDCProvider))
		aiRouter.Use(middleware.RateLimitAuthenticated(redisLimiter))
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

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Server shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// createAIProvider creates an AI provider based on configuration
func createAIProvider(cfg *config.Config) (ai.AIProvider, error) {
	if cfg.OpenAIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

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

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		log.Printf("Failed to write health check response: %v", err)
	}
}

func versionInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Only expose minimal version info (sanitized for security)
	if _, err := fmt.Fprintf(w, `{"version":"1.0.0","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		log.Printf("Failed to write version info response: %v", err)
	}
}
