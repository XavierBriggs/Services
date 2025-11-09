package calculator

import (
	"fmt"
	"github.com/XavierBriggs/fortuna/services/kelly-calculator/pkg/models"
)

// CalculateMiddleKelly calculates dual independent Kelly stakes for a middle
func CalculateMiddleKelly(opportunity models.Opportunity, bankroll, kellyFraction, minEdge, maxPct float64) (*models.KellyResponse, error) {
	if len(opportunity.Legs) != 2 {
		return nil, fmt.Errorf("middle bet must have exactly 2 legs")
	}

	legs := make([]models.LegRecommendation, 2)
	totalStake := 0.0

	// Calculate Kelly for each leg independently
	for i, leg := range opportunity.Legs {
		var edgePercent float64
		if leg.LegEdgePercent != nil {
			edgePercent = *leg.LegEdgePercent
		} else {
			// If no leg-specific edge, use half of total edge
			edgePercent = opportunity.EdgePercent / 2.0
		}

		// Check minimum edge
		if edgePercent < minEdge*100 {
			return nil, fmt.Errorf("leg %d edge %.2f%% is below minimum %.1f%%", i+1, edgePercent, minEdge*100)
		}

		// Calculate fair probability from edge
		impliedProb := calculateImpliedProbability(leg.Price)
		fairProb := (edgePercent/100 + 1.0) * impliedProb

		if fairProb >= 1.0 {
			return nil, fmt.Errorf("invalid fair probability for leg %d: %.4f", i+1, fairProb)
		}

		// Calculate Kelly percentage
		decimal := americanToDecimal(leg.Price)
		b := decimal - 1.0
		p := fairProb
		q := 1.0 - fairProb

		kellyPct := (b*p - q) / b

		if kellyPct <= 0 {
			return nil, fmt.Errorf("negative Kelly for leg %d: %.4f", i+1, kellyPct)
		}

		// Apply fractional Kelly
		fractionalKelly := kellyPct * kellyFraction

		// Cap at maximum percentage
		if fractionalKelly > maxPct {
			fractionalKelly = maxPct
		}

		// Calculate stakes
		fullKellyStake := round(bankroll * kellyPct)
		fractionalKellyStake := round(bankroll * fractionalKelly)

		totalStake += fractionalKellyStake

		legs[i] = models.LegRecommendation{
			Book:            leg.BookKey,
			Outcome:         fmt.Sprintf("%s @ %+d", leg.OutcomeName, leg.Price),
			Stake:           fractionalKellyStake,
			FullKelly:       &fullKellyStake,
			FractionalKelly: &fractionalKellyStake,
			EdgePercent:     &edgePercent,
			Explanation:     fmt.Sprintf("1/%.0f Kelly for %s", 1.0/kellyFraction, leg.OutcomeName),
		}
	}

	// Build response
	bestCase := "Both win if final lands on middle number exactly"
	worstCase := "One side wins (still +EV)"
	instructions := "Bet both sides independently for middle opportunity"

	warnings := []string{}
	if opportunity.EdgePercent < 2.0 {
		warnings = append(warnings, "Combined edge <2% - consider passing")
	}
	if totalStake > bankroll*0.10 {
		warnings = append(warnings, "Total position is >10% of bankroll")
	}

	return &models.KellyResponse{
		Type:         "middle",
		TotalStake:   totalStake,
		Legs:         legs,
		BestCase:     &bestCase,
		WorstCase:    &worstCase,
		Instructions: &instructions,
		Warnings:     warnings,
	}, nil
}

