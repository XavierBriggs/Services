package oddsmath

import (
	"fmt"
	"math"
)

// RemoveVigMultiplicative removes vig from two-way markets using the multiplicative method
// This is the standard method for spreads, totals, and other two-outcome markets
//
// Formula:
// 1. Convert both sides to implied probabilities
// 2. Calculate overround: totalProb = prob1 + prob2 (typically > 1.0)
// 3. Normalize: fairProb1 = prob1 / totalProb, fairProb2 = prob2 / totalProb
// 4. Fair probabilities now sum to 1.0
//
// Example:
// Side A: -110 (52.38% implied) | Side B: -110 (52.38% implied)
// Overround: 104.76% (4.76% vig)
// Fair: 50% / 50% (after normalization)
func RemoveVigMultiplicative(prob1, prob2 float64) (fair1, fair2 float64, err error) {
	if prob1 <= 0 || prob1 >= 1 || prob2 <= 0 || prob2 >= 1 {
		return 0, 0, fmt.Errorf("probabilities must be between 0 and 1")
	}

	totalProb := prob1 + prob2

	if totalProb <= 1.0 {
		return 0, 0, fmt.Errorf("no vig detected: probabilities sum to <= 1.0")
	}

	// Normalize by dividing each probability by the total
	// This proportionally removes the vig
	fair1 = prob1 / totalProb
	fair2 = prob2 / totalProb

	return fair1, fair2, nil
}

// RemoveVigAdditive removes vig from three-way markets using the additive method
// This is used for moneylines with draws (soccer) or three-outcome markets
//
// Formula:
// 1. Convert all outcomes to implied probabilities
// 2. Calculate total overround
// 3. Subtract equal portions of vig from each outcome
//
// Note: For two-way markets (NBA moneyline), multiplicative is preferred
func RemoveVigAdditive(probabilities []float64) ([]float64, error) {
	if len(probabilities) < 2 {
		return nil, fmt.Errorf("need at least 2 outcomes")
	}

	for _, prob := range probabilities {
		if prob <= 0 || prob >= 1 {
			return nil, fmt.Errorf("all probabilities must be between 0 and 1")
		}
	}

	// Calculate total overround
	totalProb := 0.0
	for _, prob := range probabilities {
		totalProb += prob
	}

	if totalProb <= 1.0 {
		return nil, fmt.Errorf("no vig detected: probabilities sum to <= 1.0")
	}

	overround := totalProb - 1.0
	vigPerOutcome := overround / float64(len(probabilities))

	// Subtract equal vig from each outcome
	fairProbs := make([]float64, len(probabilities))
	for i, prob := range probabilities {
		fairProbs[i] = prob - vigPerOutcome
	}

	return fairProbs, nil
}

// CalculateEdge calculates the percentage edge of offered odds vs fair probability
// Edge = (Fair Probability / Implied Probability) - 1
//
// Example:
// Fair Probability: 50% (0.50)
// Offered Odds: +110 (47.6% implied)
// Edge: (0.50 / 0.476) - 1 = 0.05 = 5% edge
//
// Positive edge = +EV bet
// Negative edge = -EV bet
func CalculateEdge(fairProbability, impliedProbability float64) (float64, error) {
	if fairProbability <= 0 || fairProbability >= 1 {
		return 0, fmt.Errorf("fair probability must be between 0 and 1")
	}

	if impliedProbability <= 0 || impliedProbability >= 1 {
		return 0, fmt.Errorf("implied probability must be between 0 and 1")
	}

	// Edge = (Fair / Implied) - 1
	edge := (fairProbability / impliedProbability) - 1.0

	return edge, nil
}

// CalculateSharpConsensus calculates the average no-vig probability from sharp books
// This represents the "true" market probability based on efficient sharp book pricing
//
// Formula:
// 1. Remove vig from each sharp book's two-way market
// 2. Average the fair probabilities across all sharp books
// 3. Result is the sharp consensus fair probability
func CalculateSharpConsensus(sharpOdds []TwoWayMarket) (consensus1, consensus2 float64, err error) {
	if len(sharpOdds) == 0 {
		return 0, 0, fmt.Errorf("no sharp books provided")
	}

	var sumFair1, sumFair2 float64

	for _, market := range sharpOdds {
		// Remove vig from this sharp book
		fair1, fair2, err := RemoveVigMultiplicative(market.Prob1, market.Prob2)
		if err != nil {
			return 0, 0, fmt.Errorf("error removing vig from sharp book: %w", err)
		}

		sumFair1 += fair1
		sumFair2 += fair2
	}

	// Average across all sharp books
	consensus1 = sumFair1 / float64(len(sharpOdds))
	consensus2 = sumFair2 / float64(len(sharpOdds))

	return consensus1, consensus2, nil
}

// TwoWayMarket represents a two-outcome market with implied probabilities
type TwoWayMarket struct {
	Prob1 float64 // Probability of outcome 1 (e.g., Over, Team A)
	Prob2 float64 // Probability of outcome 2 (e.g., Under, Team B)
}

// CalculateVigPercentage calculates the vig (overround) percentage in a market
// Vig% = (TotalProb - 1.0) * 100
//
// Example:
// Outcome A: 52.38% | Outcome B: 52.38%
// Total: 104.76%
// Vig: 4.76%
func CalculateVigPercentage(probabilities []float64) (float64, error) {
	if len(probabilities) == 0 {
		return 0, fmt.Errorf("no probabilities provided")
	}

	totalProb := 0.0
	for _, prob := range probabilities {
		if prob <= 0 || prob >= 1 {
			return 0, fmt.Errorf("all probabilities must be between 0 and 1")
		}
		totalProb += prob
	}

	if totalProb <= 1.0 {
		return 0, nil // No vig
	}

	vigPct := (totalProb - 1.0) * 100.0

	return vigPct, nil
}

// RoundToNearestCent rounds a probability to the nearest 0.01%
// Useful for display purposes
func RoundToNearestCent(probability float64) float64 {
	return math.Round(probability*10000) / 10000
}





