package sizing

import (
	"fmt"
	"math"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// ScalpSizer calculates equal profit distribution for scalp (arbitrage) opportunities
type ScalpSizer struct{}

// NewScalpSizer creates a new scalp sizer
func NewScalpSizer() *ScalpSizer {
	return &ScalpSizer{}
}

// CalculateStakes calculates stake distribution for scalp opportunities
// Formula: Stake_i = (Total_Stake / Sum(1/Odds_j)) * (1/Odds_i)
// This ensures equal profit on all outcomes
func (ss *ScalpSizer) CalculateStakes(
	opp models.Opportunity,
	totalStake float64,
) (map[string]float64, error) {
	if len(opp.Legs) < 2 {
		return nil, fmt.Errorf("scalp opportunities require at least 2 legs, got %d", len(opp.Legs))
	}

	// Convert American odds to decimal
	decimalOdds := make(map[string]float64)
	for _, leg := range opp.Legs {
		decimal := americanToDecimal(leg.Price)
		if decimal <= 1.0 {
			return nil, fmt.Errorf("invalid odds for %s: %d", leg.BookKey, leg.Price)
		}
		decimalOdds[leg.BookKey] = decimal
	}

	// Calculate sum of inverse odds
	sumInverseOdds := 0.0
	for _, decimal := range decimalOdds {
		sumInverseOdds += 1.0 / decimal
	}

	// Verify this is actually a scalp (sum < 1 means guaranteed profit)
	if sumInverseOdds >= 1.0 {
		return nil, fmt.Errorf("not a valid scalp: implied probability sum %.2f%% >= 100%%", 
			sumInverseOdds*100)
	}

	// Calculate stake for each leg
	stakes := make(map[string]float64)
	for bookKey, decimal := range decimalOdds {
		stake := (totalStake / sumInverseOdds) * (1.0 / decimal)
		stakes[bookKey] = roundToNearestDollar(stake)
	}

	// Verify equal profit (approximately)
	profits := make([]float64, 0, len(stakes))
	totalStakeAllocated := 0.0
	for bookKey, stake := range stakes {
		totalStakeAllocated += stake
		profit := stake*decimalOdds[bookKey] - totalStake
		profits = append(profits, profit)
	}

	// Check profit consistency (allow small rounding differences)
	if len(profits) > 0 {
		expectedProfit := profits[0]
		for _, profit := range profits {
			if math.Abs(profit-expectedProfit) > 0.50 { // Allow $0.50 difference due to rounding
				return nil, fmt.Errorf("profit inconsistency: expected $%.2f but got $%.2f", 
					expectedProfit, profit)
			}
		}
	}

	return stakes, nil
}

// americanToDecimal converts American odds to decimal odds
func americanToDecimal(american int) float64 {
	if american > 0 {
		return float64(american)/100.0 + 1.0
	}
	return (100.0 / float64(-american)) + 1.0
}

// roundToNearestDollar rounds a float to the nearest dollar
func roundToNearestDollar(value float64) float64 {
	return math.Round(value)
}


