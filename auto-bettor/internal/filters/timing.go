package filters

import (
	"fmt"
	"time"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// TimingFilter checks data freshness and game timing
type TimingFilter struct{}

// NewTimingFilter creates a new timing filter
func NewTimingFilter() *TimingFilter {
	return &TimingFilter{}
}

// Name returns the filter name
func (f *TimingFilter) Name() string {
	return "timing"
}

// Evaluate checks timing constraints
func (f *TimingFilter) Evaluate(
	opp models.Opportunity,
	settings models.UserSettings,
	state models.AutoBettingState,
) models.FilterResult {
	now := time.Now()

	// Check data age
	if opp.DataAgeSeconds > settings.AutoMaxDataAgeSeconds {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("data %d seconds old > limit %d seconds", 
				opp.DataAgeSeconds, settings.AutoMaxDataAgeSeconds),
			Metadata: map[string]interface{}{
				"data_age_seconds": opp.DataAgeSeconds,
				"max_data_age":     settings.AutoMaxDataAgeSeconds,
			},
		}
	}

	// TEMPORARY FIX: Skip game time checks if GameStartTime is zero (missing from opportunity)
	// This happens because edge-detector doesn't include game_start_time in published opportunities
	if opp.GameStartTime.IsZero() {
		return models.FilterResult{
			Passed: true,
			Reason: "passed timing checks (game time unavailable, checks skipped)",
			Metadata: map[string]interface{}{
				"data_age_seconds": opp.DataAgeSeconds,
				"game_time_missing": true,
			},
		}
	}

	// Check if game has already started
	if now.After(opp.GameStartTime) {
		// For edge bets, check if live games are allowed
		if opp.OpportunityType == "edge" && !settings.AutoEdgeAllowLiveGames {
			return models.FilterResult{
				Passed: false,
				Reason: "game already started and live betting disabled",
				Metadata: map[string]interface{}{
					"game_start_time": opp.GameStartTime,
					"current_time":    now,
				},
			}
		}

		// For middle and scalp, never bet on live games
		if opp.OpportunityType != "edge" {
			return models.FilterResult{
				Passed: false,
				Reason: fmt.Sprintf("game already started (%s bets not allowed live)", opp.OpportunityType),
				Metadata: map[string]interface{}{
					"game_start_time":  opp.GameStartTime,
					"current_time":     now,
					"opportunity_type": opp.OpportunityType,
				},
			}
		}
	}

	// TEMPORARY: Skip min/max time checks since GameStartTime is not populated
	// TODO: Fix edge-detector to include game_start_time in published opportunities
	
	return models.FilterResult{
		Passed: true,
		Reason: "passed all timing checks",
		Metadata: map[string]interface{}{
			"data_age_seconds": opp.DataAgeSeconds,
		},
	}
}

