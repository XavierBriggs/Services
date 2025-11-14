package contracts

import (
	"context"

	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/models"
)

// SportNormalizer defines the interface for sport-specific odds normalization
// This enables the normalizer to support multiple sports with different characteristics
type SportNormalizer interface {
	// GetSportKey returns the unique identifier for this sport (e.g., "basketball_nba")
	GetSportKey() string

	// GetDisplayName returns the human-readable name (e.g., "NBA Basketball")
	GetDisplayName() string

	// Normalize processes raw odds and returns normalized odds with fair prices and edges
	// marketOdds contains all odds for the same event+market to enable consensus calculations
	Normalize(ctx context.Context, raw models.RawOdds, marketOdds []models.RawOdds) (*models.NormalizedOdds, error)

	// GetMarketType classifies the market type for proper vig removal
	GetMarketType(marketKey string) models.MarketType

	// GetVigMethod returns the vig removal method for this market type
	GetVigMethod(marketType models.MarketType) models.VigMethod

	// GetSharpBooks returns the list of sharp book keys for consensus calculation
	GetSharpBooks() []string

	// IsSharpBook checks if a book is considered sharp for this sport
	IsSharpBook(bookKey string) bool
}




