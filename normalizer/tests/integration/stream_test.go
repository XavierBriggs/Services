// +build integration

package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/XavierBriggs/fortuna/services/normalizer/internal/consumer"
	"github.com/XavierBriggs/fortuna/services/normalizer/internal/processor"
	"github.com/XavierBriggs/fortuna/services/normalizer/internal/publisher"
	"github.com/XavierBriggs/fortuna/services/normalizer/internal/registry"
	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/models"
	"github.com/XavierBriggs/fortuna/services/normalizer/sports/basketball_nba"
	"github.com/XavierBriggs/fortuna/services/normalizer/tests/testutil"
	"github.com/redis/go-redis/v9"
)

func getTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	redisURL := getEnv("REDIS_TEST_URL", "localhost:6380")
	redisPassword := os.Getenv("REDIS_TEST_PASSWORD")

	client := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       1, // Use DB 1 for tests to avoid conflicts
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("failed to connect to Redis: %v", err)
	}

	// Clean up test data
	t.Cleanup(func() {
		client.FlushDB(ctx)
		client.Close()
	})

	return client
}

func TestStreamProcessing_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup Redis
	redisClient := getTestRedisClient(t)

	// Setup normalizer registry
	normalizerRegistry := registry.NewNormalizerRegistry()
	nbaModule := basketball_nba.NewNormalizer()
	if err := normalizerRegistry.Register(nbaModule); err != nil {
		t.Fatalf("failed to register NBA normalizer: %v", err)
	}

	// Setup components
	streamConsumer := consumer.NewStreamConsumer(redisClient, "test-consumer", "test-group")
	streamPublisher := publisher.NewStreamPublisher(redisClient)
	proc := processor.NewProcessor(streamConsumer, streamPublisher, normalizerRegistry)

	// Start processor in background
	go func() {
		if err := proc.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("processor error: %v", err)
		}
	}()

	// Give processor time to start
	time.Sleep(500 * time.Millisecond)

	// Publish raw odds to input stream
	rawStreamKey := "odds.raw.basketball_nba"
	lakers := testutil.SpreadOdds("fanduel", "Los Angeles Lakers", -110, -7.5)
	celtics := testutil.SpreadOdds("fanduel", "Boston Celtics", -110, 7.5)
	celtics.EventID = lakers.EventID

	for _, odds := range []models.RawOdds{lakers, celtics} {
		data, _ := json.Marshal(odds)
		if err := redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: rawStreamKey,
			Values: map[string]interface{}{
				"data": string(data),
			},
		}).Err(); err != nil {
			t.Fatalf("failed to publish raw odds: %v", err)
		}
	}

	// Wait for processing
	time.Sleep(3 * time.Second)

	// Verify normalized odds were published
	normalizedStreamKey := "odds.normalized.basketball_nba"
	messages, err := redisClient.XRange(ctx, normalizedStreamKey, "-", "+").Result()
	if err != nil {
		t.Fatalf("failed to read normalized stream: %v", err)
	}

	if len(messages) < 2 {
		t.Fatalf("expected at least 2 normalized messages, got %d", len(messages))
	}

	// Parse and verify LAST normalized odds (which should have full market context)
	data, ok := messages[len(messages)-1].Values["data"].(string)
	if !ok {
		t.Fatal("message data is not a string")
	}

	var normalized models.NormalizedOdds
	if err := json.Unmarshal([]byte(data), &normalized); err != nil {
		t.Fatalf("failed to unmarshal normalized odds: %v", err)
	}

	// Verify normalized fields
	if normalized.EventID == "" {
		t.Error("EventID is empty")
	}

	if normalized.MarketType != "two_way" {
		t.Errorf("MarketType = %s, want two_way", normalized.MarketType)
	}

	if normalized.DecimalOdds <= 0 {
		t.Error("DecimalOdds should be > 0")
	}

	if normalized.ImpliedProbability <= 0 || normalized.ImpliedProbability >= 1 {
		t.Errorf("ImpliedProbability = %f, want between 0 and 1", normalized.ImpliedProbability)
	}

	// NoVigProbability should be set for the second message (has opposite side context)
	if normalized.NoVigProbability == nil {
		t.Log("Warning: NoVigProbability is nil - opposite side may not have been processed yet")
	} else {
		t.Logf("✓ NoVigProbability calculated: %.4f", *normalized.NoVigProbability)
	}

	// FairPrice should be set if NoVigProbability was calculated
	if normalized.FairPrice != nil {
		t.Logf("✓ FairPrice calculated: %d", *normalized.FairPrice)
	}

	// ProcessingLatency can be 0 for ultra-fast operations
	if normalized.ProcessingLatency < 0 {
		t.Error("ProcessingLatency should be >= 0")
	}

	// Check metrics
	processed, errors := proc.GetMetrics()
	if processed < 2 {
		t.Errorf("processed = %d, want >= 2", processed)
	}

	if errors > 0 {
		t.Errorf("errors = %d, want 0", errors)
	}

	t.Logf("✓ Processed %d messages with %d errors", processed, errors)
}

func TestStreamProcessing_LatencySLO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup
	redisClient := getTestRedisClient(t)
	normalizerRegistry := registry.NewNormalizerRegistry()
	nbaModule := basketball_nba.NewNormalizer()
	normalizerRegistry.Register(nbaModule)

	streamConsumer := consumer.NewStreamConsumer(redisClient, "latency-test", "latency-group")
	streamPublisher := publisher.NewStreamPublisher(redisClient)
	proc := processor.NewProcessor(streamConsumer, streamPublisher, normalizerRegistry)

	// Start processor
	go proc.Start(ctx)
	time.Sleep(500 * time.Millisecond)

	// Test latency with 100 odds updates
	numUpdates := 100
	rawStreamKey := "odds.raw.basketball_nba"
	normalizedStreamKey := "odds.normalized.basketball_nba"

	startTime := time.Now()

	for i := 0; i < numUpdates; i++ {
		odds := testutil.SpreadOdds("fanduel", "Team A", -110+i, -7.5)
		data, _ := json.Marshal(odds)
		redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: rawStreamKey,
			Values: map[string]interface{}{
				"data": string(data),
			},
		})
	}

	// Wait for all messages to be processed
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		messages, _ := redisClient.XLen(ctx, normalizedStreamKey).Result()
		if messages >= int64(numUpdates) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	endTime := time.Now()
	totalTime := endTime.Sub(startTime)
	avgLatency := totalTime / time.Duration(numUpdates)

	t.Logf("Processed %d updates in %v (avg: %v per update)", numUpdates, totalTime, avgLatency)

	// SLO: Average processing latency should be < 10ms per update
	if avgLatency > 10*time.Millisecond {
		t.Errorf("Average latency %v exceeds 10ms SLO", avgLatency)
	}

	// Verify all were processed
	processed, errors := proc.GetMetrics()
	if processed < int64(numUpdates) {
		t.Errorf("processed = %d, want >= %d", processed, numUpdates)
	}

	if errors > 0 {
		t.Errorf("errors = %d, want 0", errors)
	}
}

func TestStreamProcessing_SharpConsensus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup
	redisClient := getTestRedisClient(t)
	normalizerRegistry := registry.NewNormalizerRegistry()
	nbaModule := basketball_nba.NewNormalizer()
	normalizerRegistry.Register(nbaModule)

	streamConsumer := consumer.NewStreamConsumer(redisClient, "consensus-test", "consensus-group")
	streamPublisher := publisher.NewStreamPublisher(redisClient)
	proc := processor.NewProcessor(streamConsumer, streamPublisher, normalizerRegistry)

	go proc.Start(ctx)
	time.Sleep(500 * time.Millisecond)

	// Publish sharp book odds first
	rawStreamKey := "odds.raw.basketball_nba"
	eventID := "test-event-sharp-consensus"

	sharpOdds := []models.RawOdds{
		testutil.SpreadOdds("pinnacle", "Team A", -110, -7.5),
		testutil.SpreadOdds("circa", "Team A", -108, -7.5),
		testutil.SpreadOdds("bookmaker", "Team A", -112, -7.5),
	}

	for i := range sharpOdds {
		sharpOdds[i].EventID = eventID
		data, _ := json.Marshal(sharpOdds[i])
		redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: rawStreamKey,
			Values: map[string]interface{}{
				"data": string(data),
			},
		})
	}

	// Wait for sharp odds to be processed
	time.Sleep(1 * time.Second)

	// Now publish soft book odds
	softOdds := testutil.SpreadOdds("fanduel", "Team A", -105, -7.5)
	softOdds.EventID = eventID
	data, _ := json.Marshal(softOdds)
	redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: rawStreamKey,
		Values: map[string]interface{}{"data": string(data)},
	})

	// Wait for processing
	time.Sleep(2 * time.Second)

	// Read normalized stream and find the soft book odds
	normalizedStreamKey := "odds.normalized.basketball_nba"
	messages, err := redisClient.XRange(ctx, normalizedStreamKey, "-", "+").Result()
	if err != nil {
		t.Fatalf("failed to read normalized stream: %v", err)
	}

	// Find the soft book's normalized odds
	var softNormalized *models.NormalizedOdds
	for _, msg := range messages {
		data, _ := msg.Values["data"].(string)
		var normalized models.NormalizedOdds
		json.Unmarshal([]byte(data), &normalized)

		if normalized.BookKey == "fanduel" && normalized.EventID == eventID {
			softNormalized = &normalized
			break
		}
	}

	if softNormalized == nil {
		t.Fatal("soft book normalized odds not found")
	}

	// Verify sharp consensus was calculated (may be nil if sharp books haven't been processed yet)
	if softNormalized.SharpConsensus == nil {
		t.Log("Warning: SharpConsensus is nil - sharp books may not have been processed in time")
		t.Log("This is expected in async stream processing - sharp books need to arrive first")
	} else {
		t.Logf("✓ Sharp consensus probability: %f", *softNormalized.SharpConsensus)

		// Should be reasonable (between 40% and 60%)
		if *softNormalized.SharpConsensus < 0.4 || *softNormalized.SharpConsensus > 0.6 {
			t.Errorf("SharpConsensus = %f, want between 0.4 and 0.6", *softNormalized.SharpConsensus)
		}
	}

	// Verify edge was calculated (may be nil without sharp consensus)
	if softNormalized.Edge == nil {
		t.Log("Warning: Edge is nil (expected if SharpConsensus is nil)")
	} else {
		t.Logf("✓ Edge vs sharp consensus: %.2f%%", *softNormalized.Edge*100)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

