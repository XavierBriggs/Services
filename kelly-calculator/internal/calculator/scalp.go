package calculator

import (
	"fmt"
	"github.com/XavierBriggs/fortuna/services/kelly-calculator/pkg/models"
)

// CalculateScalpStakes calculates optimal stake distribution for arbitrage
// Based on edge-detector/internal/detector/scalp_detector.go:218-238
func CalculateScalpStakes(opportunity models.Opportunity, totalStake float64) (*models.KellyResponse, error) {
	if len(opportunity.Legs) < 2 {
		return nil, fmt.Errorf("scalp requires at least 2 legs")
	}

	// Convert to decimal odds
	decimalOdds := make([]float64, len(opportunity.Legs))
	for i, leg := range opportunity.Legs {
		decimalOdds[i] = americanToDecimal(leg.Price)
	}

	// Calculate inverse sum to verify arbitrage exists
	inverseSum := 0.0
	for _, decimal := range decimalOdds {
		inverseSum += 1.0 / decimal
	}

	if inverseSum >= 1.0 {
		return nil, fmt.Errorf("no arbitrage exists: inverse sum = %.4f", inverseSum)
	}

	// Calculate profit margin
	profitMargin := (1.0 - inverseSum) * 100.0

	// Calculate stake for each leg
	stakes := make([]float64, len(decimalOdds))
	for i, decimal := range decimalOdds {
		stakePercent := (1.0 / decimal) / inverseSum
		stakes[i] = round(totalStake * stakePercent)
	}

	// Calculate potential returns (should all be equal)
	potentialReturn := round(stakes[0] * decimalOdds[0])
	guaranteedProfit := round(potentialReturn - totalStake)

	// Build response
	legs := make([]models.LegRecommendation, len(opportunity.Legs))
	for i, leg := range opportunity.Legs {
		returnVal := round(stakes[i] * decimalOdds[i])
		legs[i] = models.LegRecommendation{
			Book:            leg.BookKey,
			Outcome:         fmt.Sprintf("%s @ %+d", leg.OutcomeName, leg.Price),
			Stake:           stakes[i],
			PotentialReturn: &returnVal,
			Explanation:     fmt.Sprintf("Stake to guarantee $%.2f profit", guaranteedProfit),
		}
	}

	instructions := "Place both bets simultaneously for guaranteed profit"
	warnings := []string{}
	
	// Add warnings
	if profitMargin < 1.0 {
		warnings = append(warnings, "Low profit margin - consider transaction costs")
	}
	if totalStake > 1000 {
		warnings = append(warnings, "Book limits may prevent full stake")
	}

	return &models.KellyResponse{
		Type:             "scalp",
		TotalStake:       totalStake,
		GuaranteedProfit: &guaranteedProfit,
		ProfitPercent:    &profitMargin,
		Legs:             legs,
		Instructions:     &instructions,
		Warnings:         warnings,
	}, nil
}




