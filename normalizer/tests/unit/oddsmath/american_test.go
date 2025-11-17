package oddsmath_test

import (
	"math"
	"testing"

	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/oddsmath"
)

func TestAmericanToDecimal(t *testing.T) {
	tests := []struct {
		name     string
		american int
		want     float64
	}{
		{"Positive odds +100", 100, 2.0},
		{"Positive odds +150", 150, 2.5},
		{"Positive odds +200", 200, 3.0},
		{"Negative odds -110", -110, 1.909090909},
		{"Negative odds -150", -150, 1.666666667},
		{"Negative odds -200", -200, 1.5},
		{"Even odds +100", 100, 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := oddsmath.AmericanToDecimal(tt.american)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Allow small floating point differences
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("AmericanToDecimal(%d) = %f, want %f", tt.american, got, tt.want)
			}
		})
	}
}

func TestDecimalToAmerican(t *testing.T) {
	tests := []struct {
		name    string
		decimal float64
		want    int
	}{
		{"Even odds 2.0", 2.0, 100},
		{"Underdog 2.5", 2.5, 150},
		{"Underdog 3.0", 3.0, 200},
		{"Favorite 1.909", 1.909, -110},
		{"Favorite 1.667", 1.667, -150},
		{"Favorite 1.5", 1.5, -200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := oddsmath.DecimalToAmerican(tt.decimal)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Allow ±1 for rounding
			diff := math.Abs(float64(got - tt.want))
			if diff > 2 {
				t.Errorf("DecimalToAmerican(%f) = %d, want %d", tt.decimal, got, tt.want)
			}
		})
	}
}

func TestImpliedProbability(t *testing.T) {
	tests := []struct {
		name     string
		american int
		want     float64
	}{
		{"Even odds +100", 100, 0.50},
		{"Favorite -110", -110, 0.5238},
		{"Heavy favorite -200", -200, 0.6667},
		{"Underdog +150", 150, 0.40},
		{"Heavy underdog +300", 300, 0.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decimal, err := oddsmath.AmericanToDecimal(tt.american)
			if err != nil {
				t.Fatalf("unexpected error converting to decimal: %v", err)
			}

			got, err := oddsmath.DecimalToImpliedProbability(decimal)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Allow 0.01 difference (1%)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("ImpliedProbability(%d) = %f, want %f", tt.american, got, tt.want)
			}
		})
	}
}

func TestProbabilityToAmerican(t *testing.T) {
	tests := []struct {
		name        string
		probability float64
		wantMin     int
		wantMax     int
	}{
		{"50% (even odds)", 0.50, 95, 105},
		{"52.38% (-110)", 0.5238, -115, -105},
		{"66.67% (-200)", 0.6667, -205, -195},
		{"40% (+150)", 0.40, 145, 155},
		{"25% (+300)", 0.25, 290, 310},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := oddsmath.ProbabilityToAmerican(tt.probability)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("ProbabilityToAmerican(%f) = %d, want between %d and %d", 
					tt.probability, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	tests := []int{-110, -150, -200, 100, 150, 200, 250, 300}

	for _, american := range tests {
		t.Run("", func(t *testing.T) {
			// American -> Decimal -> American
			decimal, err := oddsmath.AmericanToDecimal(american)
			if err != nil {
				t.Fatalf("error converting to decimal: %v", err)
			}

			got, err := oddsmath.DecimalToAmerican(decimal)
			if err != nil {
				t.Fatalf("error converting back to american: %v", err)
			}

			// Allow ±2 for rounding
			diff := math.Abs(float64(got - american))
			if diff > 2 {
				t.Errorf("Round trip: %d -> %f -> %d (diff: %f)", american, decimal, got, diff)
			}
		})
	}
}

func TestInvalidInputs(t *testing.T) {
	t.Run("AmericanToDecimal zero", func(t *testing.T) {
		_, err := oddsmath.AmericanToDecimal(0)
		if err == nil {
			t.Error("expected error for zero American odds")
		}
	})

	t.Run("DecimalToImpliedProbability zero", func(t *testing.T) {
		_, err := oddsmath.DecimalToImpliedProbability(0)
		if err == nil {
			t.Error("expected error for zero decimal odds")
		}
	})

	t.Run("ProbabilityToAmerican invalid", func(t *testing.T) {
		_, err := oddsmath.ProbabilityToAmerican(0)
		if err == nil {
			t.Error("expected error for 0% probability")
		}

		_, err = oddsmath.ProbabilityToAmerican(1.0)
		if err == nil {
			t.Error("expected error for 100% probability")
		}

		_, err = oddsmath.ProbabilityToAmerican(-0.5)
		if err == nil {
			t.Error("expected error for negative probability")
		}
	})
}





