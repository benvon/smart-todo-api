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

	"github.com/gorilla/mux"
	"github.com/benvon/smart-todo/internal/config"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/handlers"
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/services/oidc"
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
	defer db.Close()

	// Initialize repositories
	todoRepo := database.NewTodoRepository(db)
	oidcConfigRepo := database.NewOIDCConfigRepository(db)

	// Initialize services
	oidcProvider := oidc.NewProvider(oidcConfigRepo)
	jwksManager := oidc.NewJWKSManager()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(oidcProvider)
	todoHandler := handlers.NewTodoHandler(todoRepo)
	healthChecker := handlers.NewHealthChecker(db)

	// Setup router
	r := mux.NewRouter()

	// Apply middleware (order matters - CORS should run first to handle preflight)
	// Note: In gorilla/mux, middleware executes in reverse order of registration
	// Middleware registered LAST executes FIRST (outermost wrapper)
	log.Println("Setting up middleware...")
	corsMW := middleware.CORSFromEnv(cfg.FrontendURL)
	r.Use(corsMW) // Executes FIRST (outermost) - handles OPTIONS preflight
	r.Use(middleware.ErrorHandler)
	r.Use(middleware.Logging) // Executes LAST (innermost)
	log.Println("Middleware setup complete - CORS middleware should handle all OPTIONS requests")

	// Public routes
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
	// Public auth routes
	authRouter.HandleFunc("/oidc/login", authHandler.GetOIDCLogin).Methods("GET")
	// Protected auth routes
	protectedAuthRouter := authRouter.PathPrefix("").Subrouter()
	protectedAuthRouter.Use(middleware.Auth(db, oidcProvider, jwksManager))
	protectedAuthRouter.HandleFunc("/me", authHandler.GetMe).Methods("GET")

	// Todo routes (protected)
	todosRouter := apiRouter.PathPrefix("/todos").Subrouter()
	todosRouter.Use(middleware.Auth(db, oidcProvider, jwksManager))
	todoHandler.RegisterRoutes(todosRouter)

	// Catch-all OPTIONS handler for preflight requests
	// This ensures OPTIONS requests are handled even if routes don't explicitly allow them
	// The CORS middleware will handle setting headers before this is called
	r.Methods("OPTIONS").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS middleware should have already set headers, just return 204
		w.WriteHeader(http.StatusNoContent)
	})

	// Setup server
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
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
	if _, err := fmt.Fprintf(w, `{"version":"1.0.0","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		log.Printf("Failed to write version info response: %v", err)
	}
}
