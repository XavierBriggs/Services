package filters

import (
	"fmt"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// RiskManagementFilter checks exposure limits, rate limits, and circuit breakers
type RiskManagementFilter struct{}

// NewRiskManagementFilter creates a new risk management filter
func NewRiskManagementFilter() *RiskManagementFilter {
	return &RiskManagementFilter{}
}

// Name returns the filter name
func (f *RiskManagementFilter) Name() string {
	return "risk_management"
}

// Evaluate checks if opportunity passes risk management rules
func (f *RiskManagementFilter) Evaluate(
	opp models.Opportunity,
	settings models.UserSettings,
	state models.AutoBettingState,
) models.FilterResult {
	// Check total exposure limit
	if state.TotalExposure >= settings.AutoMaxExposureTotal {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("total exposure $%.2f >= limit $%.2f", 
				state.TotalExposure, settings.AutoMaxExposureTotal),
			Metadata: map[string]interface{}{
				"total_exposure": state.TotalExposure,
				"max_exposure":   settings.AutoMaxExposureTotal,
			},
		}
	}

	// Check per-event exposure limit
	eventExposure := state.ExposureByEvent[opp.EventID]
	if eventExposure >= settings.AutoMaxExposurePerEvent {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("event exposure $%.2f >= limit $%.2f", 
				eventExposure, settings.AutoMaxExposurePerEvent),
			Metadata: map[string]interface{}{
				"event_id":         opp.EventID,
				"event_exposure":   eventExposure,
				"max_event_exposure": settings.AutoMaxExposurePerEvent,
			},
		}
	}

	// Check hourly rate limit
	if state.BetsPlacedLastHour >= settings.AutoMaxBetsPerHour {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("hourly bet limit reached (%d/%d)", 
				state.BetsPlacedLastHour, settings.AutoMaxBetsPerHour),
			Metadata: map[string]interface{}{
				"bets_placed_hour": state.BetsPlacedLastHour,
				"max_bets_hour":    settings.AutoMaxBetsPerHour,
			},
		}
	}

	// Check daily rate limit
	if state.BetsPlacedToday >= settings.AutoMaxBetsPerDay {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("daily bet limit reached (%d/%d)", 
				state.BetsPlacedToday, settings.AutoMaxBetsPerDay),
			Metadata: map[string]interface{}{
				"bets_placed_today": state.BetsPlacedToday,
				"max_bets_day":      settings.AutoMaxBetsPerDay,
			},
		}
	}

	// Check loss streak circuit breaker
	if settings.AutoPauseOnLossStreak > 0 && state.CurrentLossStreak >= settings.AutoPauseOnLossStreak {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("paused due to loss streak (%d consecutive losses)", 
				state.CurrentLossStreak),
			Metadata: map[string]interface{}{
				"current_loss_streak": state.CurrentLossStreak,
				"pause_threshold":     settings.AutoPauseOnLossStreak,
			},
		}
	}

	// Check daily loss limit circuit breaker
	if settings.AutoPauseOnDailyLoss > 0 && state.TodaysPnL <= -settings.AutoPauseOnDailyLoss {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("paused due to daily loss ($%.2f)", 
				state.TodaysPnL),
			Metadata: map[string]interface{}{
				"todays_pnl":        state.TodaysPnL,
				"daily_loss_limit": settings.AutoPauseOnDailyLoss,
			},
		}
	}

	return models.FilterResult{
		Passed: true,
		Reason: "passed all risk management checks",
		Metadata: map[string]interface{}{
			"total_exposure":     state.TotalExposure,
			"bets_placed_today":  state.BetsPlacedToday,
			"current_loss_streak": state.CurrentLossStreak,
		},
	}
}


