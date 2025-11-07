package basketball_nba

import (
	"os"
	"strconv"
	"strings"
)

// Config holds NBA-specific edge detection configuration
type Config struct {
	MinEdgePct          float64
	MaxDataAgeSeconds   int
	EnableMiddles       bool
	EnableScalps        bool
	EnabledMarkets      []string
	EnablePlayerProps   bool
	SharpBookMinimum    int
	SharpBooks          []string // Configurable list of sharp book keys
}

// NewConfig creates a new NBA configuration with defaults and environment overrides
func NewConfig() *Config {
	return &Config{
		MinEdgePct:         getEnvFloat("MIN_EDGE_PCT", 0.01),                                      // 1%
		MaxDataAgeSeconds:  getEnvInt("MAX_DATA_AGE_SECONDS", 10),                                  // 10 seconds
		EnableMiddles:      getEnvBool("ENABLE_MIDDLES", true),                                     // Enabled
		EnableScalps:       getEnvBool("ENABLE_SCALPS", true),                                      // Enabled
		EnabledMarkets:     getEnvStringSlice("ENABLED_MARKETS", []string{"h2h", "spreads", "totals"}), // Featured markets
		EnablePlayerProps:  getEnvBool("ENABLE_PLAYER_PROPS", false),                               // Not in v0
		SharpBookMinimum:   getEnvInt("SHARP_BOOK_MINIMUM", 1),                                     // At least 1 sharp book
		SharpBooks:         getEnvStringSlice("SHARP_BOOKS", []string{"pinnacle"}),                 // Default: Pinnacle
	}
}

// GetMinEdgePercent implements DetectorConfig
func (c *Config) GetMinEdgePercent() float64 {
	return c.MinEdgePct
}

// GetMaxDataAgeSeconds implements DetectorConfig
func (c *Config) GetMaxDataAgeSeconds() int {
	return c.MaxDataAgeSeconds
}

// IsMiddleDetectionEnabled implements DetectorConfig
func (c *Config) IsMiddleDetectionEnabled() bool {
	return c.EnableMiddles
}

// IsScalpDetectionEnabled implements DetectorConfig
func (c *Config) IsScalpDetectionEnabled() bool {
	return c.EnableScalps
}

// GetEnabledMarkets implements DetectorConfig
func (c *Config) GetEnabledMarkets() []string {
	return c.EnabledMarkets
}

// IsPlayerPropsEnabled implements DetectorConfig
func (c *Config) IsPlayerPropsEnabled() bool {
	return c.EnablePlayerProps
}

// IsMarketEnabled checks if a given market is enabled
func (c *Config) IsMarketEnabled(marketKey string) bool {
	for _, m := range c.EnabledMarkets {
		if m == marketKey {
			return true
		}
	}
	return false
}

// GetSharpBooks returns the configured list of sharp books
func (c *Config) GetSharpBooks() []string {
	return c.SharpBooks
}

// Helper functions for environment variable parsing

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

