package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/config"
	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/consumer"
	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/handlers"
	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/hub"
	"github.com/redis/go-redis/v9"
)

func main() {
	fmt.Println("🚀 Starting WebSocket Broadcaster...")

	// Load config
	cfg := config.LoadConfig()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.URL,
		Password: cfg.Redis.Password,
		DB:       0,
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		fmt.Printf("❌ Failed to connect to Redis: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Redis")

	// Create hub
	h := hub.NewHub()
	go h.Run(ctx)

	// Create stream consumer
	streamConsumer := consumer.NewStreamConsumer(redisClient, h, cfg.Stream)
	go streamConsumer.Start(ctx)

	// Create HTTP handler (pass context for WebSocket lifecycle)
	handler := handlers.NewHandler(h, ctx)

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handler.HandleWebSocket)
	mux.HandleFunc("/health", handler.HandleHealth)
	mux.HandleFunc("/metrics", handler.HandleMetrics)

	// Start HTTP server
	server := &http.Server{
		Addr:    cfg.Server.Addr,
		Handler: mux,
	}

	go func() {
		fmt.Printf("✓ WebSocket server listening on %s\n", cfg.Server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n🛑 Shutting down...")

	// Cancel context to stop all goroutines
	cancel()

	// Graceful shutdown of HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("⚠️  Server shutdown error: %v\n", err)
	}

	// Close Redis connection
	redisClient.Close()

	fmt.Println("✓ Shutdown complete")
}

