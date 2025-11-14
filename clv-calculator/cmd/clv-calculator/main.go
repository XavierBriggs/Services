package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XavierBriggs/fortuna/services/clv-calculator/internal/calculator"
	"github.com/XavierBriggs/fortuna/services/clv-calculator/internal/consumer"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	fmt.Println("=== CLV Calculator v0 ===")

	// Load config
	config := loadConfig()

	// Connect to Alexandria (for closing lines)
	alexandriaDB, err := sql.Open("postgres", config.AlexandriaDSN)
	if err != nil {
		fmt.Printf("❌ Failed to connect to Alexandria: %v\n", err)
		os.Exit(1)
	}
	defer alexandriaDB.Close()

	if err := alexandriaDB.Ping(); err != nil {
		fmt.Printf("❌ Failed to ping Alexandria: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Alexandria DB")

	// Connect to Holocron (for bets)
	holocronDB, err := sql.Open("postgres", config.HolocronDSN)
	if err != nil {
		fmt.Printf("❌ Failed to connect to Holocron: %v\n", err)
		os.Exit(1)
	}
	defer holocronDB.Close()

	if err := holocronDB.Ping(); err != nil {
		fmt.Printf("❌ Failed to ping Holocron: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Holocron DB")

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisURL,
		Password: config.RedisPassword,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		fmt.Printf("❌ Failed to connect to Redis: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Redis")

	// Initialize CLV calculator
	calc := calculator.NewCLVCalculator(alexandriaDB, holocronDB)

	// Initialize stream consumer
	streamConsumer := consumer.NewConsumer(
		redisClient,
		config.StreamName,
		config.ConsumerGroup,
		calc,
	)

	// Start consumer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		fmt.Printf("✓ CLV Calculator started\n")
		fmt.Printf("  Stream: %s\n", config.StreamName)
		fmt.Printf("  Consumer Group: %s\n", config.ConsumerGroup)
		if err := streamConsumer.Start(ctx); err != nil {
			fmt.Printf("❌ Consumer error: %v\n", err)
		}
	}()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n✓ Shutting down gracefully...")
	cancel()
	time.Sleep(2 * time.Second)
	fmt.Println("✓ CLV Calculator stopped")
}

type Config struct {
	AlexandriaDSN  string
	HolocronDSN    string
	RedisURL       string
	RedisPassword  string
	StreamName     string
	ConsumerGroup  string
}

func loadConfig() Config {
	return Config{
		AlexandriaDSN: getEnv("ALEXANDRIA_DSN", "postgres://fortuna:fortuna@localhost:5435/alexandria?sslmode=disable"),
		HolocronDSN:   getEnv("HOLOCRON_DSN", "postgres://fortuna:fortuna@localhost:5436/holocron?sslmode=disable"),
		RedisURL:      getEnv("REDIS_URL", "localhost:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		StreamName:    getEnv("CLV_STREAM", "closing_lines.captured"),
		ConsumerGroup: getEnv("CLV_CONSUMER_GROUP", "clv-calculator-group"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}



