package execution

import (
	"testing"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

func TestEdgeStrategy_Plan(t *testing.T) {
	strategy := NewEdgeStrategy(nil)

	tests := []struct {
		name     string
		opp      models.Opportunity
		stakes   map[string]float64
		wantErr  bool
		wantLegs int
	}{
		{
			name: "valid single leg",
			opp: models.Opportunity{
				ID:              1,
				OpportunityType: "edge",
				Legs: []models.OpportunityLeg{
					{
						ID:          10,
						BookKey:     "betus",
						OutcomeName: "Team A",
						Price:       110,
					},
				},
			},
			stakes: map[string]float64{
				"betus": 50.0,
			},
			wantLegs: 1,
		},
		{
			name: "invalid multiple legs",
			opp: models.Opportunity{
				OpportunityType: "edge",
				Legs: []models.OpportunityLeg{
					{BookKey: "betus"},
					{BookKey: "bovada"},
				},
			},
			stakes:  map[string]float64{},
			wantErr: true,
		},
		{
			name: "missing stake",
			opp: models.Opportunity{
				OpportunityType: "edge",
				Legs: []models.OpportunityLeg{
					{BookKey: "betus"},
				},
			},
			stakes:  map[string]float64{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := strategy.Plan(
				tt.opp,
				models.UserSettings{},
				models.AutoBettingState{},
				1000.0,
				tt.stakes,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("EdgeStrategy.Plan() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("EdgeStrategy.Plan() unexpected error: %v", err)
				return
			}

			if len(plan.Legs) != tt.wantLegs {
				t.Errorf("EdgeStrategy.Plan() got %d legs, want %d", len(plan.Legs), tt.wantLegs)
			}

			if plan.Strategy != "sequential" {
				t.Errorf("EdgeStrategy.Plan() strategy = %s, want sequential", plan.Strategy)
			}

			if plan.RequiredLegs != 1 {
				t.Errorf("EdgeStrategy.Plan() required legs = %d, want 1", plan.RequiredLegs)
			}
		})
	}
}

func TestMiddleStrategy_Plan(t *testing.T) {
	strategy := NewMiddleStrategy(nil)

	tests := []struct {
		name     string
		opp      models.Opportunity
		settings models.UserSettings
		stakes   map[string]float64
		wantErr  bool
	}{
		{
			name: "valid 2-leg middle",
			opp: models.Opportunity{
				OpportunityType: "middle",
				Legs: []models.OpportunityLeg{
					{BookKey: "betus", LegEdgePct: 2.5},
					{BookKey: "bovada", LegEdgePct: 3.0},
				},
			},
			settings: models.UserSettings{
				AutoMiddleExecutionStrategy: "parallel",
				AutoMiddleRequiredLegs:      2,
			},
			stakes: map[string]float64{
				"betus":  50.0,
				"bovada": 50.0,
			},
		},
		{
			name: "invalid leg count",
			opp: models.Opportunity{
				OpportunityType: "middle",
				Legs: []models.OpportunityLeg{
					{BookKey: "betus"},
				},
			},
			stakes:  map[string]float64{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := strategy.Plan(
				tt.opp,
				tt.settings,
				models.AutoBettingState{},
				1000.0,
				tt.stakes,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("MiddleStrategy.Plan() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("MiddleStrategy.Plan() unexpected error: %v", err)
				return
			}

			if plan.OpportunityType != "middle" {
				t.Errorf("MiddleStrategy.Plan() type = %s, want middle", plan.OpportunityType)
			}

			if len(plan.Legs) != 2 {
				t.Errorf("MiddleStrategy.Plan() got %d legs, want 2", len(plan.Legs))
			}
		})
	}
}

func TestScalpStrategy_Plan(t *testing.T) {
	strategy := NewScalpStrategy(nil)

	tests := []struct {
		name     string
		opp      models.Opportunity
		stakes   map[string]float64
		wantErr  bool
		wantLegs int
	}{
		{
			name: "valid 2-leg scalp",
			opp: models.Opportunity{
				OpportunityType: "scalp",
				Legs: []models.OpportunityLeg{
					{BookKey: "betus"},
					{BookKey: "bovada"},
				},
			},
			stakes: map[string]float64{
				"betus":  50.0,
				"bovada": 50.0,
			},
			wantLegs: 2,
		},
		{
			name: "valid 3-leg scalp",
			opp: models.Opportunity{
				OpportunityType: "scalp",
				Legs: []models.OpportunityLeg{
					{BookKey: "betus"},
					{BookKey: "bovada"},
					{BookKey: "draftkings"},
				},
			},
			stakes: map[string]float64{
				"betus":       33.0,
				"bovada":      33.0,
				"draftkings":  34.0,
			},
			wantLegs: 3,
		},
		{
			name: "insufficient legs",
			opp: models.Opportunity{
				OpportunityType: "scalp",
				Legs: []models.OpportunityLeg{
					{BookKey: "betus"},
				},
			},
			stakes:  map[string]float64{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := strategy.Plan(
				tt.opp,
				models.UserSettings{},
				models.AutoBettingState{},
				1000.0,
				tt.stakes,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ScalpStrategy.Plan() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ScalpStrategy.Plan() unexpected error: %v", err)
				return
			}

			if len(plan.Legs) != tt.wantLegs {
				t.Errorf("ScalpStrategy.Plan() got %d legs, want %d", len(plan.Legs), tt.wantLegs)
			}

			if plan.Strategy != "parallel" {
				t.Errorf("ScalpStrategy.Plan() strategy = %s, want parallel", plan.Strategy)
			}

			if plan.RequiredLegs != len(tt.opp.Legs) {
				t.Errorf("ScalpStrategy.Plan() required legs = %d, want %d",
					plan.RequiredLegs, len(tt.opp.Legs))
			}
		})
	}
}


