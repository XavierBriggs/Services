package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XavierBriggs/fortuna/services/alert-service/internal/consumer"
	"github.com/XavierBriggs/fortuna/services/alert-service/internal/dedup"
	"github.com/XavierBriggs/fortuna/services/alert-service/internal/filter"
	"github.com/XavierBriggs/fortuna/services/alert-service/internal/notifier"
	"github.com/XavierBriggs/fortuna/services/alert-service/internal/ratelimit"
	"github.com/redis/go-redis/v9"
)

func main() {
	fmt.Println("=== Fortuna Alert Service v0 ===")

	// Load configuration
	config := loadConfig()

	// Validate webhook URL
	if config.SlackWebhookURL == "" {
		fmt.Println("⚠️  WARNING: SLACK_WEBHOOK_URL not set - alerts will be logged but not sent")
	}

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

	// Initialize components
	streamConsumer := consumer.NewStreamConsumer(redisClient, config.ConsumerID, config.GroupName)
	alertFilter := filter.NewFilter(config.MinEdgePercent, config.MaxDataAgeSeconds)
	deduplicator := dedup.NewDeduplicator(redisClient, config.DedupTTLMinutes)
	rateLimiter := ratelimit.NewTokenBucket(redisClient, config.AlertRateLimit)
	slackNotifier := notifier.NewSlackNotifier(config.SlackWebhookURL)

	fmt.Printf("✓ Alert Service configured:\n")
	fmt.Printf("  Min Edge: %.1f%%\n", config.MinEdgePercent)
	fmt.Printf("  Max Data Age: %ds\n", config.MaxDataAgeSeconds)
	fmt.Printf("  Rate Limit: %d alerts/min\n", config.AlertRateLimit)
	fmt.Printf("  Dedup TTL: %d minutes\n", config.DedupTTLMinutes)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start alert processor
	alertCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start processing in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- processAlerts(alertCtx, streamConsumer, alertFilter, deduplicator, rateLimiter, slackNotifier)
	}()

	// Start metrics reporter
	alertsSent := int64(0)
	alertsFiltered := int64(0)
	alertsRateLimited := int64(0)
	totalLatencyMs := int64(0)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-alertCtx.Done():
				return
			case <-ticker.C:
				avgLatency := float64(0)
				if alertsSent > 0 {
					avgLatency = float64(totalLatencyMs) / float64(alertsSent)
				}
				fmt.Printf("📊 Metrics: sent=%d filtered=%d rate_limited=%d avg_latency=%.1fms\n",
					alertsSent, alertsFiltered, alertsRateLimited, avgLatency)
			}
		}
	}()

	fmt.Println("✓ Alert Service started - monitoring opportunities")

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		fmt.Printf("\n⚠️  Received signal: %v\n", sig)
		cancel()
	case err := <-errChan:
		if err != nil {
			fmt.Printf("❌ Alert processor error: %v\n", err)
			os.Exit(1)
		}
	}

	// Graceful shutdown
	fmt.Println("🛑 Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	<-shutdownCtx.Done()

	if err := redisClient.Close(); err != nil {
		fmt.Printf("⚠️  Error closing Redis: %v\n", err)
	}

	fmt.Println("✓ Shutdown complete")
}

// processAlerts processes opportunities and sends alerts
func processAlerts(
	ctx context.Context,
	consumer *consumer.StreamConsumer,
	filter *filter.Filter,
	dedup *dedup.Deduplicator,
	rateLimiter *ratelimit.TokenBucket,
	notifier *notifier.SlackNotifier,
) error {
	streamKey := "opportunities.detected"

	messageCh, errorCh := consumer.ConsumeStream(ctx, streamKey)

	for {
		select {
		case <-ctx.Done():
			return nil

		case err := <-errorCh:
			if err != nil {
				fmt.Printf("stream error: %v\n", err)
			}

		case msg, ok := <-messageCh:
			if !ok {
				return nil
			}

			startTime := time.Now()
			opp := msg.Opportunity

			// Filter check
			shouldAlert, reason := filter.ShouldAlert(opp)
			if !shouldAlert {
				fmt.Printf("⊘ Filtered opportunity %d: %s\n", opp.ID, reason)
				consumer.AckMessage(ctx, msg.StreamKey, msg.ID)
				continue
			}

			// Deduplication check
			shouldAlert, err := dedup.ShouldAlert(ctx, opp)
			if err != nil {
				fmt.Printf("error checking dedup: %v\n", err)
			}
			if !shouldAlert {
				fmt.Printf("⊘ Duplicate opportunity %d\n", opp.ID)
				consumer.AckMessage(ctx, msg.StreamKey, msg.ID)
				continue
			}

			// Rate limit check
			allowed, err := rateLimiter.AllowAlert(ctx)
			if err != nil {
				fmt.Printf("error checking rate limit: %v\n", err)
			}
			if !allowed {
				fmt.Printf("⊘ Rate limited opportunity %d\n", opp.ID)
				consumer.AckMessage(ctx, msg.StreamKey, msg.ID)
				continue
			}

			// Send alert
			if err := notifier.SendAlert(ctx, opp); err != nil {
				fmt.Printf("error sending alert: %v\n", err)
			} else {
				latency := time.Since(startTime).Milliseconds()
				fmt.Printf("✓ Alert sent for opportunity %d (latency=%dms)\n", opp.ID, latency)
			}

			// Acknowledge message
			consumer.AckMessage(ctx, msg.StreamKey, msg.ID)
		}
	}
}

// Config holds alert service configuration
type Config struct {
	RedisURL          string
	RedisPassword     string
	SlackWebhookURL   string
	ConsumerID        string
	GroupName         string
	MinEdgePercent    float64
	MaxDataAgeSeconds int
	AlertRateLimit    int
	DedupTTLMinutes   int
}

// loadConfig loads configuration from environment variables
func loadConfig() Config {
	return Config{
		RedisURL:          getEnv("REDIS_URL", "localhost:6380"),
		RedisPassword:     os.Getenv("REDIS_PASSWORD"),
		SlackWebhookURL:   os.Getenv("SLACK_WEBHOOK_URL"),
		ConsumerID:        getEnv("ALERT_SERVICE_CONSUMER_ID", "alert-service-1"),
		GroupName:         getEnv("ALERT_SERVICE_GROUP_NAME", "alert-services"),
		MinEdgePercent:    getEnvFloat("ALERT_MIN_EDGE_PCT", 1.0),
		MaxDataAgeSeconds: getEnvInt("ALERT_MAX_DATA_AGE_SECONDS", 10),
		AlertRateLimit:    getEnvInt("ALERT_RATE_LIMIT", 10),
		DedupTTLMinutes:   getEnvInt("ALERT_DEDUP_TTL_MINUTES", 5),
	}
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		fmt.Sscanf(value, "%d", &intValue)
		return intValue
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		var floatValue float64
		fmt.Sscanf(value, "%f", &floatValue)
		return floatValue
	}
	return defaultValue
}

