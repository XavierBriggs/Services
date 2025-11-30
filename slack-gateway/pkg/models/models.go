package models

import (
	"time"
)

// SlackFilterPreference represents user-specific Slack alert and bet filtering preferences
type SlackFilterPreference struct {
	SlackUserID       string    `json:"slack_user_id"`
	BooksWhitelist    []string  `json:"books_whitelist"`
	MinEdgePercent    *float64  `json:"min_edge_percent,omitempty"`
	Enabled           bool      `json:"enabled"`
	DefaultStakeCents int       `json:"default_stake_cents"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// BetIntent represents a bet placement intent from Slack
type BetIntent struct {
	BetIntentID string `json:"bet_intent_id"`
	OppID       int64  `json:"opp_id"`
	Book        string `json:"book"`
	StakeCents  int    `json:"stake_cents"`
	ChannelID   string `json:"channel_id"`
	MessageTS   string `json:"message_ts"`
	LegIdx      int    `json:"leg_idx"`
}

// ButtonValue is the JSON structure stored in the Place Bet button value
type ButtonValue struct {
	OppID        int64  `json:"opp_id"`
	Book         string `json:"book"`
	DefaultStake int    `json:"default_stake"`
	LegIdx       int    `json:"leg_idx"`
	MessageTS    string `json:"message_ts"`
}

// ModalMetadata is the JSON structure stored in modal private_metadata
type ModalMetadata struct {
	OppID       int64  `json:"opp_id"`
	Book        string `json:"book"`
	ChannelID   string `json:"channel_id"`
	BetIntentID string `json:"bet_intent_id"`
	LegIdx      int    `json:"leg_idx"`
	MessageTS   string `json:"message_ts"`
}

// Opportunity represents a detected betting opportunity (mirrored from alert-service)
type Opportunity struct {
	ID              int64            `json:"id"`
	OpportunityType string           `json:"opportunity_type"`
	SportKey        string           `json:"sport_key"`
	EventID         string           `json:"event_id"`
	MarketKey       string           `json:"market_key"`
	EdgePercent     float64          `json:"edge_pct"`
	FairPrice       *int             `json:"fair_price,omitempty"`
	DetectedAt      time.Time        `json:"detected_at"`
	DataAgeSeconds  int              `json:"data_age_seconds"`
	Legs            []OpportunityLeg `json:"legs"`
}

// OpportunityLeg represents a single betting leg
type OpportunityLeg struct {
	BookKey        string   `json:"book_key"`
	OutcomeName    string   `json:"outcome_name"`
	Price          int      `json:"price"`
	Point          *float64 `json:"point,omitempty"`
	LegEdgePercent *float64 `json:"leg_edge_pct,omitempty"`
}

// PlaceBetRequest is the request to send to the bet API
type PlaceBetRequest struct {
	OpportunityID int64        `json:"opportunity_id"`
	Legs          []LegRequest `json:"legs"`
}

// LegRequest contains bet details for a single leg
type LegRequest struct {
	BookKey      string  `json:"book_key"`
	OutcomeName  string  `json:"outcome_name"`
	Stake        float64 `json:"stake"`
	ExpectedOdds int     `json:"expected_odds"`
}

// PlaceBetResponse is the response from the bet API
type PlaceBetResponse struct {
	Success bool        `json:"success"`
	Results []LegResult `json:"results"`
}

// LegResult contains the result for a single leg
type LegResult struct {
	BookKey      string `json:"book_key"`
	Success      bool   `json:"success"`
	BetID        *int64 `json:"bet_id,omitempty"`
	TicketNumber string `json:"ticket_number,omitempty"`
	LatencyMs    int64  `json:"latency_ms"`
	Error        string `json:"error,omitempty"`
}
