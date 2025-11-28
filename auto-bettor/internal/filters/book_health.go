package filters

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// BookHealthFilter checks if bot is logged in and healthy
type BookHealthFilter struct {
	botServiceURL string
	httpClient    *http.Client
}

// NewBookHealthFilter creates a new book health filter
func NewBookHealthFilter(botServiceURL string) *BookHealthFilter {
	return &BookHealthFilter{
		botServiceURL: botServiceURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Increased from 5s to handle slow Talos bot responses
		},
	}
}

// Name returns the filter name
func (f *BookHealthFilter) Name() string {
	return "book_health"
}

// BotStatus represents the bot service status response
type BotStatus struct {
	Bots []struct {
		Name        string  `json:"name"`
		DisplayName string  `json:"display_name"`
		Status      string  `json:"status"`
		LoggedIn    bool    `json:"logged_in"`
		Balance     *string `json:"balance"`
		Error       *string `json:"error"`
	} `json:"bots"`
}

// Evaluate checks if bots for all legs are healthy
func (f *BookHealthFilter) Evaluate(
	opp models.Opportunity,
	settings models.UserSettings,
	state models.AutoBettingState,
) models.FilterResult {
	// Get bot status
	resp, err := f.httpClient.Get(f.botServiceURL + "/api/v1/bots/status")
	if err != nil {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("bot service unreachable: %v", err),
			Metadata: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("bot service returned status %d", resp.StatusCode),
			Metadata: map[string]interface{}{
				"status_code": resp.StatusCode,
			},
		}
	}

	var botStatus BotStatus
	if err := json.NewDecoder(resp.Body).Decode(&botStatus); err != nil {
		return models.FilterResult{
			Passed: false,
			Reason: fmt.Sprintf("failed to parse bot status: %v", err),
			Metadata: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	// Create map of bot statuses
	botMap := make(map[string]bool)
	for _, bot := range botStatus.Bots {
		botMap[bot.Name] = bot.LoggedIn && bot.Status == "ok"
	}

	// Check if all required bots are healthy
	for _, leg := range opp.Legs {
		if healthy, exists := botMap[leg.BookKey]; !exists || !healthy {
			return models.FilterResult{
				Passed: false,
				Reason: fmt.Sprintf("bot for '%s' not available or not logged in", leg.BookKey),
				Metadata: map[string]interface{}{
					"book_key": leg.BookKey,
					"healthy":  healthy,
					"exists":   exists,
				},
			}
		}
	}

	return models.FilterResult{
		Passed: true,
		Reason: "all required bots are healthy and logged in",
		Metadata: map[string]interface{}{
			"books_checked": len(opp.Legs),
		},
	}
}

