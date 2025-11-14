package detector

import (
	"context"
	"math"
	"time"

	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/contracts"
	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/models"
)

// ScalpDetector detects scalp opportunities (guaranteed profit arbitrage)
type ScalpDetector struct {
	config contracts.DetectorConfig
}

// NewScalpDetector creates a new scalp detector
func NewScalpDetector(config contracts.DetectorConfig) *ScalpDetector {
	return &ScalpDetector{
		config: config,
	}
}

// Detect analyzes market odds and returns scalp opportunities
func (d *ScalpDetector) Detect(ctx context.Context, odds models.NormalizedOdds, marketOdds []models.NormalizedOdds) ([]models.Opportunity, error) {
	if !d.IsEnabled() {
		return nil, nil
	}

	// Check data age
	dataAge := time.Since(odds.ReceivedAt)
	if int(dataAge.Seconds()) > d.config.GetMaxDataAgeSeconds() {
		return nil, nil
	}

	// Group odds by outcome
	outcomeOdds := make(map[string][]models.NormalizedOdds)
	for _, marketOdd := range marketOdds {
		outcomeOdds[marketOdd.OutcomeName] = append(outcomeOdds[marketOdd.OutcomeName], marketOdd)
	}

	var opportunities []models.Opportunity

	// For two-way markets, check all combinations
	if len(outcomeOdds) == 2 {
		var outcomes []string
		var oddsLists [][]models.NormalizedOdds

		for outcome, oddsList := range outcomeOdds {
			outcomes = append(outcomes, outcome)
			oddsLists = append(oddsLists, oddsList)
		}

		// Check all combinations of odds from both sides
		for _, odds1 := range oddsLists[0] {
			for _, odds2 := range oddsLists[1] {
				// Calculate inverse sum: 1/decimal1 + 1/decimal2
				inverseSum := (1.0 / odds1.DecimalOdds) + (1.0 / odds2.DecimalOdds)

				// Arbitrage exists if inverse sum < 1.0
				if inverseSum < 1.0 {
					// Calculate profit margin
					profitMargin := (1.0 - inverseSum) * 100.0

					// Check if profit meets minimum threshold
					if profitMargin >= d.config.GetMinEdgePercent()*100 {
						opp := models.Opportunity{
							OpportunityType: models.OpportunityTypeScalp,
							SportKey:        odds.SportKey,
							EventID:         odds.EventID,
							MarketKey:       odds.MarketKey,
							EdgePercent:     profitMargin,
							FairPrice:       nil, // No fair price for scalps (guaranteed profit)
							DetectedAt:      time.Now(),
							DataAgeSeconds:  int(dataAge.Seconds()),
							Legs: []models.OpportunityLeg{
								{
									BookKey:        odds1.BookKey,
									OutcomeName:    odds1.OutcomeName,
									Price:          odds1.Price,
									Point:          odds1.Point,
									LegEdgePercent: &[]float64{profitMargin / 2.0}[0], // Split profit
								},
								{
									BookKey:        odds2.BookKey,
									OutcomeName:    odds2.OutcomeName,
									Price:          odds2.Price,
									Point:          odds2.Point,
									LegEdgePercent: &[]float64{profitMargin / 2.0}[0], // Split profit
								},
							},
						}

						opportunities = append(opportunities, opp)
					}
				}
			}
		}
	}

	// For three-way markets (h2h with draw), check all combinations
	if len(outcomeOdds) == 3 {
		opportunities = append(opportunities, d.detectThreeWayScalps(odds, outcomeOdds, dataAge)...)
	}

	return opportunities, nil
}

// detectThreeWayScalps finds arbitrage in three-way markets
func (d *ScalpDetector) detectThreeWayScalps(odds models.NormalizedOdds, outcomeOdds map[string][]models.NormalizedOdds, dataAge time.Duration) []models.Opportunity {
	var opportunities []models.Opportunity

	// Get all outcomes
	var outcomes []string
	var oddsLists [][]models.NormalizedOdds

	for outcome, oddsList := range outcomeOdds {
		outcomes = append(outcomes, outcome)
		oddsLists = append(oddsLists, oddsList)
	}

	if len(oddsLists) != 3 {
		return opportunities
	}

	// Check all combinations of three odds
	for _, odds1 := range oddsLists[0] {
		for _, odds2 := range oddsLists[1] {
			for _, odds3 := range oddsLists[2] {
				// Calculate inverse sum for three-way
				inverseSum := (1.0 / odds1.DecimalOdds) + (1.0 / odds2.DecimalOdds) + (1.0 / odds3.DecimalOdds)

				// Arbitrage exists if inverse sum < 1.0
				if inverseSum < 1.0 {
					profitMargin := (1.0 - inverseSum) * 100.0

					if profitMargin >= d.config.GetMinEdgePercent()*100 {
						opp := models.Opportunity{
							OpportunityType: models.OpportunityTypeScalp,
							SportKey:        odds.SportKey,
							EventID:         odds.EventID,
							MarketKey:       odds.MarketKey,
							EdgePercent:     profitMargin,
							FairPrice:       nil,
							DetectedAt:      time.Now(),
							DataAgeSeconds:  int(dataAge.Seconds()),
							Legs: []models.OpportunityLeg{
								{
									BookKey:        odds1.BookKey,
									OutcomeName:    odds1.OutcomeName,
									Price:          odds1.Price,
									Point:          odds1.Point,
									LegEdgePercent: &[]float64{profitMargin / 3.0}[0],
								},
								{
									BookKey:        odds2.BookKey,
									OutcomeName:    odds2.OutcomeName,
									Price:          odds2.Price,
									Point:          odds2.Point,
									LegEdgePercent: &[]float64{profitMargin / 3.0}[0],
								},
								{
									BookKey:        odds3.BookKey,
									OutcomeName:    odds3.OutcomeName,
									Price:          odds3.Price,
									Point:          odds3.Point,
									LegEdgePercent: &[]float64{profitMargin / 3.0}[0],
								},
							},
						}

						opportunities = append(opportunities, opp)
					}
				}
			}
		}
	}

	return opportunities
}

// GetType returns the detector type
func (d *ScalpDetector) GetType() models.OpportunityType {
	return models.OpportunityTypeScalp
}

// IsEnabled returns whether scalp detection is enabled
func (d *ScalpDetector) IsEnabled() bool {
	return d.config.IsScalpDetectionEnabled()
}

// CalculateArbitrage checks if odds create an arbitrage opportunity
// Returns true if arbitrage exists, along with the profit margin
func CalculateArbitrage(decimalOdds []float64) (bool, float64) {
	if len(decimalOdds) < 2 {
		return false, 0
	}

	inverseSum := 0.0
	for _, decimal := range decimalOdds {
		if decimal <= 0 {
			return false, 0
		}
		inverseSum += 1.0 / decimal
	}

	// Arbitrage exists if sum < 1.0
	isArbitrage := inverseSum < 1.0
	profitMargin := 0.0

	if isArbitrage {
		profitMargin = (1.0 - inverseSum) * 100.0
	}

	return isArbitrage, profitMargin
}

// CalculateStakes calculates optimal stake distribution for arbitrage
// Returns stake percentages for each leg (sums to 1.0)
func CalculateStakes(decimalOdds []float64, totalStake float64) []float64 {
	if len(decimalOdds) == 0 {
		return nil
	}

	inverseSum := 0.0
	for _, decimal := range decimalOdds {
		inverseSum += 1.0 / decimal
	}

	stakes := make([]float64, len(decimalOdds))
	for i, decimal := range decimalOdds {
		// Stake percentage = (1/decimal) / inverseSum
		stakePercent := (1.0 / decimal) / inverseSum
		stakes[i] = math.Round(totalStake * stakePercent * 100) / 100 // Round to cents
	}

	return stakes
}



