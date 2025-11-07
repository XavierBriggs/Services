package oddsmath

import (
	"fmt"
	"math"
)

// AmericanToDecimal converts American odds to decimal odds
// American +150 → Decimal 2.50
// American -150 → Decimal 1.67
func AmericanToDecimal(american int) (float64, error) {
	if american == 0 {
		return 0, fmt.Errorf("invalid American odds: cannot be 0")
	}

	if american > 0 {
		// Positive odds: (american / 100) + 1
		return (float64(american) / 100.0) + 1.0, nil
	}

	// Negative odds: (100 / abs(american)) + 1
	return (100.0 / float64(-american)) + 1.0, nil
}

// DecimalToAmerican converts decimal odds to American odds
// Decimal 2.50 → American +150
// Decimal 1.67 → American -150
func DecimalToAmerican(decimal float64) (int, error) {
	if decimal < 1.0 {
		return 0, fmt.Errorf("invalid decimal odds: must be >= 1.0")
	}

	if decimal >= 2.0 {
		// Positive American odds: (decimal - 1) * 100
		return int(math.Round((decimal - 1.0) * 100.0)), nil
	}

	// Negative American odds: -100 / (decimal - 1)
	return int(math.Round(-100.0 / (decimal - 1.0))), nil
}

// DecimalToImpliedProbability converts decimal odds to implied probability
// Decimal 2.00 → 0.50 (50%)
// Decimal 1.50 → 0.667 (66.7%)
func DecimalToImpliedProbability(decimal float64) (float64, error) {
	if decimal <= 0 {
		return 0, fmt.Errorf("invalid decimal odds: must be > 0")
	}

	return 1.0 / decimal, nil
}

// ProbabilityToDecimal converts probability to decimal odds
// 0.50 (50%) → Decimal 2.00
// 0.667 (66.7%) → Decimal 1.50
func ProbabilityToDecimal(probability float64) (float64, error) {
	if probability <= 0 || probability >= 1 {
		return 0, fmt.Errorf("invalid probability: must be between 0 and 1")
	}

	return 1.0 / probability, nil
}

// AmericanToImpliedProbability converts American odds directly to implied probability
// Convenience function that combines AmericanToDecimal + DecimalToImpliedProbability
func AmericanToImpliedProbability(american int) (float64, error) {
	decimal, err := AmericanToDecimal(american)
	if err != nil {
		return 0, err
	}

	return DecimalToImpliedProbability(decimal)
}

// ProbabilityToAmerican converts probability directly to American odds
// Convenience function that combines ProbabilityToDecimal + DecimalToAmerican
func ProbabilityToAmerican(probability float64) (int, error) {
	decimal, err := ProbabilityToDecimal(probability)
	if err != nil {
		return 0, err
	}

	return DecimalToAmerican(decimal)
}

