package calculator

import (
	"fmt"
	"github.com/XavierBriggs/fortuna/services/kelly-calculator/pkg/models"
)

// CalculateEdgeKelly calculates Kelly Criterion stake for an edge bet
func CalculateEdgeKelly(opportunity models.Opportunity, bankroll, kellyFraction, minEdge, maxPct float64) (*models.KellyResponse, error) {
	if len(opportunity.Legs) != 1 {
		return nil, fmt.Errorf("edge bet must have exactly 1 leg")
	}

	leg := opportunity.Legs[0]
	edgePercent := opportunity.EdgePercent

	// Check minimum edge
	if edgePercent < minEdge*100 {
		return nil, fmt.Errorf("edge %.2f%% is below minimum %.1f%%", edgePercent, minEdge*100)
	}

	// Calculate fair probability from edge
	// edge = (fairProb / impliedProb) - 1
	// fairProb = (edge + 1) * impliedProb
	impliedProb := calculateImpliedProbability(leg.Price)
	fairProb := (edgePercent/100 + 1.0) * impliedProb

	if fairProb >= 1.0 {
		return nil, fmt.Errorf("invalid fair probability: %.4f", fairProb)
	}

	// Calculate Kelly percentage
	decimal := americanToDecimal(leg.Price)
	b := decimal - 1.0 // Net odds
	p := fairProb
	q := 1.0 - fairProb

	kellyPct := (b*p - q) / b

	if kellyPct <= 0 {
		return nil, fmt.Errorf("negative Kelly: %.4f (no edge)", kellyPct)
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

	// Calculate EV per dollar
	evPerDollar := round(edgePercent / 100.0)

	// Determine confidence level
	confidence := "medium"
	if edgePercent > 5.0 {
		confidence = "high"
	} else if edgePercent < 2.0 {
		confidence = "low"
	}

	// Build warnings
	warnings := []string{}
	if edgePercent < 2.0 {
		warnings = append(warnings, "Edge is below 2% - consider passing")
	}
	if fractionalKellyStake > bankroll*0.05 {
		warnings = append(warnings, "Recommended bet is >5% of bankroll - high variance")
	}
	if fairProb > 0.6 || fairProb < 0.4 {
		warnings = append(warnings, "Fair probability estimate uncertainty: ±3%")
	}

	legRec := models.LegRecommendation{
		Book:            leg.BookKey,
		Outcome:         fmt.Sprintf("%s @ %+d", leg.OutcomeName, leg.Price),
		Stake:           fractionalKellyStake,
		FullKelly:       &fullKellyStake,
		FractionalKelly: &fractionalKellyStake,
		EdgePercent:     &edgePercent,
		EVPerDollar:     &evPerDollar,
		Explanation:     fmt.Sprintf("1/%.0f Kelly sizing (conservative)", 1.0/kellyFraction),
	}

	return &models.KellyResponse{
		Type:       "edge",
		TotalStake: fractionalKellyStake,
		Legs:       []models.LegRecommendation{legRec},
		Confidence: &confidence,
		Warnings:   warnings,
	}, nil
}



