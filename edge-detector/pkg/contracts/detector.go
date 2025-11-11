package contracts

import (
	"context"

	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/models"
)

// OpportunityDetector defines the interface for detecting betting opportunities
type OpportunityDetector interface {
	// Detect analyzes normalized odds and returns detected opportunities
	Detect(ctx context.Context, odds models.NormalizedOdds, marketOdds []models.NormalizedOdds) ([]models.Opportunity, error)

	// GetType returns the type of opportunities this detector finds
	GetType() models.OpportunityType

	// IsEnabled returns whether this detector is currently enabled
	IsEnabled() bool
}

// SharpBookProvider defines the interface for identifying sharp sportsbooks
type SharpBookProvider interface {
	// GetSharpBooks returns the list of book keys considered "sharp" for this sport
	GetSharpBooks(ctx context.Context, sportKey string) ([]string, error)

	// IsSharpBook returns whether a given book is considered sharp
	IsSharpBook(bookKey string) bool

	// GetSharpConsensus calculates the average fair probability from sharp books
	// Returns the consensus probability for each outcome in a market
	GetSharpConsensus(ctx context.Context, marketOdds []models.NormalizedOdds) (map[string]float64, error)
}

// DetectorConfig defines configuration for opportunity detection
type DetectorConfig interface {
	// GetMinEdgePercent returns the minimum edge percentage threshold
	GetMinEdgePercent() float64

	// GetMaxDataAgeSeconds returns the maximum allowed data age
	GetMaxDataAgeSeconds() int

	// IsMiddleDetectionEnabled returns whether middle detection is enabled
	IsMiddleDetectionEnabled() bool

	// IsScalpDetectionEnabled returns whether scalp detection is enabled
	IsScalpDetectionEnabled() bool

	// GetEnabledMarkets returns the list of markets to monitor
	GetEnabledMarkets() []string

	// IsPlayerPropsEnabled returns whether player props detection is enabled
	IsPlayerPropsEnabled() bool
}


