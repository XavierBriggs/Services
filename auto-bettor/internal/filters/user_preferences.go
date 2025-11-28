package filters

import (
	"fmt"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// UserPreferencesFilter checks if opportunity matches user's configured preferences
type UserPreferencesFilter struct{}

// NewUserPreferencesFilter creates a new user preferences filter
func NewUserPreferencesFilter() *UserPreferencesFilter {
	return &UserPreferencesFilter{}
}

// Name returns the filter name
func (f *UserPreferencesFilter) Name() string {
	return "user_preferences"
}

// Evaluate checks if opportunity matches user preferences
func (f *UserPreferencesFilter) Evaluate(
	opp models.Opportunity,
	settings models.UserSettings,
	state models.AutoBettingState,
) models.FilterResult {
	// Check if opportunity type is enabled
	if !contains(settings.AutoEnabledOpportunityTypes, opp.OpportunityType) {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("opportunity type '%s' not enabled (enabled types: %v)", 
				opp.OpportunityType, settings.AutoEnabledOpportunityTypes),
			Metadata: map[string]interface{}{
				"opportunity_type": opp.OpportunityType,
				"enabled_types":    settings.AutoEnabledOpportunityTypes,
			},
		}
	}

	// Check additional type-specific enables
	if opp.OpportunityType == "middle" && !settings.AutoMiddleEnabled {
		return models.FilterResult{
			Passed: false,
			Reason: "middle betting specifically disabled",
			Metadata: map[string]interface{}{
				"opportunity_type": opp.OpportunityType,
			},
		}
	}

	if opp.OpportunityType == "scalp" && !settings.AutoScalpEnabled {
		return models.FilterResult{
			Passed: false,
			Reason: "scalp betting specifically disabled",
			Metadata: map[string]interface{}{
				"opportunity_type": opp.OpportunityType,
			},
		}
	}

	// Check if market type is enabled
	if !contains(settings.AutoEnabledMarkets, opp.MarketKey) {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("market '%s' not enabled (enabled markets: %v)", 
				opp.MarketKey, settings.AutoEnabledMarkets),
			Metadata: map[string]interface{}{
				"market_key":      opp.MarketKey,
				"enabled_markets": settings.AutoEnabledMarkets,
			},
		}
	}

	// Check book filters (whitelist if provided, otherwise check blacklist)
	for _, leg := range opp.Legs {
		if len(settings.AutoEnabledBooks) > 0 {
			// Whitelist mode: book must be in enabled list
			if !contains(settings.AutoEnabledBooks, leg.BookKey) {
				return models.FilterResult{
					Passed: false,
					Reason: fmt.Sprintf("book '%s' not in whitelist (enabled books: %v)", 
						leg.BookKey, settings.AutoEnabledBooks),
					Metadata: map[string]interface{}{
						"book_key":      leg.BookKey,
						"enabled_books": settings.AutoEnabledBooks,
					},
				}
			}
		} else {
			// Blacklist mode: book must not be in disabled list
			if contains(settings.AutoDisabledBooks, leg.BookKey) {
				return models.FilterResult{
					Passed: false,
					Reason: fmt.Sprintf("book '%s' is blacklisted (disabled books: %v)", 
						leg.BookKey, settings.AutoDisabledBooks),
					Metadata: map[string]interface{}{
						"book_key":       leg.BookKey,
						"disabled_books": settings.AutoDisabledBooks,
					},
				}
			}
		}
	}

	// Check minimum edge threshold
	if opp.EdgePct < settings.AutoMinEdgePct {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("edge %.2f%% < minimum %.2f%%", 
				opp.EdgePct, settings.AutoMinEdgePct),
			Metadata: map[string]interface{}{
				"edge_pct":     opp.EdgePct,
				"min_edge_pct": settings.AutoMinEdgePct,
			},
		}
	}

	// For scalp opportunities, check minimum profit percentage
	if opp.OpportunityType == "scalp" {
		if opp.EdgePct < settings.AutoScalpMinProfitPct {
			return models.FilterResult{
				Passed: false,
				Reason: fmt.Sprintf("scalp profit %.2f%% < minimum %.2f%%", 
					opp.EdgePct, settings.AutoScalpMinProfitPct),
				Metadata: map[string]interface{}{
					"profit_pct":     opp.EdgePct,
					"min_profit_pct": settings.AutoScalpMinProfitPct,
				},
			}
		}
	}

	return models.FilterResult{
		Passed: true,
		Reason: "passed all user preference checks",
		Metadata: map[string]interface{}{
			"opportunity_type": opp.OpportunityType,
			"market_key":       opp.MarketKey,
			"edge_pct":         opp.EdgePct,
		},
	}
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}


