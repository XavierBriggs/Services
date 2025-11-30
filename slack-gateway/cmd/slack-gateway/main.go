package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/XavierBriggs/fortuna/services/slack-gateway/internal/dedup"
	"github.com/XavierBriggs/fortuna/services/slack-gateway/internal/handler"
	"github.com/XavierBriggs/fortuna/services/slack-gateway/internal/logger"
	slackclient "github.com/XavierBriggs/fortuna/services/slack-gateway/internal/slack"
	"github.com/XavierBriggs/fortuna/services/slack-gateway/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

// Config holds all configuration for the slack-gateway
type Config struct {
	Port               string
	SlackBotToken      string
	SlackSigningSecret string
	RedisURL           string
	RedisPassword      string
	HolocronDSN        string
	AlexandriaDSN      string
	BetAPIURL          string
	LogLevel           string
}

func main() {
	fmt.Println("=== Fortuna Slack Gateway v1.0 ===")

	// Initialize logger
	log := logger.New("slack-gateway")
	log.Info("starting", map[string]interface{}{"version": "1.0.0"})

	// Load configuration from environment
	config := loadConfig()

	// Log Slack config (tokens have defaults hardcoded)
	fmt.Printf("✓ Slack Bot Token: %s...%s\n", config.SlackBotToken[:10], config.SlackBotToken[len(config.SlackBotToken)-4:])
	fmt.Printf("✓ Slack Signing Secret: %s...\n", config.SlackSigningSecret[:8])

	ctx := context.Background()

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisURL,
		Password: config.RedisPassword,
		DB:       0,
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		fmt.Printf("❌ Failed to connect to Redis at %s: %v\n", config.RedisURL, err)
		os.Exit(1)
	}
	defer redisClient.Close()
	fmt.Printf("✓ Connected to Redis at %s\n", config.RedisURL)

	// Connect to Holocron database (opportunities, preferences)
	holocronDB, err := sql.Open("postgres", config.HolocronDSN)
	if err != nil {
		fmt.Printf("❌ Failed to open Holocron DB: %v\n", err)
		os.Exit(1)
	}
	defer holocronDB.Close()

	holocronDB.SetMaxOpenConns(10)
	holocronDB.SetMaxIdleConns(5)
	holocronDB.SetConnMaxLifetime(5 * time.Minute)

	if err := holocronDB.PingContext(ctx); err != nil {
		fmt.Printf("❌ Failed to ping Holocron DB: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Holocron DB")

	// Connect to Alexandria database (events)
	alexandriaDB, err := sql.Open("postgres", config.AlexandriaDSN)
	if err != nil {
		fmt.Printf("❌ Failed to open Alexandria DB: %v\n", err)
		os.Exit(1)
	}
	defer alexandriaDB.Close()

	alexandriaDB.SetMaxOpenConns(5)
	alexandriaDB.SetMaxIdleConns(2)
	alexandriaDB.SetConnMaxLifetime(5 * time.Minute)

	if err := alexandriaDB.PingContext(ctx); err != nil {
		fmt.Printf("❌ Failed to ping Alexandria DB: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Alexandria DB")

	// Initialize Slack client
	slackClient := slackclient.NewClient(config.SlackBotToken, config.SlackSigningSecret)
	fmt.Println("✓ Slack client initialized")

	// Initialize stores
	prefStore := store.NewPreferenceStore(holocronDB, redisClient)
	oppStore := store.NewOpportunityStore(holocronDB)

	// Initialize deduplicator (0 = use default 10 minute TTL)
	deduplicator := dedup.NewDeduplicator(redisClient, 0)

	// Initialize handlers
	commandsHandler := handler.NewCommandsHandler(slackClient, prefStore, log)
	interactionsHandler := handler.NewInteractionsHandler(
		slackClient,
		prefStore,
		oppStore,
		deduplicator,
		log,
		config.BetAPIURL,
		alexandriaDB,
	)

	// Setup router
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(120 * time.Second))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		// Quick health check - verify DB connections
		if err := holocronDB.PingContext(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("holocron db unhealthy"))
			return
		}
		if err := redisClient.Ping(r.Context()).Err(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("redis unhealthy"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Slack endpoints
	r.Post("/slack/commands", commandsHandler.HandleCommands)
	r.Post("/slack/interactions", interactionsHandler.HandleInteractions)

	// Debug endpoint (for development)
	r.Get("/debug/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"redis_url":"%s","bet_api_url":"%s","port":"%s"}`,
			config.RedisURL, config.BetAPIURL, config.Port)
	})

	// Determine port (handle both ":8095" and "8095" formats)
	port := config.Port
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	// Start server
	server := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("✓ Slack Gateway starting on port %s\n", port)
		fmt.Printf("  Health: http://localhost%s/health\n", port)
		fmt.Printf("  Commands: POST /slack/commands\n")
		fmt.Printf("  Interactions: POST /slack/interactions\n")
		fmt.Printf("  Bet API: %s\n", config.BetAPIURL)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-done
	fmt.Println("\n⚠️  Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("❌ Server shutdown error: %v\n", err)
	}

	fmt.Println("✓ Slack Gateway stopped")
}

// loadConfig loads configuration from environment variables
func loadConfig() Config {
	return Config{
		Port:               getEnv("PORT", ":8095"),
		SlackBotToken:      getEnv("SLACK_BOT_TOKEN", "xoxb-9860285501522-9858354975574-PREPxxpnbLjEMqg0t8r4lTjO"),
		SlackSigningSecret: getEnv("SLACK_SIGNING_SECRET", "5431a8ca94843403a60f3ac33243553a"),
		RedisURL:           getEnv("REDIS_URL", "localhost:6380"),
		RedisPassword:      os.Getenv("REDIS_PASSWORD"),
		HolocronDSN:        getEnv("HOLOCRON_DSN", "postgres://fortuna:fortuna_dev_password@localhost:5436/holocron?sslmode=disable"),
		AlexandriaDSN:      getEnv("ALEXANDRIA_DSN", "postgres://fortuna:fortuna_dev_password@localhost:5435/alexandria?sslmode=disable"),
		BetAPIURL:          getEnv("BET_API_URL", "http://api-gateway:8080"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
	}
}

// getEnv gets an environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
