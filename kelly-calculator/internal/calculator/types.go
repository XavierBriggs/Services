package calculator

import "math"

// americanToDecimal converts American odds to decimal odds
func americanToDecimal(american int) float64 {
	if american > 0 {
		return (float64(american) / 100.0) + 1.0
	}
	return (100.0 / float64(-american)) + 1.0
}

// decimalToAmerican converts decimal odds to American odds
func decimalToAmerican(decimal float64) int {
	if decimal >= 2.0 {
		return int((decimal - 1.0) * 100)
	}
	return int(-100.0 / (decimal - 1.0))
}

// round rounds a float to 2 decimal places
func round(val float64) float64 {
	return math.Round(val*100) / 100
}

// calculateImpliedProbability calculates implied probability from American odds
func calculateImpliedProbability(american int) float64 {
	decimal := americanToDecimal(american)
	return 1.0 / decimal
}




