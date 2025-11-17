package config_test

import (
	"os"
	"testing"

	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/config"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear environment variables
	os.Clearenv()

	cfg := config.LoadConfig()

	// Check server defaults
	if cfg.Server.Addr != ":8080" {
		t.Errorf("Expected default server addr ':8080', got '%s'", cfg.Server.Addr)
	}

	// Check Redis defaults
	if cfg.Redis.URL != "localhost:6380" {
		t.Errorf("Expected default redis URL 'localhost:6380', got '%s'", cfg.Redis.URL)
	}

	if cfg.Redis.Password != "" {
		t.Errorf("Expected empty default redis password, got '%s'", cfg.Redis.Password)
	}

	// Check stream defaults
	if cfg.Stream.ConsumerGroup != "ws-broadcaster" {
		t.Errorf("Expected default consumer group 'ws-broadcaster', got '%s'", cfg.Stream.ConsumerGroup)
	}

	if cfg.Stream.ConsumerID != "broadcaster-1" {
		t.Errorf("Expected default consumer ID 'broadcaster-1', got '%s'", cfg.Stream.ConsumerID)
	}

	// Check default sport (basketball_nba)
	if len(cfg.Stream.NormalizedOddsStreams) != 1 {
		t.Fatalf("Expected 1 default stream, got %d", len(cfg.Stream.NormalizedOddsStreams))
	}

	if cfg.Stream.NormalizedOddsStreams[0] != "odds.normalized.basketball_nba" {
		t.Errorf("Expected default stream 'odds.normalized.basketball_nba', got '%s'", cfg.Stream.NormalizedOddsStreams[0])
	}

	if cfg.Stream.OpportunitiesStream != "opportunities.detected" {
		t.Errorf("Expected opportunities stream 'opportunities.detected', got '%s'", cfg.Stream.OpportunitiesStream)
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	// Set custom environment variables
	os.Setenv("SERVER_ADDR", ":9090")
	os.Setenv("REDIS_URL", "redis.example.com:6379")
	os.Setenv("REDIS_PASSWORD", "secretpass")
	os.Setenv("SPORTS", "basketball_nba,americanfootball_nfl")
	os.Setenv("CONSUMER_GROUP", "custom-group")
	os.Setenv("CONSUMER_ID", "custom-id")
	defer os.Clearenv()

	cfg := config.LoadConfig()

	// Check server config
	if cfg.Server.Addr != ":9090" {
		t.Errorf("Expected server addr ':9090', got '%s'", cfg.Server.Addr)
	}

	// Check Redis config
	if cfg.Redis.URL != "redis.example.com:6379" {
		t.Errorf("Expected redis URL 'redis.example.com:6379', got '%s'", cfg.Redis.URL)
	}

	if cfg.Redis.Password != "secretpass" {
		t.Errorf("Expected redis password 'secretpass', got '%s'", cfg.Redis.Password)
	}

	// Check stream config
	if cfg.Stream.ConsumerGroup != "custom-group" {
		t.Errorf("Expected consumer group 'custom-group', got '%s'", cfg.Stream.ConsumerGroup)
	}

	if cfg.Stream.ConsumerID != "custom-id" {
		t.Errorf("Expected consumer ID 'custom-id', got '%s'", cfg.Stream.ConsumerID)
	}
}

func TestStreamConfig_MultipleSports(t *testing.T) {
	os.Setenv("SPORTS", "basketball_nba,americanfootball_nfl,baseball_mlb")
	defer os.Clearenv()

	cfg := config.LoadConfig()

	expected := []string{
		"odds.normalized.basketball_nba",
		"odds.normalized.americanfootball_nfl",
		"odds.normalized.baseball_mlb",
	}

	if len(cfg.Stream.NormalizedOddsStreams) != len(expected) {
		t.Fatalf("Expected %d streams, got %d", len(expected), len(cfg.Stream.NormalizedOddsStreams))
	}

	for i, expectedStream := range expected {
		if cfg.Stream.NormalizedOddsStreams[i] != expectedStream {
			t.Errorf("Stream %d: expected '%s', got '%s'", i, expectedStream, cfg.Stream.NormalizedOddsStreams[i])
		}
	}
}

func TestStreamConfig_GetAllStreams(t *testing.T) {
	os.Setenv("SPORTS", "basketball_nba,americanfootball_nfl")
	defer os.Clearenv()

	cfg := config.LoadConfig()
	allStreams := cfg.Stream.GetAllStreams()

	// Should include 2 normalized odds streams + 1 opportunities stream
	expectedCount := 3
	if len(allStreams) != expectedCount {
		t.Fatalf("Expected %d total streams, got %d", expectedCount, len(allStreams))
	}

	// Check that opportunities stream is included
	hasOpportunities := false
	for _, stream := range allStreams {
		if stream == "opportunities.detected" {
			hasOpportunities = true
			break
		}
	}

	if !hasOpportunities {
		t.Error("Expected opportunities stream to be included in GetAllStreams()")
	}
}

func TestStreamConfig_EmptySport(t *testing.T) {
	// Test with empty sport in list (should be filtered out)
	os.Setenv("SPORTS", "basketball_nba,,americanfootball_nfl")
	defer os.Clearenv()

	cfg := config.LoadConfig()

	// Should only have 2 streams (empty string filtered out)
	if len(cfg.Stream.NormalizedOddsStreams) != 2 {
		t.Errorf("Expected 2 streams (empty filtered), got %d", len(cfg.Stream.NormalizedOddsStreams))
	}
}

func TestStreamConfig_WhitespaceHandling(t *testing.T) {
	// Test with whitespace around sport names
	os.Setenv("SPORTS", " basketball_nba , americanfootball_nfl ")
	defer os.Clearenv()

	cfg := config.LoadConfig()

	expected := []string{
		"odds.normalized.basketball_nba",
		"odds.normalized.americanfootball_nfl",
	}

	if len(cfg.Stream.NormalizedOddsStreams) != len(expected) {
		t.Fatalf("Expected %d streams, got %d", len(expected), len(cfg.Stream.NormalizedOddsStreams))
	}

	for i, expectedStream := range expected {
		if cfg.Stream.NormalizedOddsStreams[i] != expectedStream {
			t.Errorf("Stream %d: expected '%s', got '%s'", i, expectedStream, cfg.Stream.NormalizedOddsStreams[i])
		}
	}
}





