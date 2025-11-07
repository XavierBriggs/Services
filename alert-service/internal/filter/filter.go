package filter

import (
	"fmt"

	"github.com/XavierBriggs/fortuna/services/alert-service/pkg/models"
)

// Filter filters opportunities based on thresholds
type Filter struct {
	minEdgePercent    float64
	maxDataAgeSeconds int
}

// NewFilter creates a new filter
func NewFilter(minEdgePercent float64, maxDataAgeSeconds int) *Filter {
	return &Filter{
		minEdgePercent:    minEdgePercent,
		maxDataAgeSeconds: maxDataAgeSeconds,
	}
}

// ShouldAlert returns true if the opportunity meets alert thresholds
func (f *Filter) ShouldAlert(opp models.Opportunity) (bool, string) {
	// Check edge threshold
	if opp.EdgePercent < f.minEdgePercent {
		return false, fmt.Sprintf("edge %.2f%% below threshold %.2f%%", opp.EdgePercent, f.minEdgePercent)
	}

	// Check data age
	if opp.DataAgeSeconds > f.maxDataAgeSeconds {
		return false, fmt.Sprintf("data age %ds exceeds threshold %ds", opp.DataAgeSeconds, f.maxDataAgeSeconds)
	}

	return true, ""
}

// FilterOpportunities filters a list of opportunities
func (f *Filter) FilterOpportunities(opportunities []models.Opportunity) []models.Opportunity {
	var filtered []models.Opportunity

	for _, opp := range opportunities {
		if should, _ := f.ShouldAlert(opp); should {
			filtered = append(filtered, opp)
		}
	}

	return filtered
}

