package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// BotServiceClient wraps the Bot Service API
type BotServiceClient struct {
	botServiceURL string
	httpClient    *http.Client
}

// NewBotServiceClient creates a new bot service client
func NewBotServiceClient(botServiceURL string) *BotServiceClient {
	return &BotServiceClient{
		botServiceURL: botServiceURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for bet placement
		},
	}
}

// PlaceBetRequest represents a bet placement request to bot service
type PlaceBetRequest struct {
	OpportunityID int64             `json:"opportunity_id"`
	Legs          []BotServiceLeg   `json:"legs"`
	EventInfo     *EventInfo        `json:"event_info,omitempty"`
	Opportunity   *OpportunityInfo  `json:"opportunity,omitempty"`
}

// BotServiceLeg represents a single bet leg
type BotServiceLeg struct {
	BookKey      string  `json:"book_key"`
	OutcomeName  string  `json:"outcome_name"`
	Stake        float64 `json:"stake"`
	ExpectedOdds int     `json:"expected_odds"`
}

// EventInfo contains event details for the bot
type EventInfo struct {
	EventID       string `json:"event_id"`
	SportKey      string `json:"sport_key"`
	HomeTeam      string `json:"home_team"`
	AwayTeam      string `json:"away_team"`
	HomeTeamShort string `json:"home_team_short"`
	AwayTeamShort string `json:"away_team_short"`
}

// OpportunityInfo contains opportunity details
type OpportunityInfo struct {
	OpportunityType string  `json:"opportunity_type"`
	MarketKey       string  `json:"market_key"`
	EdgePercent     float64 `json:"edge_pct"`
}

// PlaceBetResponse represents the bot service response
type PlaceBetResponse struct {
	Success bool        `json:"success"`
	Results []LegResult `json:"results"`
}

// LegResult contains the result for a single leg
type LegResult struct {
	BookKey      string  `json:"book_key"`
	Success      bool    `json:"success"`
	BetID        *int64  `json:"bet_id"`
	TicketNumber string  `json:"ticket_number"`
	LatencyMs    int64   `json:"latency_ms"`
	Error        string  `json:"error"`
}

// PlaceBet places a bet through the bot service
func (bsc *BotServiceClient) PlaceBet(req PlaceBetRequest) (*PlaceBetResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := bsc.httpClient.Post(
		bsc.botServiceURL+"/api/v1/place-bet",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call bot service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bot service returned status %d", resp.StatusCode)
	}

	var botResp PlaceBetResponse
	if err := json.NewDecoder(resp.Body).Decode(&botResp); err != nil {
		return nil, fmt.Errorf("failed to parse bot response: %w", err)
	}

	return &botResp, nil
}

