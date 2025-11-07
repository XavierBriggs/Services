package detector

import (
	"context"
	"time"

	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/contracts"
	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/models"
)

// EdgeDetector detects simple +EV opportunities (single bets with positive edge)
type EdgeDetector struct {
	config            contracts.DetectorConfig
	sharpBookProvider contracts.SharpBookProvider
}

// NewEdgeDetector creates a new edge detector
func NewEdgeDetector(config contracts.DetectorConfig, sharpBookProvider contracts.SharpBookProvider) *EdgeDetector {
	return &EdgeDetector{
		config:            config,
		sharpBookProvider: sharpBookProvider,
	}
}

// Detect analyzes odds and returns edge opportunities
func (d *EdgeDetector) Detect(ctx context.Context, odds models.NormalizedOdds, marketOdds []models.NormalizedOdds) ([]models.Opportunity, error) {
	// Check if market is enabled
	if !d.isMarketEnabled(odds.MarketKey) {
		return nil, nil
	}

	// Check data age
	dataAge := time.Since(odds.ReceivedAt)
	if int(dataAge.Seconds()) > d.config.GetMaxDataAgeSeconds() {
		return nil, nil // Data too stale
	}

	// Get sharp consensus for this outcome
	sharpConsensus, err := d.sharpBookProvider.GetSharpConsensus(ctx, marketOdds)
	if err != nil {
		// No sharp consensus available - skip this opportunity
		return nil, nil
	}

	fairProb, exists := sharpConsensus[odds.OutcomeName]
	if !exists {
		// No sharp consensus for this specific outcome
		return nil, nil
	}

	// Calculate edge: (fairProb / impliedProb) - 1
	edge := (fairProb / odds.ImpliedProbability) - 1.0

	// Check if edge meets threshold
	if edge < d.config.GetMinEdgePercent() {
		return nil, nil // Edge too small
	}

	// Check if this is a soft book (we only want edges at soft books)
	if d.sharpBookProvider.IsSharpBook(odds.BookKey) {
		return nil, nil // Don't bet sharp books (they're efficient)
	}

	// Calculate fair price in American odds
	fairDecimal := 1.0 / fairProb
	fairPrice := decimalToAmerican(fairDecimal)

	// Create opportunity
	opportunity := models.Opportunity{
		OpportunityType: models.OpportunityTypeEdge,
		SportKey:        odds.SportKey,
		EventID:         odds.EventID,
		MarketKey:       odds.MarketKey,
		EdgePercent:     edge * 100, // Convert to percentage
		FairPrice:       &fairPrice,
		DetectedAt:      time.Now(),
		DataAgeSeconds:  int(dataAge.Seconds()),
		Legs: []models.OpportunityLeg{
			{
				BookKey:        odds.BookKey,
				OutcomeName:    odds.OutcomeName,
				Price:          odds.Price,
				Point:          odds.Point,
				LegEdgePercent: &[]float64{edge * 100}[0],
			},
		},
	}

	return []models.Opportunity{opportunity}, nil
}

// GetType returns the detector type
func (d *EdgeDetector) GetType() models.OpportunityType {
	return models.OpportunityTypeEdge
}

// IsEnabled returns whether edge detection is enabled (always true for edge detector)
func (d *EdgeDetector) IsEnabled() bool {
	return true
}

// isMarketEnabled checks if a market is enabled in config
func (d *EdgeDetector) isMarketEnabled(marketKey string) bool {
	enabledMarkets := d.config.GetEnabledMarkets()
	for _, m := range enabledMarkets {
		if m == marketKey {
			return true
		}
	}
	return false
}

// decimalToAmerican converts decimal odds to American odds
func decimalToAmerican(decimal float64) int {
	if decimal >= 2.0 {
		// Positive American odds: (decimal - 1) * 100
		return int((decimal - 1.0) * 100)
	}

	// Negative American odds: -100 / (decimal - 1)
	return int(-100.0 / (decimal - 1.0))
}

// CalculateEdge computes edge percentage given fair and implied probabilities
func CalculateEdge(fairProb, impliedProb float64) float64 {
	if impliedProb == 0 {
		return 0
	}
	return (fairProb / impliedProb) - 1.0
}

