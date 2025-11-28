package sizing

import (
	"testing"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

func TestParseBalance(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{
			name:  "simple dollar amount",
			input: "$1234.56",
			want:  1234.56,
		},
		{
			name:  "with comma separator",
			input: "$1,234.56",
			want:  1234.56,
		},
		{
			name:  "large amount",
			input: "$12,345.67",
			want:  12345.67,
		},
		{
			name:  "no dollar sign",
			input: "1234.56",
			want:  1234.56,
		},
		{
			name:  "with spaces",
			input: " $1,234.56 ",
			want:  1234.56,
		},
		{
			name:    "negative amount",
			input:   "$-100.00",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBalance(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseBalance() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("parseBalance() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("parseBalance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScalpSizer_CalculateStakes(t *testing.T) {
	sizer := NewScalpSizer()

	tests := []struct {
		name        string
		opp         models.Opportunity
		totalStake  float64
		wantErr     bool
		checkProfit bool
	}{
		{
			name: "valid 2-leg scalp",
			opp: models.Opportunity{
				Legs: []models.OpportunityLeg{
					{BookKey: "betus", Price: 110},   // 1.95 decimal
					{BookKey: "bovada", Price: -105}, // 1.95 decimal
				},
			},
			totalStake:  100.0,
			checkProfit: true,
		},
		{
			name: "valid 3-leg scalp",
			opp: models.Opportunity{
				Legs: []models.OpportunityLeg{
					{BookKey: "betus", Price: 200},
					{BookKey: "bovada", Price: 250},
					{BookKey: "draftkings", Price: 300},
				},
			},
			totalStake:  100.0,
			checkProfit: true,
		},
		{
			name: "not a valid scalp (overround)",
			opp: models.Opportunity{
				Legs: []models.OpportunityLeg{
					{BookKey: "betus", Price: -110},
					{BookKey: "bovada", Price: -110},
				},
			},
			totalStake: 100.0,
			wantErr:    true,
		},
		{
			name: "insufficient legs",
			opp: models.Opportunity{
				Legs: []models.OpportunityLeg{
					{BookKey: "betus", Price: 110},
				},
			},
			totalStake: 100.0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stakes, err := sizer.CalculateStakes(tt.opp, tt.totalStake)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CalculateStakes() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CalculateStakes() unexpected error: %v", err)
				return
			}

			if len(stakes) != len(tt.opp.Legs) {
				t.Errorf("CalculateStakes() got %d stakes, want %d", len(stakes), len(tt.opp.Legs))
			}

			// Verify stakes sum approximately to total
			totalAllocated := 0.0
			for _, stake := range stakes {
				totalAllocated += stake
			}

			// Allow 5% tolerance due to rounding
			if totalAllocated < tt.totalStake*0.95 || totalAllocated > tt.totalStake*1.05 {
				t.Errorf("Total allocated %v not close to target %v", totalAllocated, tt.totalStake)
			}

			// If checking profit, verify equal profit on all legs
			if tt.checkProfit {
				profits := []float64{}
				for _, leg := range tt.opp.Legs {
					stake := stakes[leg.BookKey]
					decimal := americanToDecimal(leg.Price)
					profit := stake*decimal - totalAllocated
					profits = append(profits, profit)
				}

				// All profits should be within $1 of each other
				for i := 1; i < len(profits); i++ {
					diff := profits[i] - profits[0]
					if diff < -1.0 || diff > 1.0 {
						t.Errorf("Profit inconsistency: profit[0]=%.2f, profit[%d]=%.2f",
							profits[0], i, profits[i])
					}
				}
			}
		})
	}
}

func TestAmericanToDecimal(t *testing.T) {
	tests := []struct {
		name      string
		american  int
		wantClose float64
	}{
		{name: "even odds", american: 100, wantClose: 2.0},
		{name: "favorite -110", american: -110, wantClose: 1.909},
		{name: "favorite -200", american: -200, wantClose: 1.5},
		{name: "underdog +150", american: 150, wantClose: 2.5},
		{name: "underdog +200", american: 200, wantClose: 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := americanToDecimal(tt.american)

			// Allow small tolerance for floating point
			diff := got - tt.wantClose
			if diff < -0.01 || diff > 0.01 {
				t.Errorf("americanToDecimal(%d) = %.3f, want %.3f",
					tt.american, got, tt.wantClose)
			}
		})
	}
}


