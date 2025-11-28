package filters

import (
	"fmt"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// CorrelationFilter detects and adjusts for correlated bets (same game)
type CorrelationFilter struct{}

// NewCorrelationFilter creates a new correlation filter
func NewCorrelationFilter() *CorrelationFilter {
	return &CorrelationFilter{}
}

// Name returns the filter name
func (f *CorrelationFilter) Name() string {
	return "correlation"
}

// Evaluate checks for correlated exposure and applies discount
func (f *CorrelationFilter) Evaluate(
	opp models.Opportunity,
	settings models.UserSettings,
	state models.AutoBettingState,
) models.FilterResult {
	// Check if we already have exposure on this event
	eventExposure := state.ExposureByEvent[opp.EventID]

	if eventExposure > 0 {
		// We have correlated exposure - this is not a failure, but we'll apply a discount
		correlationFactor := settings.AutoCorrelationDiscount

		return models.FilterResult{
			Passed: true, // Still pass, but signal correlation
			Reason: fmt.Sprintf("correlated bet (existing exposure $%.2f), will apply %.1f%% discount", 
				eventExposure, correlationFactor*100),
			Metadata: map[string]interface{}{
				"existing_exposure":    eventExposure,
				"correlation_discount": correlationFactor,
				"is_correlated":        true,
				"event_id":             opp.EventID,
			},
		}
	}

	return models.FilterResult{
		Passed: true,
		Reason: "no correlation detected",
		Metadata: map[string]interface{}{
			"is_correlated": false,
			"event_id":      opp.EventID,
		},
	}
}


