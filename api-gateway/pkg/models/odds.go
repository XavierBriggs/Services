package models

import "time"

// Event represents a sporting event
type Event struct {
	EventID      string    `json:"event_id"`
	SportKey     string    `json:"sport_key"`
	HomeTeam     string    `json:"home_team"`
	AwayTeam     string    `json:"away_team"`
	CommenceTime time.Time `json:"commence_time"`
	EventStatus  string    `json:"event_status"` // upcoming, live, completed
	DiscoveredAt time.Time `json:"discovered_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

// CurrentOdds represents the latest odds for a market
type CurrentOdds struct {
	EventID          string     `json:"event_id"`
	SportKey         string     `json:"sport_key"`
	MarketKey        string     `json:"market_key"`
	BookKey          string     `json:"book_key"`
	OutcomeName      string     `json:"outcome_name"`
	Price            int        `json:"price"`              // American odds
	Point            *float64   `json:"point,omitempty"`    // For spreads/totals
	VendorLastUpdate time.Time  `json:"vendor_last_update"`
	ReceivedAt       time.Time  `json:"received_at"`
	DataAge          float64    `json:"data_age_seconds"`   // Calculated field
}

// OddsHistory represents historical odds data
type OddsHistory struct {
	ID               int64      `json:"id"`
	EventID          string     `json:"event_id"`
	SportKey         string     `json:"sport_key"`
	MarketKey        string     `json:"market_key"`
	BookKey          string     `json:"book_key"`
	OutcomeName      string     `json:"outcome_name"`
	Price            int        `json:"price"`
	Point            *float64   `json:"point,omitempty"`
	VendorLastUpdate time.Time  `json:"vendor_last_update"`
	ReceivedAt       time.Time  `json:"received_at"`
	IsLatest         bool       `json:"is_latest"`
}

// EventWithOdds combines event details with current odds
type EventWithOdds struct {
	Event       Event         `json:"event"`
	CurrentOdds []CurrentOdds `json:"current_odds"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}


