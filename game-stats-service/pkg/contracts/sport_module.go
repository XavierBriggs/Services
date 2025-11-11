package contracts

import (
	"time"

	"github.com/fortuna/services/game-stats-service/pkg/models"
)

// SportModule is the pluggable interface for adding new sports
// Similar to Mercury's SportModule pattern
type SportModule interface {
	// Identification
	GetSportKey() string       // "basketball_nba", "american_football_nfl"
	GetDisplayName() string    // "NBA", "NFL"
	GetESPNSportPath() string  // "basketball/nba", "football/nfl"

	// Configuration
	GetPollingConfig() PollingConfig
	IsEnabled() bool

	// Data parsing (sport-specific stat formats)
	ParseGameSummary(rawData map[string]interface{}) (*models.Game, error)
	ParseBoxScore(rawData map[string]interface{}) (*models.BoxScore, error)
	ParsePlayerStats(rawData map[string]interface{}) ([]models.PlayerStat, error)

	// Validation
	ValidateGame(game *models.Game) error

	// Team normalization (for odds integration)
	NormalizeTeamName(espnName string) string
	GetTeamAbbreviation(fullName string) string
}

// PollingConfig defines sport-specific polling behavior
type PollingConfig struct {
	LiveInterval     time.Duration // 30s for live games
	UpcomingInterval time.Duration // 5min for upcoming games
	PreGameRampup    time.Duration // Start fast polling 30min before
	Enabled          bool          // Feature flag per sport
}


