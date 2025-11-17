package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fortuna/services/game-stats-service/internal/cache"
	"github.com/fortuna/services/game-stats-service/internal/poller"
	"github.com/fortuna/services/game-stats-service/internal/providers/espn"
	"github.com/fortuna/services/game-stats-service/internal/publisher"
	"github.com/fortuna/services/game-stats-service/internal/registry"
	"github.com/redis/go-redis/v9"
)

func main() {
	log.Println("Starting Game Stats Service...")

	// Load configuration from environment
	redisURL := getEnv("REDIS_URL", "redis://localhost:6380")

	// Initialize Redis client
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}

	redisClient := redis.NewClient(opts)
	defer redisClient.Close()

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	// Initialize components
	sportRegistry := registry.New()
	espnClient := espn.New()
	cacheWriter := cache.NewRedisWriter(redisClient)
	streamPublisher := publisher.NewStreamPublisher(redisClient)

	// Create orchestrator
	orch := poller.NewOrchestrator(sportRegistry, espnClient, cacheWriter, streamPublisher)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal")
		cancel()
	}()

	// Start polling
	log.Println("Starting sport pollers...")
	orch.Start(ctx)

	log.Println("Game Stats Service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}




