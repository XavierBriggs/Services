package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XavierBriggs/fortuna/services/api-gateway/internal/db"
	"github.com/XavierBriggs/fortuna/services/api-gateway/internal/handlers"
	"github.com/XavierBriggs/fortuna/services/api-gateway/internal/middleware"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	fmt.Println("=== Fortuna API Gateway v0 ===")

	// Load configuration
	config := loadConfig()

	// Connect to Alexandria DB
	dbClient, err := db.NewClient(config.AlexandriaDSN)
	if err != nil {
		fmt.Printf("❌ Failed to connect to Alexandria: %v\n", err)
		os.Exit(1)
	}
	defer dbClient.Close()

	fmt.Println("✓ Connected to Alexandria DB")

	// Initialize handlers
	handler := handlers.NewHandler(dbClient)

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   config.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Routes
	r.Get("/health", handler.HealthCheck)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Events
		r.Get("/events", handler.GetEvents)
		r.Get("/events/{eventID}", handler.GetEvent)
		r.Get("/events/{eventID}/odds", handler.GetEventWithOdds)

		// Odds
		r.Get("/odds/current", handler.GetCurrentOdds)
		r.Get("/odds/history", handler.GetOddsHistory)
	})

	// Start server
	srv := &http.Server{
		Addr:         config.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	serverErrors := make(chan error, 1)
	go func() {
		fmt.Printf("✓ API Gateway listening on %s\n", config.Port)
		fmt.Println("  Endpoints:")
		fmt.Println("    GET  /health")
		fmt.Println("    GET  /api/v1/events")
		fmt.Println("    GET  /api/v1/events/{eventID}")
		fmt.Println("    GET  /api/v1/events/{eventID}/odds")
		fmt.Println("    GET  /api/v1/odds/current")
		fmt.Println("    GET  /api/v1/odds/history")

		serverErrors <- srv.ListenAndServe()
	}()

	// Wait for interrupt signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		fmt.Printf("❌ Server error: %v\n", err)
		os.Exit(1)

	case sig := <-shutdown:
		fmt.Printf("\n⚠️  Received signal: %v\n", sig)

		// Give outstanding requests a deadline for completion
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			fmt.Printf("⚠️  Graceful shutdown failed: %v\n", err)
			if err := srv.Close(); err != nil {
				fmt.Printf("❌ Could not stop server: %v\n", err)
			}
		}
	}

	fmt.Println("✓ Shutdown complete")
}

// Config holds application configuration
type Config struct {
	Port           string
	AlexandriaDSN  string
	CORSOrigins    []string
}

// loadConfig loads configuration from environment variables
func loadConfig() Config {
	return Config{
		Port:          getEnv("API_GATEWAY_PORT", ":8080"),
		AlexandriaDSN: getEnv("ALEXANDRIA_DSN", "postgres://fortuna_dev:fortuna_dev_password@localhost:5435/alexandria?sslmode=disable"),
		CORSOrigins:   []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"http://localhost:3002",
			"http://localhost:3003",
		},
	}
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

