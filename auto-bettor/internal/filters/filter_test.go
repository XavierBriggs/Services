package filters

import (
	"testing"
	"time"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

func TestUserPreferencesFilter(t *testing.T) {
	filter := NewUserPreferencesFilter()

	tests := []struct {
		name     string
		opp      models.Opportunity
		settings models.UserSettings
		wantPass bool
	}{
		{
			name: "passes all checks",
			opp: models.Opportunity{
				OpportunityType: "edge",
				MarketKey:       "spreads",
				EdgePct:         3.5,
				Legs: []models.OpportunityLeg{
					{BookKey: "betus"},
				},
			},
			settings: models.UserSettings{
				AutoEnabledOpportunityTypes: []string{"edge"},
				AutoEnabledMarkets:          []string{"spreads"},
				AutoEnabledBooks:            []string{"betus"},
				AutoMinEdgePct:              2.0,
			},
			wantPass: true,
		},
		{
			name: "fails on disabled opportunity type",
			opp: models.Opportunity{
				OpportunityType: "middle",
				MarketKey:       "spreads",
				EdgePct:         3.5,
			},
			settings: models.UserSettings{
				AutoEnabledOpportunityTypes: []string{"edge"},
				AutoMinEdgePct:              2.0,
			},
			wantPass: false,
		},
		{
			name: "fails on low edge",
			opp: models.Opportunity{
				OpportunityType: "edge",
				MarketKey:       "spreads",
				EdgePct:         1.5,
			},
			settings: models.UserSettings{
				AutoEnabledOpportunityTypes: []string{"edge"},
				AutoEnabledMarkets:          []string{"spreads"},
				AutoMinEdgePct:              2.0,
			},
			wantPass: false,
		},
		{
			name: "fails on blacklisted book",
			opp: models.Opportunity{
				OpportunityType: "edge",
				MarketKey:       "spreads",
				EdgePct:         3.5,
				Legs: []models.OpportunityLeg{
					{BookKey: "draftkings"},
				},
			},
			settings: models.UserSettings{
				AutoEnabledOpportunityTypes: []string{"edge"},
				AutoEnabledMarkets:          []string{"spreads"},
				AutoDisabledBooks:           []string{"draftkings"},
				AutoMinEdgePct:              2.0,
			},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Evaluate(tt.opp, tt.settings, models.AutoBettingState{})

			if result.Passed != tt.wantPass {
				t.Errorf("UserPreferencesFilter.Evaluate() passed = %v, want %v. Reason: %s",
					result.Passed, tt.wantPass, result.Reason)
			}
		})
	}
}

func TestRiskManagementFilter(t *testing.T) {
	filter := NewRiskManagementFilter()

	tests := []struct {
		name     string
		opp      models.Opportunity
		settings models.UserSettings
		state    models.AutoBettingState
		wantPass bool
	}{
		{
			name: "passes all checks",
			opp:  models.Opportunity{EventID: "event1"},
			settings: models.UserSettings{
				AutoMaxExposureTotal:    1000.0,
				AutoMaxExposurePerEvent: 200.0,
				AutoMaxBetsPerHour:      10,
				AutoMaxBetsPerDay:       50,
			},
			state: models.AutoBettingState{
				TotalExposure:      500.0,
				ExposureByEvent:    map[string]float64{"event1": 100.0},
				BetsPlacedLastHour: 5,
				BetsPlacedToday:    25,
			},
			wantPass: true,
		},
		{
			name: "fails on total exposure limit",
			opp:  models.Opportunity{EventID: "event1"},
			settings: models.UserSettings{
				AutoMaxExposureTotal: 1000.0,
			},
			state: models.AutoBettingState{
				TotalExposure: 1000.0,
			},
			wantPass: false,
		},
		{
			name: "fails on event exposure limit",
			opp:  models.Opportunity{EventID: "event1"},
			settings: models.UserSettings{
				AutoMaxExposurePerEvent: 200.0,
			},
			state: models.AutoBettingState{
				ExposureByEvent: map[string]float64{"event1": 200.0},
			},
			wantPass: false,
		},
		{
			name: "fails on hourly rate limit",
			opp:  models.Opportunity{},
			settings: models.UserSettings{
				AutoMaxBetsPerHour: 10,
			},
			state: models.AutoBettingState{
				BetsPlacedLastHour: 10,
			},
			wantPass: false,
		},
		{
			name: "fails on loss streak",
			opp:  models.Opportunity{},
			settings: models.UserSettings{
				AutoPauseOnLossStreak: 5,
			},
			state: models.AutoBettingState{
				CurrentLossStreak: 5,
			},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Evaluate(tt.opp, tt.settings, tt.state)

			if result.Passed != tt.wantPass {
				t.Errorf("RiskManagementFilter.Evaluate() passed = %v, want %v. Reason: %s",
					result.Passed, tt.wantPass, result.Reason)
			}
		})
	}
}

func TestTimingFilter(t *testing.T) {
	filter := NewTimingFilter()
	now := time.Now()

	tests := []struct {
		name     string
		opp      models.Opportunity
		settings models.UserSettings
		wantPass bool
	}{
		{
			name: "passes all timing checks",
			opp: models.Opportunity{
				DataAgeSeconds: 10,
				GameStartTime:  now.Add(2 * time.Hour),
			},
			settings: models.UserSettings{
				AutoMaxDataAgeSeconds:   30,
				AutoMinTimeToStartHours: 1,
				AutoMaxTimeToStartHours: 72,
			},
			wantPass: true,
		},
		{
			name: "fails on stale data",
			opp: models.Opportunity{
				DataAgeSeconds: 60,
				GameStartTime:  now.Add(2 * time.Hour),
			},
			settings: models.UserSettings{
				AutoMaxDataAgeSeconds: 30,
			},
			wantPass: false,
		},
		{
			name: "fails on game too soon",
			opp: models.Opportunity{
				DataAgeSeconds: 10,
				GameStartTime:  now.Add(30 * time.Minute),
			},
			settings: models.UserSettings{
				AutoMaxDataAgeSeconds:   30,
				AutoMinTimeToStartHours: 1,
			},
			wantPass: false,
		},
		{
			name: "fails on game too far away",
			opp: models.Opportunity{
				DataAgeSeconds: 10,
				GameStartTime:  now.Add(100 * time.Hour),
			},
			settings: models.UserSettings{
				AutoMaxDataAgeSeconds:   30,
				AutoMaxTimeToStartHours: 72,
			},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Evaluate(tt.opp, tt.settings, models.AutoBettingState{})

			if result.Passed != tt.wantPass {
				t.Errorf("TimingFilter.Evaluate() passed = %v, want %v. Reason: %s",
					result.Passed, tt.wantPass, result.Reason)
			}
		})
	}
}

func TestCorrelationFilter(t *testing.T) {
	filter := NewCorrelationFilter()

	tests := []struct {
		name            string
		opp             models.Opportunity
		settings        models.UserSettings
		state           models.AutoBettingState
		wantPass        bool
		wantCorrelation bool
	}{
		{
			name: "no correlation",
			opp:  models.Opportunity{EventID: "event1"},
			state: models.AutoBettingState{
				ExposureByEvent: map[string]float64{},
			},
			wantPass:        true,
			wantCorrelation: false,
		},
		{
			name: "has correlation",
			opp:  models.Opportunity{EventID: "event1"},
			state: models.AutoBettingState{
				ExposureByEvent: map[string]float64{"event1": 100.0},
			},
			settings: models.UserSettings{
				AutoCorrelationDiscount: 0.5,
			},
			wantPass:        true,
			wantCorrelation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Evaluate(tt.opp, tt.settings, tt.state)

			if result.Passed != tt.wantPass {
				t.Errorf("CorrelationFilter.Evaluate() passed = %v, want %v",
					result.Passed, tt.wantPass)
			}

			if isCorr, ok := result.Metadata["is_correlated"].(bool); ok {
				if isCorr != tt.wantCorrelation {
					t.Errorf("CorrelationFilter.Evaluate() is_correlated = %v, want %v",
						isCorr, tt.wantCorrelation)
				}
			}
		})
	}
}

func TestFilterChain(t *testing.T) {
	chain := NewFilterChain(
		NewUserPreferencesFilter(),
		NewRiskManagementFilter(),
	)

	opp := models.Opportunity{
		OpportunityType: "edge",
		MarketKey:       "spreads",
		EdgePct:         3.5,
		Legs:            []models.OpportunityLeg{{BookKey: "betus"}},
	}

	settings := models.UserSettings{
		AutoEnabledOpportunityTypes: []string{"edge"},
		AutoEnabledMarkets:          []string{"spreads"},
		AutoEnabledBooks:            []string{"betus"},
		AutoMinEdgePct:              2.0,
		AutoMaxExposureTotal:        1000.0,
		AutoMaxBetsPerHour:          10,
	}

	state := models.AutoBettingState{
		TotalExposure:      500.0,
		BetsPlacedLastHour: 5,
	}

	passed, results := chain.Evaluate(opp, settings, state)

	if !passed {
		t.Errorf("FilterChain should pass, but failed")
	}

	if len(results) != 2 {
		t.Errorf("FilterChain should have 2 results, got %d", len(results))
	}
}


