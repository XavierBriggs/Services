package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/XavierBriggs/fortuna/services/bot-service/internal/client"
	"github.com/XavierBriggs/fortuna/services/bot-service/internal/executor"
	"github.com/XavierBriggs/fortuna/services/bot-service/internal/handler"
	"github.com/XavierBriggs/fortuna/services/bot-service/internal/logger"
	"github.com/XavierBriggs/fortuna/services/bot-service/internal/retry"
	"github.com/XavierBriggs/fortuna/services/bot-service/internal/transformer"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/lib/pq"
)

func main() {
	fmt.Println("=== Fortuna Bot Service v0 ===")

	// Load configuration
	config := loadConfig()

	// Connect to Holocron DB
	holocronDB, err := sql.Open("postgres", config.HolocronDSN)
	if err != nil {
		fmt.Printf("❌ Failed to connect to Holocron: %v\n", err)
		os.Exit(1)
	}
	defer holocronDB.Close()

	// Configure connection pool
	holocronDB.SetMaxOpenConns(25)
	holocronDB.SetMaxIdleConns(5)
	holocronDB.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := holocronDB.PingContext(ctx); err != nil {
		fmt.Printf("❌ Failed to ping Holocron: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Holocron DB")

	// Connect to Alexandria DB
	alexandriaDB, err := sql.Open("postgres", config.AlexandriaDSN)
	if err != nil {
		fmt.Printf("❌ Failed to connect to Alexandria: %v\n", err)
		os.Exit(1)
	}
	defer alexandriaDB.Close()

	alexandriaDB.SetMaxOpenConns(25)
	alexandriaDB.SetMaxIdleConns(5)
	alexandriaDB.SetConnMaxLifetime(5 * time.Minute)

	if err := alexandriaDB.PingContext(ctx); err != nil {
		fmt.Printf("❌ Failed to ping Alexandria: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Alexandria DB")

	// Connect to Atlas DB (for team mappings)
	var atlasDB *sql.DB
	if config.AtlasDSN != "" {
		atlasDB, err = sql.Open("postgres", config.AtlasDSN)
		if err != nil {
			fmt.Printf("⚠️  Failed to connect to Atlas: %v (will use fallback)\n", err)
		} else {
			atlasDB.SetMaxOpenConns(10)
			atlasDB.SetMaxIdleConns(2)
			atlasDB.SetConnMaxLifetime(5 * time.Minute)
			if err := atlasDB.PingContext(ctx); err != nil {
				fmt.Printf("⚠️  Failed to ping Atlas: %v (will use fallback)\n", err)
				atlasDB = nil
			} else {
				fmt.Println("✓ Connected to Atlas DB")
			}
		}
	}

	// Initialize components
	talosClient := client.NewTalosClient(config.TalosBotManagerURL, nil)
	transformer := transformer.NewTransformer(alexandriaDB, atlasDB)
	betLogger := logger.NewBetLogger(holocronDB)
	retryPolicy := retry.NewRetryPolicy(config.RetryMaxAttempts, config.RetryInitialDelay)
	
	executor := executor.NewExecutor(
		talosClient,
		transformer,
		betLogger,
		retryPolicy,
		holocronDB,
		alexandriaDB,
	)

	// Setup HTTP handler
	botHandler := handler.NewBotHandler(executor, talosClient, holocronDB)

	// Setup router
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(150 * time.Second)) // Long timeout for bot execution

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:3001"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Routes
	r.Get("/health", botHandler.HealthCheck)
	r.Post("/api/v1/place-bet", botHandler.PlaceBet)
	r.Get("/api/v1/bot-status", botHandler.GetBotStatus)
	r.Get("/api/v1/bots/status", botHandler.GetBotsStatus)
	r.Get("/api/v1/bots/bets/recent", botHandler.GetRecentBets)

	// Start HTTP server
	addr := fmt.Sprintf(":%d", config.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  150 * time.Second, // Long timeout for bot execution
		WriteTimeout: 150 * time.Second,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("🚀 Bot Service listening on :%d\n", config.Port)
		fmt.Printf("   Talos Bot Manager: %s\n", config.TalosBotManagerURL)
		fmt.Printf("   Retry Policy: %d attempts, %v initial delay\n", config.RetryMaxAttempts, config.RetryInitialDelay)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n🛑 Shutting down gracefully...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("❌ Shutdown error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Bot Service stopped")
}

// Config holds service configuration
type Config struct {
	Port              int
	TalosBotManagerURL string
	HolocronDSN       string
	AlexandriaDSN     string
	AtlasDSN          string
	RetryMaxAttempts  int
	RetryInitialDelay time.Duration
}

// loadConfig loads configuration from environment
func loadConfig() Config {
	return Config{
		Port:               getEnvInt("BOT_SERVICE_PORT", 8090),
		TalosBotManagerURL: getEnv("TALOS_BOT_MANAGER_URL", "http://localhost:5000"),
		HolocronDSN:        getEnv("HOLOCRON_DSN", ""),
		AlexandriaDSN:      getEnv("ALEXANDRIA_DSN", ""),
		AtlasDSN:           getEnv("ATLAS_DSN", ""), // Optional - for team mappings
		RetryMaxAttempts:   getEnvInt("RETRY_MAX_ATTEMPTS", 3),
		RetryInitialDelay:  getEnvDuration("RETRY_INITIAL_DELAY", 1*time.Second),
	}
}

func getEnv(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

