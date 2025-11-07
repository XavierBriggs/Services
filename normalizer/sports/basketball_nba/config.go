package basketball_nba

import "github.com/XavierBriggs/fortuna/services/normalizer/pkg/models"

// Config contains NBA-specific normalization configuration
type Config struct {
	SportKey    string
	DisplayName string

	// Sharp books for consensus calculation (ordered by reliability)
	SharpBooks []string

	// Market type classification
	TwoWayMarkets   []string // spreads, totals
	ThreeWayMarkets []string // h2h (though NBA rarely has draws)
	PropsMarkets    []string // player props

	// Edge thresholds
	MinEdgeForAlert float64 // Minimum edge to generate alert (e.g., 0.02 = 2%)
	SignificantEdge float64 // Edge considered significant (e.g., 0.05 = 5%)
}

// DefaultConfig returns the standard NBA normalization configuration
func DefaultConfig() *Config {
	return &Config{
		SportKey:    "basketball_nba",
		DisplayName: "NBA Basketball",

		// Sharp books (in order of reliability)
		// Pinnacle: Lowest margins, fastest to move, accepts high limits
		// Circa: Vegas sharp book, accepts large bets
		// Bookmaker: Sharp offshore book
		SharpBooks: []string{
			"pinnacle",
			"circa",
			"bookmaker",
		},

		// Two-way markets (use multiplicative vig removal)
		TwoWayMarkets: []string{
			"spreads",
			"totals",
		},

		// Three-way markets (use additive vig removal if needed)
		// NBA rarely has draws, but h2h is technically three-way in some books
		ThreeWayMarkets: []string{
			"h2h", // moneyline
		},

		// Player props (compare across books, no vig removal needed)
		PropsMarkets: []string{
			"player_points",
			"player_rebounds",
			"player_assists",
			"player_threes",
			"player_points_rebounds_assists",
			"player_points_rebounds",
			"player_points_assists",
			"player_rebounds_assists",
			"player_steals",
			"player_blocks",
			"player_turnovers",
			"player_double_double",
			"player_triple_double",
		},

		// Edge thresholds
		MinEdgeForAlert: 0.01, // 1% minimum for alerts
		SignificantEdge: 0.02, // 2%+ is significant
	}
}

// GetMarketType returns the market type for a given market key
func (c *Config) GetMarketType(marketKey string) models.MarketType {
	// Check two-way markets
	for _, m := range c.TwoWayMarkets {
		if m == marketKey {
			return models.MarketTypeTwoWay
		}
	}

	// Check three-way markets
	for _, m := range c.ThreeWayMarkets {
		if m == marketKey {
			return models.MarketTypeThreeWay
		}
	}

	// Check props
	for _, m := range c.PropsMarkets {
		if m == marketKey {
			return models.MarketTypeProps
		}
	}

	// Default to props if unknown
	return models.MarketTypeProps
}

// IsSharpBook checks if a book is in the sharp list
func (c *Config) IsSharpBook(bookKey string) bool {
	for _, sharp := range c.SharpBooks {
		if sharp == bookKey {
			return true
		}
	}
	return false
}

