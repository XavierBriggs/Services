package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XavierBriggs/fortuna/services/settlement-service/internal/settler"
	_ "github.com/lib/pq"
)

func main() {
	fmt.Println("=== Settlement Service v0 ===")

	// Load config
	config := loadConfig()

	// Connect to Alexandria (for event scores)
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

	// Initialize settler
	s := settler.NewSettler(
		alexandriaDB,
		holocronDB,
		config.OddsAPIKey,
		config.PollInterval,
	)

	// Start settler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		fmt.Printf("✓ Settlement Service started\n")
		fmt.Printf("  Poll Interval: %v\n", config.PollInterval)
		fmt.Printf("  Auto-settlement enabled\n")
		if err := s.Start(ctx); err != nil {
			fmt.Printf("❌ Settler error: %v\n", err)
		}
	}()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n✓ Shutting down gracefully...")
	cancel()
	time.Sleep(2 * time.Second)
	fmt.Println("✓ Settlement Service stopped")
}

type Config struct {
	AlexandriaDSN string
	HolocronDSN   string
	OddsAPIKey    string
	PollInterval  time.Duration
}

func loadConfig() Config {
	pollInterval := 5 * time.Minute
	if intervalStr := os.Getenv("SETTLEMENT_POLL_INTERVAL"); intervalStr != "" {
		if parsed, err := time.ParseDuration(intervalStr); err == nil {
			pollInterval = parsed
		}
	}

	return Config{
		AlexandriaDSN: getEnv("ALEXANDRIA_DSN", "postgres://fortuna:fortuna@localhost:5435/alexandria?sslmode=disable"),
		HolocronDSN:   getEnv("HOLOCRON_DSN", "postgres://fortuna:fortuna@localhost:5436/holocron?sslmode=disable"),
		OddsAPIKey:    getEnv("ODDS_API_KEY", ""),
		PollInterval:  pollInterval,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}


