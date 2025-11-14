package config

import (
	"fmt"
	"os"
	"strings"
)

// StreamConfig defines which Redis streams to consume from
type StreamConfig struct {
	// Sport-specific normalized odds streams (e.g., odds.normalized.basketball_nba)
	NormalizedOddsStreams []string

	// Sport-specific game updates streams (e.g., games.updates.basketball_nba)
	GameUpdatesStreams []string

	// Minerva live game streams (e.g., games.live.basketball_nba)
	LiveGameStreams []string

	// Minerva final stats streams (e.g., games.stats.basketball_nba)
	GameStatsStreams []string

	// Opportunities stream (sport-agnostic)
	OpportunitiesStream string

	// Consumer group and ID
	ConsumerGroup string
	ConsumerID    string
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Addr string
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	URL      string
	Password string
}

// Config holds all application configuration
type Config struct {
	Server ServerConfig
	Redis  RedisConfig
	Stream StreamConfig
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Addr: getEnv("SERVER_ADDR", ":8080"),
		},
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", "localhost:6380"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		Stream: loadStreamConfig(),
	}
}

// loadStreamConfig loads stream configuration
// Supports multiple sports via comma-separated SPORTS environment variable
func loadStreamConfig() StreamConfig {
	// Get list of sports to consume (default: basketball_nba)
	sportsStr := getEnv("SPORTS", "basketball_nba")
	sports := strings.Split(sportsStr, ",")

	// Build normalized odds streams, game updates streams, and Minerva streams for each sport
	normalizedStreams := make([]string, 0, len(sports))
	gameStreams := make([]string, 0, len(sports))
	liveGameStreams := make([]string, 0, len(sports))
	gameStatsStreams := make([]string, 0, len(sports))
	
	for _, sport := range sports {
		sport = strings.TrimSpace(sport)
		if sport != "" {
			normalizedStreams = append(normalizedStreams, fmt.Sprintf("odds.normalized.%s", sport))
			gameStreams = append(gameStreams, fmt.Sprintf("games.updates.%s", sport))
			
			// Minerva streams
			liveGameStreams = append(liveGameStreams, fmt.Sprintf("games.live.%s", sport))
			gameStatsStreams = append(gameStatsStreams, fmt.Sprintf("games.stats.%s", sport))
		}
	}

	return StreamConfig{
		NormalizedOddsStreams: normalizedStreams,
		GameUpdatesStreams:    gameStreams,
		LiveGameStreams:       liveGameStreams,
		GameStatsStreams:      gameStatsStreams,
		OpportunitiesStream:   "opportunities.detected",
		ConsumerGroup:         getEnv("CONSUMER_GROUP", "ws-broadcaster"),
		ConsumerID:            getEnv("CONSUMER_ID", "broadcaster-1"),
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetAllStreams returns all streams to consume from
func (sc *StreamConfig) GetAllStreams() []string {
	streams := make([]string, 0, len(sc.NormalizedOddsStreams)+len(sc.GameUpdatesStreams)+len(sc.LiveGameStreams)+len(sc.GameStatsStreams)+1)
	streams = append(streams, sc.NormalizedOddsStreams...)
	streams = append(streams, sc.GameUpdatesStreams...)
	streams = append(streams, sc.LiveGameStreams...)
	streams = append(streams, sc.GameStatsStreams...)
	streams = append(streams, sc.OpportunitiesStream)
	return streams
}


