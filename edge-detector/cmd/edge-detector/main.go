package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XavierBriggs/fortuna/services/edge-detector/internal/consumer"
	"github.com/XavierBriggs/fortuna/services/edge-detector/internal/detector"
	"github.com/XavierBriggs/fortuna/services/edge-detector/internal/publisher"
	"github.com/XavierBriggs/fortuna/services/edge-detector/internal/writer"
	"github.com/XavierBriggs/fortuna/services/edge-detector/sports/basketball_nba"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	fmt.Println("=== Fortuna Edge Detector v0 ===")

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

	// Connect to Holocron DB
	holocronDB, err := sql.Open("postgres", config.HolocronDSN)
	if err != nil {
		fmt.Printf("❌ Failed to connect to Holocron: %v\n", err)
		os.Exit(1)
	}
	defer holocronDB.Close()

	// Ping Holocron
	if err := holocronDB.PingContext(ctx); err != nil {
		fmt.Printf("❌ Failed to ping Holocron: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Holocron DB")

	// Connect to Alexandria DB (for sharp book detection)
	alexandriaDB, err := sql.Open("postgres", config.AlexandriaDSN)
	if err != nil {
		fmt.Printf("❌ Failed to connect to Alexandria: %v\n", err)
		os.Exit(1)
	}
	defer alexandriaDB.Close()

	// Ping Alexandria
	if err := alexandriaDB.PingContext(ctx); err != nil {
		fmt.Printf("❌ Failed to ping Alexandria: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Alexandria DB")

	// Initialize NBA configuration
	nbaConfig := basketball_nba.NewConfig()
	fmt.Printf("✓ NBA Config loaded: min_edge=%.1f%%, max_age=%ds\n",
		nbaConfig.MinEdgePct*100, nbaConfig.MaxDataAgeSeconds)

	// Initialize components
	streamConsumer := consumer.NewStreamConsumer(redisClient, config.ConsumerID, config.GroupName)
	holocronWriter := writer.NewHolocronWriter(holocronDB)
	streamPublisher := publisher.NewStreamPublisher(redisClient)

	// Initialize sharp book provider for NBA
	sharpBookProvider := basketball_nba.NewSharpBookProvider(alexandriaDB, nbaConfig.GetSharpBooks())

	// Initialize detection engine
	detectionEngine := detector.NewEngine(
		streamConsumer,
		holocronWriter,
		streamPublisher,
		sharpBookProvider,
		nbaConfig,
	)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start detection engine
	detectCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start engine in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- detectionEngine.Start(detectCtx, "basketball_nba")
	}()

	// Start metrics reporter
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-detectCtx.Done():
				return
			case <-ticker.C:
				detected, errors := detectionEngine.GetMetrics()
				avgTotal, avgDetection := detectionEngine.GetLatencyMetrics()
				fmt.Printf("📊 Metrics: detected=%d errors=%d avg_latency=%.1fms (detection=%.1fms)\n", 
					detected, errors, avgTotal, avgDetection)
			}
		}
	}()

	fmt.Println("✓ Edge Detector started - monitoring normalized odds")
	fmt.Printf("  Consumer ID: %s\n", config.ConsumerID)
	fmt.Printf("  Group Name: %s\n", config.GroupName)
	fmt.Printf("  Sports: basketball_nba\n")
	fmt.Printf("  Enabled Detectors: edge=%t, middle=%t, scalp=%t\n",
		true, nbaConfig.EnableMiddles, nbaConfig.EnableScalps)

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		fmt.Printf("\n⚠️  Received signal: %v\n", sig)
		cancel()
	case err := <-errChan:
		if err != nil {
			fmt.Printf("❌ Detection engine error: %v\n", err)
			os.Exit(1)
		}
	}

	// Graceful shutdown
	fmt.Println("🛑 Shutting down gracefully...")

	// Give processes time to finish current work
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	<-shutdownCtx.Done()

	// Close connections
	if err := redisClient.Close(); err != nil {
		fmt.Printf("⚠️  Error closing Redis: %v\n", err)
	}

	fmt.Println("✓ Shutdown complete")
}

// Config holds edge detector configuration
type Config struct {
	RedisURL       string
	RedisPassword  string
	HolocronDSN    string
	AlexandriaDSN  string
	ConsumerID     string
	GroupName      string
}

// loadConfig loads configuration from environment variables
func loadConfig() Config {
	return Config{
		RedisURL:       getEnv("REDIS_URL", "localhost:6380"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		HolocronDSN:    getEnv("HOLOCRON_DSN", "postgres://fortuna:fortuna_pw@localhost:5436/holocron?sslmode=disable"),
		AlexandriaDSN:  getEnv("ALEXANDRIA_DSN", "postgres://fortuna:fortuna_pw@localhost:5432/alexandria?sslmode=disable"),
		ConsumerID:     getEnv("EDGE_DETECTOR_CONSUMER_ID", "edge-detector-1"),
		GroupName:      getEnv("EDGE_DETECTOR_GROUP_NAME", "edge-detectors"),
	}
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

