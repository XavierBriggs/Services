package oddsmath

import "fmt"

// EdgeAnalysis contains detailed edge calculation results
type EdgeAnalysis struct {
	OfferedOdds        int     // American odds offered by book
	OfferedProbability float64 // Implied probability of offered odds
	FairProbability    float64 // True probability after vig removal
	FairOdds           int     // American odds equivalent of fair probability
	Edge               float64 // Percentage edge (positive = +EV)
	VigPercentage      float64 // Vig in the market
	IsPositiveEV       bool    // True if edge > 0
	IsSignificantEdge  bool    // True if edge >= threshold (e.g., 2%)
}

// AnalyzeEdge performs comprehensive edge analysis
// threshold is the minimum edge to be considered significant (e.g., 0.02 for 2%)
func AnalyzeEdge(offeredOdds int, fairProbability float64, threshold float64) (*EdgeAnalysis, error) {
	// Convert offered odds to probability
	decimal, err := AmericanToDecimal(offeredOdds)
	if err != nil {
		return nil, fmt.Errorf("invalid offered odds: %w", err)
	}

	offeredProb, err := DecimalToImpliedProbability(decimal)
	if err != nil {
		return nil, fmt.Errorf("error calculating implied probability: %w", err)
	}

	// Calculate edge
	edge, err := CalculateEdge(fairProbability, offeredProb)
	if err != nil {
		return nil, fmt.Errorf("error calculating edge: %w", err)
	}

	// Convert fair probability to American odds
	fairOdds, err := ProbabilityToAmerican(fairProbability)
	if err != nil {
		return nil, fmt.Errorf("error converting fair probability to odds: %w", err)
	}

	// Calculate vig (difference between offered and fair)
	vigPct := (offeredProb - fairProbability) * 100.0

	return &EdgeAnalysis{
		OfferedOdds:        offeredOdds,
		OfferedProbability: offeredProb,
		FairProbability:    fairProbability,
		FairOdds:           fairOdds,
		Edge:               edge,
		VigPercentage:      vigPct,
		IsPositiveEV:       edge > 0,
		IsSignificantEdge:  edge >= threshold,
	}, nil
}

// CompareToSharpConsensus compares a soft book's odds to sharp consensus
// Returns edge relative to the sharp market
func CompareToSharpConsensus(softBookOdds int, sharpConsensus float64) (*EdgeAnalysis, error) {
	return AnalyzeEdge(softBookOdds, sharpConsensus, 0.02) // 2% threshold
}

// CalculateEVDollar calculates expected value in dollars
// EV$ = (WinProb × WinAmount) - (LoseProb × StakeAmount)
func CalculateEVDollar(stake float64, offeredOdds int, fairProbability float64) (float64, error) {
	// Calculate potential win amount
	decimal, err := AmericanToDecimal(offeredOdds)
	if err != nil {
		return 0, err
	}

	totalPayout := stake * decimal
	winAmount := totalPayout - stake

	// EV = (P(win) × WinAmount) - (P(lose) × Stake)
	loseProb := 1.0 - fairProbability
	ev := (fairProbability * winAmount) - (loseProb * stake)

	return ev, nil
}

// CalculateROI calculates return on investment percentage
// ROI% = (Edge × 100)
func CalculateROI(edge float64) float64 {
	return edge * 100.0
}

// IsArbitrage checks if two odds create an arbitrage opportunity
// Arbitrage exists when: (1/decimal1) + (1/decimal2) < 1
func IsArbitrage(odds1, odds2 int) (bool, float64, error) {
	decimal1, err := AmericanToDecimal(odds1)
	if err != nil {
		return false, 0, err
	}

	decimal2, err := AmericanToDecimal(odds2)
	if err != nil {
		return false, 0, err
	}

	// Calculate inverse sum
	inverseSum := (1.0 / decimal1) + (1.0 / decimal2)

	// Arbitrage exists if sum < 1.0
	isArb := inverseSum < 1.0

	// Profit margin (if arbitrage exists)
	profitMargin := 0.0
	if isArb {
		profitMargin = (1.0 - inverseSum) * 100.0
	}

	return isArb, profitMargin, nil
}

// DetectMiddle checks if two opposite sides of a market both have positive EV
// A middle exists when both sides beat the fair price
func DetectMiddle(odds1, odds2 int, fairProb1, fairProb2 float64) (bool, float64, float64, error) {
	edge1, err := CalculateEdge(fairProb1, 1.0/float64(odds1))
	if err != nil {
		// Try converting from American first
		decimal1, err := AmericanToDecimal(odds1)
		if err != nil {
			return false, 0, 0, err
		}
		impliedProb1, _ := DecimalToImpliedProbability(decimal1)
		edge1, err = CalculateEdge(fairProb1, impliedProb1)
		if err != nil {
			return false, 0, 0, err
		}
	}

	edge2, err := CalculateEdge(fairProb2, 1.0/float64(odds2))
	if err != nil {
		// Try converting from American first
		decimal2, err := AmericanToDecimal(odds2)
		if err != nil {
			return false, 0, 0, err
		}
		impliedProb2, _ := DecimalToImpliedProbability(decimal2)
		edge2, err = CalculateEdge(fairProb2, impliedProb2)
		if err != nil {
			return false, 0, 0, err
		}
	}

	// Middle exists if BOTH sides have positive EV
	isMiddle := edge1 > 0 && edge2 > 0

	return isMiddle, edge1, edge2, nil
}

