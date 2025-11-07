package oddsmath_test

import (
	"math"
	"testing"

	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/oddsmath"
)

func TestRemoveVigMultiplicative(t *testing.T) {
	tests := []struct {
		name       string
		prob1      float64
		prob2      float64
		wantFair1  float64
		wantFair2  float64
		shouldFail bool
	}{
		{
			name:      "Standard -110/-110 (4.76% vig)",
			prob1:     0.5238,
			prob2:     0.5238,
			wantFair1: 0.50,
			wantFair2: 0.50,
		},
		{
			name:      "Asymmetric -120/-110",
			prob1:     0.5455, // -120
			prob2:     0.5238, // -110
			wantFair1: 0.5099,
			wantFair2: 0.4901,
		},
		{
			name:      "Heavy favorite -200/+170",
			prob1:     0.6667, // -200
			prob2:     0.3704, // +170
			wantFair1: 0.6429,
			wantFair2: 0.3571,
		},
		{
			name:       "No vig (probabilities sum to 1.0)",
			prob1:      0.50,
			prob2:      0.50,
			shouldFail: true,
		},
		{
			name:       "Invalid probability > 1",
			prob1:      1.5,
			prob2:      0.5,
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fair1, fair2, err := oddsmath.RemoveVigMultiplicative(tt.prob1, tt.prob2)

			if tt.shouldFail {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check fair probabilities are close to expected
			if math.Abs(fair1-tt.wantFair1) > 0.01 {
				t.Errorf("fair1 = %f, want %f", fair1, tt.wantFair1)
			}

			if math.Abs(fair2-tt.wantFair2) > 0.01 {
				t.Errorf("fair2 = %f, want %f", fair2, tt.wantFair2)
			}

			// Fair probabilities should sum to 1.0
			sum := fair1 + fair2
			if math.Abs(sum-1.0) > 0.0001 {
				t.Errorf("fair probabilities don't sum to 1.0: %f + %f = %f", fair1, fair2, sum)
			}
		})
	}
}

func TestRemoveVigAdditive(t *testing.T) {
	tests := []struct {
		name          string
		probabilities []float64
		wantSum       float64
		shouldFail    bool
	}{
		{
			name:          "Three-way market with 5% vig",
			probabilities: []float64{0.45, 0.35, 0.25}, // Sums to 1.05
			wantSum:       1.0,
		},
		{
			name:          "Two-way market",
			probabilities: []float64{0.5238, 0.5238}, // -110/-110
			wantSum:       1.0,
		},
		{
			name:          "No vig",
			probabilities: []float64{0.50, 0.50},
			shouldFail:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fairProbs, err := oddsmath.RemoveVigAdditive(tt.probabilities)

			if tt.shouldFail {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check sum is 1.0
			sum := 0.0
			for _, prob := range fairProbs {
				sum += prob
			}

			if math.Abs(sum-tt.wantSum) > 0.0001 {
				t.Errorf("fair probabilities sum to %f, want %f", sum, tt.wantSum)
			}
		})
	}
}

func TestCalculateEdge(t *testing.T) {
	tests := []struct {
		name           string
		fairProb       float64
		impliedProb    float64
		wantEdge       float64
		wantPositiveEV bool
	}{
		{
			name:           "5% edge (+EV)",
			fairProb:       0.50,
			impliedProb:    0.476, // +110 odds
			wantEdge:       0.05,
			wantPositiveEV: true,
		},
		{
			name:           "No edge (fair odds)",
			fairProb:       0.50,
			impliedProb:    0.50,
			wantEdge:       0.0,
			wantPositiveEV: false,
		},
		{
			name:           "Negative edge (-EV)",
			fairProb:       0.45,
			impliedProb:    0.50,
			wantEdge:       -0.10,
			wantPositiveEV: false,
		},
		{
			name:           "10% edge (significant)",
			fairProb:       0.55,
			impliedProb:    0.50,
			wantEdge:       0.10,
			wantPositiveEV: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edge, err := oddsmath.CalculateEdge(tt.fairProb, tt.impliedProb)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Allow 1% difference
			if math.Abs(edge-tt.wantEdge) > 0.01 {
				t.Errorf("edge = %f, want %f", edge, tt.wantEdge)
			}

			isPositive := edge > 0
			if isPositive != tt.wantPositiveEV {
				t.Errorf("isPositiveEV = %v, want %v", isPositive, tt.wantPositiveEV)
			}
		})
	}
}

func TestCalculateSharpConsensus(t *testing.T) {
	tests := []struct {
		name         string
		sharpMarkets []oddsmath.TwoWayMarket
		wantProb1    float64
		wantProb2    float64
	}{
		{
			name: "Single sharp book",
			sharpMarkets: []oddsmath.TwoWayMarket{
				{Prob1: 0.5238, Prob2: 0.5238}, // -110/-110
			},
			wantProb1: 0.50,
			wantProb2: 0.50,
		},
		{
			name: "Two sharp books with slight difference",
			sharpMarkets: []oddsmath.TwoWayMarket{
				{Prob1: 0.5238, Prob2: 0.5238}, // -110/-110
				{Prob1: 0.5455, Prob2: 0.5128}, // -120/-105
			},
			wantProb1: 0.505,  // Average of no-vig fair prices
			wantProb2: 0.495,
		},
		{
			name: "Three sharp books (Pinnacle, Circa, Bookmaker)",
			sharpMarkets: []oddsmath.TwoWayMarket{
				{Prob1: 0.5238, Prob2: 0.5238}, // Pinnacle -110/-110
				{Prob1: 0.5455, Prob2: 0.5128}, // Circa -120/-105
				{Prob1: 0.5348, Prob2: 0.5348}, // Bookmaker -115/-115
			},
			wantProb1: 0.503,
			wantProb2: 0.497,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consensus1, consensus2, err := oddsmath.CalculateSharpConsensus(tt.sharpMarkets)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Allow 1% difference
			if math.Abs(consensus1-tt.wantProb1) > 0.01 {
				t.Errorf("consensus1 = %f, want %f", consensus1, tt.wantProb1)
			}

			if math.Abs(consensus2-tt.wantProb2) > 0.01 {
				t.Errorf("consensus2 = %f, want %f", consensus2, tt.wantProb2)
			}

			// Consensus should sum to 1.0
			sum := consensus1 + consensus2
			if math.Abs(sum-1.0) > 0.0001 {
				t.Errorf("consensus doesn't sum to 1.0: %f", sum)
			}
		})
	}
}

func TestCalculateVigPercentage(t *testing.T) {
	tests := []struct {
		name          string
		probabilities []float64
		wantVig       float64
	}{
		{
			name:          "Standard -110/-110",
			probabilities: []float64{0.5238, 0.5238},
			wantVig:       4.76,
		},
		{
			name:          "Heavy vig -120/-120",
			probabilities: []float64{0.5455, 0.5455},
			wantVig:       9.10,
		},
		{
			name:          "Three-way with 5% vig",
			probabilities: []float64{0.45, 0.35, 0.25},
			wantVig:       5.0,
		},
		{
			name:          "No vig (fair odds)",
			probabilities: []float64{0.50, 0.50},
			wantVig:       0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vig, err := oddsmath.CalculateVigPercentage(tt.probabilities)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Allow 0.5% difference
			if math.Abs(vig-tt.wantVig) > 0.5 {
				t.Errorf("vig = %f%%, want %f%%", vig, tt.wantVig)
			}
		})
	}
}

func TestRoundToNearestCent(t *testing.T) {
	tests := []struct {
		name  string
		prob  float64
		want  float64
	}{
		{"Already rounded", 0.5000, 0.5000},
		{"Round down", 0.50004, 0.5000},
		{"Round up", 0.50006, 0.5001},
		{"Three decimals", 0.523, 0.523},
		{"Many decimals", 0.52380952, 0.5238},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := oddsmath.RoundToNearestCent(tt.prob)
			if got != tt.want {
				t.Errorf("RoundToNearestCent(%f) = %f, want %f", tt.prob, got, tt.want)
			}
		})
	}
}

