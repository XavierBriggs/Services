package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/XavierBriggs/fortuna/services/opportunity-analytics/internal/enricher"
)

func main() {
	fmt.Println("📊 Bet Enrichment Service Starting...")
	fmt.Println("   Version: 3.0.0")
	fmt.Println("   Features: Hold time, Edge distribution, Execution rate, Opportunity CLV")
	fmt.Println()

	// Load configuration
	config := loadConfig()

	// Initialize database connection
	holocronDB, err := sql.Open("postgres", config.HolocronDSN)
	if err != nil {
		fmt.Printf("Failed to connect to Holocron: %v\n", err)
		os.Exit(1)
	}
	defer holocronDB.Close()

	// Configure connection pool
	holocronDB.SetMaxOpenConns(5)
	holocronDB.SetMaxIdleConns(2)
	holocronDB.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := holocronDB.Ping(); err != nil {
		fmt.Printf("Failed to ping Holocron: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Database connection established")

	// Initialize enricher
	betEnricher := enricher.NewBetEnricher(holocronDB)

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server for health checks and monitoring
	go startHTTPServer(config.Port, betEnricher)

	// Run initial enrichment (includes opportunity CLV)
	fmt.Println("🚀 Running initial full enrichment...")
	if err := betEnricher.RunFullEnrichment(ctx, config.LookbackHours); err != nil {
		fmt.Printf("Initial enrichment failed: %v\n", err)
	}

	// Start periodic enrichment
	ticker := time.NewTicker(config.EnrichmentInterval)
	defer ticker.Stop()

	fmt.Printf("🔥 Bet Enrichment Service Running\n")
	fmt.Printf("   Enrichment Interval: %v\n", config.EnrichmentInterval)
	fmt.Printf("   Lookback Hours: %d\n", config.LookbackHours)
	fmt.Printf("   HTTP Port: %s\n", config.Port)
	fmt.Println()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n🛑 Shutting down...")
			return

		case <-sigChan:
			fmt.Println("\n🛑 Received shutdown signal...")
			cancel()
			return

		case <-ticker.C:
			fmt.Printf("\n⏰ Running scheduled enrichment at %s...\n", time.Now().Format(time.RFC3339))

			// Run full enrichment (bets + opportunity CLV)
			if err := betEnricher.RunFullEnrichment(ctx, config.LookbackHours); err != nil {
				fmt.Printf("❌ Enrichment cycle failed: %v\n", err)
			}
		}
	}
}

// Config holds service configuration
type Config struct {
	HolocronDSN        string
	EnrichmentInterval time.Duration
	LookbackHours      int
	Port               string
}

func loadConfig() Config {
	return Config{
		HolocronDSN:        getEnv("HOLOCRON_DSN", "postgres://fortuna:fortuna_dev_password@localhost:5436/holocron?sslmode=disable"),
		EnrichmentInterval: parseDuration(getEnv("ENRICHMENT_INTERVAL", "15m")),
		LookbackHours:      parseInt(getEnv("LOOKBACK_HOURS", "48")),
		Port:               getEnv("PORT", "8092"),
	}
}

func startHTTPServer(port string, betEnricher *enricher.BetEnricher) {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "healthy",
			"service": "bet-enricher",
			"version": "3.0.0",
			"features": []string{
				"bet_enrichment",
				"opportunity_clv",
				"book_pair_clv",
				"missed_opportunities",
			},
			"timestamp": time.Now(),
		})
	})

	http.HandleFunc("/api/enricher/summary", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		summary, err := betEnricher.GetEnrichmentSummary(ctx)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
	})

	http.HandleFunc("/api/enricher/backfill", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		days := 7 // Default to 7 days
		if daysParam := r.URL.Query().Get("days"); daysParam != "" {
			fmt.Sscanf(daysParam, "%d", &days)
		}

		// Run backfill in background
		go func() {
			ctx := context.Background()
			if err := betEnricher.BackfillHistoricalBets(ctx, days); err != nil {
				fmt.Printf("❌ Backfill failed: %v\n", err)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "started",
			"days":    days,
			"message": fmt.Sprintf("Backfill started for %d days of historical data", days),
		})
	})

	http.HandleFunc("/api/enricher/trigger", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		hours := 48 // Default lookback
		if hoursParam := r.URL.Query().Get("hours"); hoursParam != "" {
			fmt.Sscanf(hoursParam, "%d", &hours)
		}

		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()

		// Run full enrichment including opportunity CLV
		if err := betEnricher.RunFullEnrichment(ctx, hours); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "completed",
			"hours":   hours,
			"message": fmt.Sprintf("Full enrichment (bets + opportunity CLV) completed for last %d hours", hours),
		})
	})

	// New endpoint for opportunity CLV summary
	http.HandleFunc("/api/enricher/opportunity-clv", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		summary, err := betEnricher.GetOpportunityCLVSummary(ctx)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
	})

	addr := fmt.Sprintf(":%s", port)
	fmt.Printf("📡 HTTP Server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Printf("HTTP server error: %v\n", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 15 * time.Minute
	}
	return d
}

func parseInt(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	if i == 0 {
		return 48
	}
	return i
}
