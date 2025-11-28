package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/XavierBriggs/fortuna/services/opportunity-analytics/internal/aggregator"
	"github.com/XavierBriggs/fortuna/services/opportunity-analytics/internal/consumer"
	"github.com/XavierBriggs/fortuna/services/opportunity-analytics/internal/handlers"
	"github.com/XavierBriggs/fortuna/services/opportunity-analytics/internal/writer"
)

func main() {
	fmt.Println("📊 Opportunity Analytics Service Starting...")

	// Load configuration from environment
	config := loadConfig()

	// Initialize database
	holocronDB, err := sql.Open("postgres", config.HolocronDSN)
	if err != nil {
		fmt.Printf("Failed to connect to Holocron: %v\n", err)
		os.Exit(1)
	}
	defer holocronDB.Close()

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisURL,
		Password: config.RedisPassword,
		DB:       0,
	})
	defer redisClient.Close()

	// Test connections
	if err := holocronDB.Ping(); err != nil {
		fmt.Printf("Failed to ping Holocron: %v\n", err)
		os.Exit(1)
	}

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		fmt.Printf("Failed to ping Redis: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Database connections established")

	// Initialize components
	agg := aggregator.NewAggregator(config.BucketResolution, config.ExcludeLiveGames)
	holocronWriter := writer.NewHolocronWriter(holocronDB)
	holocronWriter.SetExcludeLive(config.ExcludeLiveGames) // Pass config to writer
	streamConsumer := consumer.NewStreamConsumer(redisClient, config.ConsumerGroup, config.ConsumerID)

	// Initialize consumer group
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := streamConsumer.Initialize(ctx, config.StreamKey); err != nil {
		fmt.Printf("Failed to initialize consumer: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Components initialized")

	// Start HTTP server in goroutine
	handler := handlers.NewHandler(holocronWriter)
	router := handler.SetupRouter()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.Port),
		Handler: router,
	}

	go func() {
		fmt.Printf("🚀 HTTP Server listening on :%s\n", config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start opportunity consumer goroutine
	go consumeOpportunities(ctx, streamConsumer, agg, config)

	// Start periodic flush goroutine
	go periodicFlush(ctx, agg, holocronWriter, config.FlushInterval)

	fmt.Println("🔥 Opportunity Analytics Service Running")
	fmt.Println("   Stream:", config.StreamKey)
	fmt.Println("   Consumer Group:", config.ConsumerGroup)
	fmt.Println("   Consumer ID:", config.ConsumerID)
	fmt.Println("   Bucket Resolution:", config.BucketResolution)
	fmt.Println("   Flush Interval:", config.FlushInterval)
	fmt.Printf("   Exclude Live Games: %v\n", config.ExcludeLiveGames)
	fmt.Println()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n🛑 Shutting down gracefully...")

	// Cancel context to stop goroutines
	cancel()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("HTTP server shutdown error: %v\n", err)
	}

	// Final flush before exit
	fmt.Println("Performing final flush...")
	buckets := agg.GetAndClearBuckets()
	stats := agg.ConvertToBookStats(buckets)
	if len(stats) > 0 {
		if err := holocronWriter.UpsertBookStats(context.Background(), stats); err != nil {
			fmt.Printf("Failed final flush: %v\n", err)
		} else {
			fmt.Printf("✓ Flushed %d stat buckets\n", len(stats))
		}
	}

	fmt.Println("✓ Shutdown complete")
}

// consumeOpportunities continuously consumes opportunities from the stream
func consumeOpportunities(ctx context.Context, streamConsumer *consumer.StreamConsumer, agg *aggregator.Aggregator, config Config) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			opportunities, err := streamConsumer.ReadMessages(ctx, config.StreamKey, 100, 1*time.Second)
			if err != nil {
				fmt.Printf("Error reading messages: %v\n", err)
				continue
			}

			if len(opportunities) > 0 {
				fmt.Printf("📥 Received %d opportunities\n", len(opportunities))

				// Process opportunities with event status from the opportunity itself
				for _, opp := range opportunities {
					// Event status flows through the pipeline from Mercury → Normalizer → Edge-detector
					gameStatus := opp.EventStatus
					if gameStatus == "" {
						gameStatus = "upcoming" // Default if not set
					}
					agg.ProcessOpportunity(opp, gameStatus)
				}
				fmt.Printf("📊 %s\n", agg.Stats())
			}
		}
	}
}

// periodicFlush periodically flushes aggregated stats to the database
func periodicFlush(ctx context.Context, agg *aggregator.Aggregator, holocronWriter *writer.HolocronWriter, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Flush book stats
			buckets := agg.GetAndClearBuckets()
			if len(buckets) > 0 {
				stats := agg.ConvertToBookStats(buckets)
				fmt.Printf("💾 Flushing %d stat buckets to database...\n", len(stats))

				if err := holocronWriter.UpsertBookStats(ctx, stats); err != nil {
					fmt.Printf("❌ Failed to flush stats: %v\n", err)
				} else {
					fmt.Printf("✓ Flushed %d stat buckets successfully\n", len(stats))
				}
			}

			// Flush book pair stats (for scalps/middles)
			pairBuckets := agg.GetAndClearBookPairBuckets()
			if len(pairBuckets) > 0 {
				pairStats := agg.ConvertToBookPairStats(pairBuckets)
				fmt.Printf("💾 Flushing %d book pair buckets to database...\n", len(pairStats))

				if err := holocronWriter.UpsertBookPairStats(ctx, pairStats); err != nil {
					fmt.Printf("❌ Failed to flush book pair stats: %v\n", err)
				} else {
					fmt.Printf("✓ Flushed %d book pair buckets successfully\n", len(pairStats))
				}
			}
		}
	}
}

// Config holds service configuration
type Config struct {
	HolocronDSN      string
	RedisURL         string
	RedisPassword    string
	StreamKey        string
	ConsumerGroup    string
	ConsumerID       string
	FlushInterval    time.Duration
	BucketResolution time.Duration
	Port             string
	ExcludeLiveGames bool
}

// loadConfig loads configuration from environment variables
func loadConfig() Config {
	return Config{
		HolocronDSN:      getEnv("HOLOCRON_DSN", "postgres://fortuna:fortuna_dev_password@localhost:5436/holocron?sslmode=disable"),
		RedisURL:         getEnv("REDIS_URL", "localhost:6380"),
		RedisPassword:    getEnv("REDIS_PASSWORD", "reddis_pw"),
		StreamKey:        getEnv("STREAM_KEY", "opportunities.detected"),
		ConsumerGroup:    getEnv("CONSUMER_GROUP", "opportunity-analytics"),
		ConsumerID:       getEnv("CONSUMER_ID", "analytics-1"),
		FlushInterval:    parseDuration(getEnv("FLUSH_INTERVAL", "60s")),
		BucketResolution: parseDuration(getEnv("BUCKET_RESOLUTION", "5m")),
		Port:             getEnv("PORT", "8091"),
		ExcludeLiveGames: getEnv("EXCLUDE_LIVE_GAMES", "true") == "true",
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseDuration parses a duration string with a default fallback
func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 60 * time.Second
	}
	return d
}
