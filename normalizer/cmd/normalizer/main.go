package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XavierBriggs/fortuna/services/normalizer/internal/consumer"
	"github.com/XavierBriggs/fortuna/services/normalizer/internal/processor"
	"github.com/XavierBriggs/fortuna/services/normalizer/internal/publisher"
	"github.com/XavierBriggs/fortuna/services/normalizer/internal/registry"
	"github.com/XavierBriggs/fortuna/services/normalizer/sports/basketball_nba"
	"github.com/redis/go-redis/v9"
)

func main() {
	fmt.Println("=== Fortuna Normalizer v0 ===")

	// Load configuration
	config := loadConfig()

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisURL,
		Password: config.RedisPassword,
		DB:       0,
	})

	// Ping Redis
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		fmt.Printf("❌ Failed to connect to Redis: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Redis")

	// Initialize sport registry
	normalizerRegistry := registry.NewNormalizerRegistry()

	// Register NBA normalizer
	nbaModule := basketball_nba.NewNormalizer()
	if err := normalizerRegistry.Register(nbaModule); err != nil {
		fmt.Printf("❌ Failed to register NBA normalizer: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Registered NBA normalizer\n")

	// Initialize components
	streamConsumer := consumer.NewStreamConsumer(redisClient, config.ConsumerID, config.GroupName)
	streamPublisher := publisher.NewStreamPublisher(redisClient)
	proc := processor.NewProcessor(streamConsumer, streamPublisher, normalizerRegistry)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start processing
	processCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start processor in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- proc.Start(processCtx)
	}()

	// Start metrics reporter
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-processCtx.Done():
				return
			case <-ticker.C:
				processed, errors := proc.GetMetrics()
				fmt.Printf("📊 Metrics: processed=%d errors=%d\n", processed, errors)
			}
		}
	}()

	fmt.Println("✓ Normalizer started - processing odds")
	fmt.Printf("  Consumer ID: %s\n", config.ConsumerID)
	fmt.Printf("  Group Name: %s\n", config.GroupName)
	fmt.Printf("  Registered Sports: %d\n", normalizerRegistry.Count())

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		fmt.Printf("\n⚠️  Received signal: %v\n", sig)
		cancel()
	case err := <-errChan:
		if err != nil {
			fmt.Printf("❌ Processor error: %v\n", err)
			os.Exit(1)
		}
	}

	// Graceful shutdown
	fmt.Println("🛑 Shutting down gracefully...")
	
	// Give processes time to finish current work
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	<-shutdownCtx.Done()

	// Close Redis connection
	if err := redisClient.Close(); err != nil {
		fmt.Printf("⚠️  Error closing Redis: %v\n", err)
	}

	fmt.Println("✓ Shutdown complete")
}

// Config holds normalizer configuration
type Config struct {
	RedisURL      string
	RedisPassword string
	ConsumerID    string
	GroupName     string
}

// loadConfig loads configuration from environment variables
func loadConfig() Config {
	return Config{
		RedisURL:      getEnv("REDIS_URL", "localhost:6380"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		ConsumerID:    getEnv("NORMALIZER_CONSUMER_ID", "normalizer-1"),
		GroupName:     getEnv("NORMALIZER_GROUP_NAME", "normalizers"),
	}
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}





