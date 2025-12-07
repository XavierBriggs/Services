package filter

import (
	"fmt"
	"strings"

	"github.com/XavierBriggs/fortuna/services/slack-gateway/pkg/models"
)

// FilterResult contains the result of a filter check
type FilterResult struct {
	Allowed bool
	Reason  string
}

// ShouldAllow checks if an opportunity should be allowed based on user preferences
// This function is shared between alert filtering and bet-time enforcement
func ShouldAllow(pref *models.SlackFilterPreference, opp *models.Opportunity, bookKey string) FilterResult {
	// If no preference exists, allow everything
	if pref == nil {
		return FilterResult{Allowed: true}
	}

	// Check if user has disabled alerts/betting
	if !pref.Enabled {
		return FilterResult{
			Allowed: false,
			Reason:  "Alerts and betting are disabled in your preferences",
		}
	}

	// Check books whitelist
	if len(pref.BooksWhitelist) > 0 {
		bookAllowed := false
		normalizedBookKey := strings.ToLower(bookKey)
		for _, allowed := range pref.BooksWhitelist {
			if strings.ToLower(allowed) == normalizedBookKey {
				bookAllowed = true
				break
			}
		}
		if !bookAllowed {
			return FilterResult{
				Allowed: false,
				Reason:  fmt.Sprintf("Book '%s' is not in your allowed books list", bookKey),
			}
		}
	}

	// Check minimum edge
	if pref.MinEdgePercent != nil {
		if opp.EdgePercent < *pref.MinEdgePercent {
			return FilterResult{
				Allowed: false,
				Reason:  fmt.Sprintf("Edge %.2f%% is below your minimum of %.2f%%", opp.EdgePercent, *pref.MinEdgePercent),
			}
		}
	}

	return FilterResult{Allowed: true}
}

// ShouldAlertForOpportunity checks if an alert should be sent for an opportunity
// Returns true if the opportunity passes all filter checks
func ShouldAlertForOpportunity(pref *models.SlackFilterPreference, opp *models.Opportunity) FilterResult {
	// If no preference exists, allow everything
	if pref == nil {
		return FilterResult{Allowed: true}
	}

	// Check if user has disabled alerts
	if !pref.Enabled {
		return FilterResult{
			Allowed: false,
			Reason:  "Alerts are disabled",
		}
	}

	// Check minimum edge
	if pref.MinEdgePercent != nil {
		if opp.EdgePercent < *pref.MinEdgePercent {
			return FilterResult{
				Allowed: false,
				Reason:  fmt.Sprintf("edge_below_min: %.2f%% < %.2f%%", opp.EdgePercent, *pref.MinEdgePercent),
			}
		}
	}

	// Check if any leg's book is in the whitelist (if whitelist is set)
	if len(pref.BooksWhitelist) > 0 {
		hasAllowedBook := false
		for _, leg := range opp.Legs {
			normalizedBookKey := strings.ToLower(leg.BookKey)
			for _, allowed := range pref.BooksWhitelist {
				if strings.ToLower(allowed) == normalizedBookKey {
					hasAllowedBook = true
					break
				}
			}
			if hasAllowedBook {
				break
			}
		}
		if !hasAllowedBook {
			return FilterResult{
				Allowed: false,
				Reason:  "no_allowed_books: no legs match whitelist",
			}
		}
	}

	return FilterResult{Allowed: true}
}

// ValidateStake validates a stake amount
func ValidateStake(stakeCents int, minCents int, maxCents int) error {
	if stakeCents <= 0 {
		return fmt.Errorf("stake must be positive")
	}
	if stakeCents < minCents {
		return fmt.Errorf("stake $%.2f is below minimum $%.2f", float64(stakeCents)/100, float64(minCents)/100)
	}
	if maxCents > 0 && stakeCents > maxCents {
		return fmt.Errorf("stake $%.2f exceeds maximum $%.2f", float64(stakeCents)/100, float64(maxCents)/100)
	}
	return nil
}

// ParseStakeDollars parses a stake string in dollars and returns cents
func ParseStakeDollars(stakeStr string) (int, error) {
	var stakeDollars float64
	_, err := fmt.Sscanf(strings.TrimSpace(stakeStr), "%f", &stakeDollars)
	if err != nil {
		return 0, fmt.Errorf("invalid stake format: %s", stakeStr)
	}
	if stakeDollars <= 0 {
		return 0, fmt.Errorf("stake must be positive")
	}
	return int(stakeDollars * 100), nil
}






