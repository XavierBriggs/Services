package filters

import "github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"

// Filter interface for evaluating opportunities
type Filter interface {
	// Name returns the filter name for logging
	Name() string

	// Evaluate checks if the opportunity passes this filter
	Evaluate(
		opp models.Opportunity,
		settings models.UserSettings,
		state models.AutoBettingState,
	) models.FilterResult
}

// FilterChain runs multiple filters in sequence
type FilterChain struct {
	filters []Filter
}

// NewFilterChain creates a new filter chain
func NewFilterChain(filters ...Filter) *FilterChain {
	return &FilterChain{
		filters: filters,
	}
}

// Evaluate runs all filters and returns the first failure or success if all pass
func (fc *FilterChain) Evaluate(
	opp models.Opportunity,
	settings models.UserSettings,
	state models.AutoBettingState,
) (bool, map[string]models.FilterResult) {
	results := make(map[string]models.FilterResult)

	for _, filter := range fc.filters {
		result := filter.Evaluate(opp, settings, state)
		results[filter.Name()] = result

		if !result.Passed {
			return false, results
		}
	}

	return true, results
}


