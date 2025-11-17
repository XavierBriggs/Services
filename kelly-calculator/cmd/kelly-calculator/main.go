package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/XavierBriggs/fortuna/services/kelly-calculator/internal/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	// Load configuration
	config := loadConfig()

	// Create handler
	handler := handlers.NewHandler(
		config.DefaultBankroll,
		config.KellyFraction,
		config.MinEdge,
		config.MaxPct,
	)

	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:3001"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Routes
	r.Get("/health", handler.HealthCheck)
	r.Post("/api/v1/calculate-from-opportunity", handler.CalculateFromOpportunity)

	// Start server
	addr := fmt.Sprintf(":%d", config.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("✓ Kelly Calculator started on port %d\n", config.Port)
		fmt.Printf("  Default Bankroll: $%.2f\n", config.DefaultBankroll)
		fmt.Printf("  Kelly Fraction: %.2f (1/%.0f Kelly)\n", config.KellyFraction, 1.0/config.KellyFraction)
		fmt.Printf("  Min Edge: %.1f%%\n", config.MinEdge*100)
		fmt.Printf("  Max Stake: %.1f%% of bankroll\n", config.MaxPct*100)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("✗ Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n✓ Shutting down gracefully...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		fmt.Printf("✗ Shutdown error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Kelly Calculator stopped")
}

// Config holds service configuration
type Config struct {
	Port            int
	DefaultBankroll float64
	KellyFraction   float64
	MinEdge         float64
	MaxPct          float64
}

// loadConfig loads configuration from environment
func loadConfig() Config {
	return Config{
		Port:            getEnvInt("KELLY_SERVICE_PORT", 8084),
		DefaultBankroll: getEnvFloat("DEFAULT_BANKROLL", 10000.0),
		KellyFraction:   getEnvFloat("KELLY_DEFAULT_FRACTION", 0.25),
		MinEdge:         getEnvFloat("KELLY_MIN_EDGE_PCT", 1.0) / 100.0, // Convert to decimal
		MaxPct:          getEnvFloat("KELLY_MAX_PCT", 10.0) / 100.0,     // Convert to decimal
	}
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}




