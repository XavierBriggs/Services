package detector

import (
	"context"
	"time"

	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/contracts"
	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/models"
)

// MiddleDetector detects middle opportunities (both sides of market have +EV)
type MiddleDetector struct {
	config            contracts.DetectorConfig
	sharpBookProvider contracts.SharpBookProvider
}

// NewMiddleDetector creates a new middle detector
func NewMiddleDetector(config contracts.DetectorConfig, sharpBookProvider contracts.SharpBookProvider) *MiddleDetector {
	return &MiddleDetector{
		config:            config,
		sharpBookProvider: sharpBookProvider,
	}
}

// Detect analyzes market odds and returns middle opportunities
func (d *MiddleDetector) Detect(ctx context.Context, odds models.NormalizedOdds, marketOdds []models.NormalizedOdds) ([]models.Opportunity, error) {
	if !d.IsEnabled() {
		return nil, nil
	}

	// Only detect middles for two-way markets (spreads, totals)
	if odds.MarketKey != "spreads" && odds.MarketKey != "totals" {
		return nil, nil
	}

	// Check data age
	dataAge := time.Since(odds.ReceivedAt)
	if int(dataAge.Seconds()) > d.config.GetMaxDataAgeSeconds() {
		return nil, nil
	}

	// Get sharp consensus
	sharpConsensus, err := d.sharpBookProvider.GetSharpConsensus(ctx, marketOdds)
	if err != nil {
		return nil, nil
	}

	// Group market odds by outcome (e.g., "Over 223.5" and "Under 223.5")
	// For a middle to exist, we need opposite sides of the same line to both have +EV
	outcomeEdges := make(map[string][]middleCandidate)

	for _, marketOdd := range marketOdds {
		// Skip sharp books (we bet soft books)
		if d.sharpBookProvider.IsSharpBook(marketOdd.BookKey) {
			continue
		}

		// Calculate edge for this odd
		fairProb, exists := sharpConsensus[marketOdd.OutcomeName]
		if !exists {
			continue
		}

		edge := (fairProb / marketOdd.ImpliedProbability) - 1.0

		// Check if this has positive edge
		if edge > 0 {
			outcomeEdges[marketOdd.OutcomeName] = append(outcomeEdges[marketOdd.OutcomeName], middleCandidate{
				odds: marketOdd,
				edge: edge,
			})
		}
	}

	// Look for middles: same point value with both sides having +EV
	var opportunities []models.Opportunity

	// For spreads/totals, look for opposing outcomes
	for outcome1, candidates1 := range outcomeEdges {
		for _, cand1 := range candidates1 {
			// Look for opposing side with same point
			if cand1.odds.Point == nil {
				continue
			}

			for outcome2, candidates2 := range outcomeEdges {
				if outcome1 == outcome2 {
					continue // Same outcome, not a middle
				}

				for _, cand2 := range candidates2 {
					if cand2.odds.Point == nil {
						continue
					}

					// Check if points match (or are close for a middle window)
					if *cand1.odds.Point == *cand2.odds.Point {
						// Both sides have +EV at the same line = MIDDLE
						totalEdge := (cand1.edge + cand2.edge) / 2.0 * 100 // Average edge

						if totalEdge >= d.config.GetMinEdgePercent()*100 {
							opp := models.Opportunity{
								OpportunityType: models.OpportunityTypeMiddle,
								SportKey:        odds.SportKey,
								EventID:         odds.EventID,
								MarketKey:       odds.MarketKey,
								EdgePercent:     totalEdge,
								FairPrice:       nil, // No single fair price for middles
								DetectedAt:      time.Now(),
								DataAgeSeconds:  int(dataAge.Seconds()),
								Legs: []models.OpportunityLeg{
									{
										BookKey:        cand1.odds.BookKey,
										OutcomeName:    cand1.odds.OutcomeName,
										Price:          cand1.odds.Price,
										Point:          cand1.odds.Point,
										LegEdgePercent: &[]float64{cand1.edge * 100}[0],
									},
									{
										BookKey:        cand2.odds.BookKey,
										OutcomeName:    cand2.odds.OutcomeName,
										Price:          cand2.odds.Price,
										Point:          cand2.odds.Point,
										LegEdgePercent: &[]float64{cand2.edge * 100}[0],
									},
								},
							}

							opportunities = append(opportunities, opp)
						}
					}
				}
			}
		}
	}

	return opportunities, nil
}

// GetType returns the detector type
func (d *MiddleDetector) GetType() models.OpportunityType {
	return models.OpportunityTypeMiddle
}

// IsEnabled returns whether middle detection is enabled
func (d *MiddleDetector) IsEnabled() bool {
	return d.config.IsMiddleDetectionEnabled()
}

// middleCandidate represents a potential leg of a middle opportunity
type middleCandidate struct {
	odds models.NormalizedOdds
	edge float64
}

