package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the auto-bettor service
type Config struct {
	// Redis
	RedisURL      string
	RedisPassword string

	// Consumer configuration
	ConsumerGroup string
	ConsumerID    string
	StreamKey     string
	BatchSize     int
	BlockTime     time.Duration

	// Database connections
	HolocronDSN    string
	AlexandriaDSN  string

	// External services
	KellyCalculatorURL string
	BotServiceURL      string

	// Service configuration
	LogLevel         string
	BalanceCacheTTL  time.Duration
	SettingsCacheTTL time.Duration
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		// Redis
		RedisURL:      getEnv("REDIS_URL", "redis:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		// Consumer
		ConsumerGroup: getEnv("CONSUMER_GROUP", "auto-bettor"),
		ConsumerID:    getEnv("CONSUMER_ID", "auto-bettor-1"),
		StreamKey:     getEnv("STREAM_KEY", "opportunities.detected"),
		BatchSize:     getEnvInt("BATCH_SIZE", 500),
		BlockTime:     getEnvDuration("BLOCK_TIME", 1*time.Second),

		// Databases
		HolocronDSN:    getEnvRequired("HOLOCRON_DSN"),
		AlexandriaDSN:  getEnvRequired("ALEXANDRIA_DSN"),

		// External services
		KellyCalculatorURL: getEnv("KELLY_CALCULATOR_URL", "http://kelly-calculator:8084"),
		BotServiceURL:      getEnv("BOT_SERVICE_URL", "http://bot-service:8090"),

		// Service
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		BalanceCacheTTL:  getEnvDuration("BALANCE_CACHE_TTL", 30*time.Second),
		SettingsCacheTTL: getEnvDuration("SETTINGS_CACHE_TTL", 10*time.Second),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvRequired(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return value
}

func getEnvInt(key string, defaultValue int) int {
	strValue := os.Getenv(key)
	if strValue == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(strValue)
	if err != nil {
		return defaultValue
	}

	return intValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	strValue := os.Getenv(key)
	if strValue == "" {
		return defaultValue
	}

	duration, err := time.ParseDuration(strValue)
	if err != nil {
		return defaultValue
	}

	return duration
}

