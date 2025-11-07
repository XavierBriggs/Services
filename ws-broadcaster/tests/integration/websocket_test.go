// +build integration

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/config"
	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/consumer"
	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/handlers"
	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/hub"
	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/pkg/models"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

func getTestRedisClient(t *testing.T) *redis.Client {
	redisURL := getEnv("REDIS_URL", "localhost:6380")
	redisPassword := getEnv("REDIS_PASSWORD", "")

	client := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}

	return client
}

func TestWebSocket_Connection(t *testing.T) {
	// Create hub
	h := hub.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Create handler
	handler := handlers.NewHandler(h, ctx)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()

	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}

	// Wait for connection to be registered
	time.Sleep(200 * time.Millisecond)

	// Verify client is registered in hub
	if h.GetClientCount() != 1 {
		t.Errorf("Expected 1 connected client, got %d", h.GetClientCount())
	}

	// Close connection
	conn.Close()
	time.Sleep(200 * time.Millisecond)

	// Verify client is unregistered
	if h.GetClientCount() != 0 {
		t.Errorf("Expected 0 connected clients after disconnect, got %d", h.GetClientCount())
	}
}

func TestWebSocket_Subscribe(t *testing.T) {
	// Create hub
	h := hub.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Create handler
	handler := handlers.NewHandler(h, ctx)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()

	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}

	// Wait for connection to register
	time.Sleep(200 * time.Millisecond)

	// Send subscribe message
	subscribeMsg := models.ClientMessage{
		Type: "subscribe",
		Payload: map[string]interface{}{
			"sports":  []string{"basketball_nba"},
			"markets": []string{"h2h", "spreads"},
			"books":   []string{"fanduel", "draftkings"},
		},
	}

	if err := conn.WriteJSON(subscribeMsg); err != nil {
		t.Fatalf("Failed to send subscribe message: %v", err)
	}

	// Wait for message to be processed
	time.Sleep(200 * time.Millisecond)

	// Connection should still be active
	if h.GetClientCount() != 1 {
		t.Errorf("Expected 1 connected client after subscribe, got %d", h.GetClientCount())
	}

	// Clean up
	conn.Close()
	time.Sleep(100 * time.Millisecond)
}

func TestWebSocket_Heartbeat(t *testing.T) {
	// Create hub
	h := hub.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Create handler
	handler := handlers.NewHandler(h, ctx)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()

	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}

	// Wait for connection to register
	time.Sleep(200 * time.Millisecond)

	// Send heartbeat message
	heartbeatMsg := models.ClientMessage{
		Type:    "heartbeat",
		Payload: map[string]interface{}{},
	}

	if err := conn.WriteJSON(heartbeatMsg); err != nil {
		t.Fatalf("Failed to send heartbeat message: %v", err)
	}

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// Read heartbeat response
	var response models.ServerMessage
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatalf("Failed to read heartbeat response: %v", err)
	}

	// Verify response type
	if response.Type != "heartbeat" {
		t.Errorf("Expected heartbeat response, got %s", response.Type)
	}

	// Verify response contains stats
	if response.Payload == nil {
		t.Error("Expected heartbeat response to contain stats payload")
	}

	t.Logf("Heartbeat response: %+v", response)

	// Clean up
	conn.Close()
	time.Sleep(100 * time.Millisecond)
}

func TestStreamConsumer_RedisIntegration(t *testing.T) {
	redisClient := getTestRedisClient(t)
	defer redisClient.Close()

	// Create hub
	h := hub.NewHub()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go h.Run(ctx)

	// Create stream config
	streamConfig := config.StreamConfig{
		NormalizedOddsStreams: []string{"odds.normalized.basketball_nba"},
		OpportunitiesStream:   "opportunities.detected",
		ConsumerGroup:         "test-ws-broadcaster",
		ConsumerID:            "test-broadcaster-1",
	}

	// Create and start consumer
	streamConsumer := consumer.NewStreamConsumer(redisClient, h, streamConfig)
	go streamConsumer.Start(ctx)

	// Wait for consumer to initialize
	time.Sleep(500 * time.Millisecond)

	// Publish a test message to Redis stream
	testOdds := map[string]interface{}{
		"event_id":            "test_event_1",
		"sport_key":           "basketball_nba",
		"market_key":          "h2h",
		"book_key":            "fanduel",
		"outcome_name":        "Los Angeles Lakers",
		"price":               -110,
		"decimal_odds":        1.909,
		"implied_probability": 0.5238,
		"normalized_at":       time.Now().Format(time.RFC3339),
	}

	oddsJSON, _ := json.Marshal(testOdds)
	_, err := redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: "odds.normalized.basketball_nba",
		Values: map[string]interface{}{
			"data": string(oddsJSON),
		},
	}).Result()

	if err != nil {
		t.Fatalf("Failed to publish test message to Redis: %v", err)
	}

	// Wait for message to be processed
	time.Sleep(1 * time.Second)

	t.Log("✓ Stream consumer successfully consumed test message from Redis")

	// Clean up - delete test stream
	redisClient.Del(ctx, "odds.normalized.basketball_nba")
}

func TestEndToEnd_WebSocketBroadcast(t *testing.T) {
	redisClient := getTestRedisClient(t)
	defer redisClient.Close()

	// Create hub
	h := hub.NewHub()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go h.Run(ctx)

	// Create stream config
	streamConfig := config.StreamConfig{
		NormalizedOddsStreams: []string{"odds.normalized.basketball_nba"},
		OpportunitiesStream:   "opportunities.detected",
		ConsumerGroup:         "test-ws-broadcaster-e2e",
		ConsumerID:            "test-broadcaster-e2e-1",
	}

	// Create and start consumer
	streamConsumer := consumer.NewStreamConsumer(redisClient, h, streamConfig)
	go streamConsumer.Start(ctx)

	// Create handler and test server
	handler := handlers.NewHandler(h, ctx)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()

	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}

	// Wait for connection to register
	time.Sleep(300 * time.Millisecond)

	// Subscribe to NBA odds
	subscribeMsg := models.ClientMessage{
		Type: "subscribe",
		Payload: map[string]interface{}{
			"sports": []string{"basketball_nba"},
		},
	}

	if err := conn.WriteJSON(subscribeMsg); err != nil {
		t.Fatalf("Failed to send subscribe message: %v", err)
	}

	// Wait for subscription to be processed and stream consumer to be ready
	time.Sleep(1 * time.Second)

	// Publish a test message to Redis stream
	testOdds := map[string]interface{}{
		"event_id":            "test_event_2",
		"sport_key":           "basketball_nba",
		"market_key":          "h2h",
		"book_key":            "draftkings",
		"outcome_name":        "Boston Celtics",
		"price":               120,
		"implied_probability": 0.4545,
		"market_type":         "two_way",
		"normalized_at":       time.Now().Format(time.RFC3339),
		"data_age_seconds":    0.5,
	}

	oddsJSON, _ := json.Marshal(testOdds)
	_, err = redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: "odds.normalized.basketball_nba",
		Values: map[string]interface{}{
			"data": string(oddsJSON),
		},
	}).Result()

	if err != nil {
		t.Fatalf("Failed to publish test message to Redis: %v", err)
	}

	// Wait for message to be processed and broadcast
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read broadcast message
	var response models.ServerMessage
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatalf("Failed to read broadcast message: %v", err)
	}

	// Verify message type
	if response.Type != "odds_update" {
		t.Errorf("Expected odds_update message, got %s", response.Type)
	}

	t.Logf("✓ Successfully received broadcast message: %+v", response)

	// Clean up
	conn.Close()
	time.Sleep(100 * time.Millisecond)
	redisClient.Del(ctx, "odds.normalized.basketball_nba")
}

func TestHealthEndpoint(t *testing.T) {
	// Create hub
	h := hub.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Create handler
	handler := handlers.NewHandler(h, ctx)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleHealth))
	defer server.Close()

	// Make HTTP request
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to call health endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}

	// Verify fields
	if health["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%v'", health["status"])
	}

	if health["service"] != "ws-broadcaster" {
		t.Errorf("Expected service 'ws-broadcaster', got '%v'", health["service"])
	}

	t.Logf("✓ Health check response: %+v", health)
}

func TestMetricsEndpoint(t *testing.T) {
	// Create hub
	h := hub.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Create handler
	handler := handlers.NewHandler(h, ctx)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleMetrics))
	defer server.Close()

	// Make HTTP request
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to call metrics endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	var metrics map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		t.Fatalf("Failed to parse metrics response: %v", err)
	}

	// Verify fields exist
	requiredFields := []string{"active_clients", "total_connections", "total_messages", "broadcast_capacity", "broadcast_usage"}
	for _, field := range requiredFields {
		if _, exists := metrics[field]; !exists {
			t.Errorf("Expected metrics to contain field '%s'", field)
		}
	}

	t.Logf("✓ Metrics response: %+v", metrics)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

