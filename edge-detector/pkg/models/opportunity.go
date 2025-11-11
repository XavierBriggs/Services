package models

import "time"

// OpportunityType defines the type of betting opportunity
type OpportunityType string

const (
	OpportunityTypeEdge   OpportunityType = "edge"   // Single +EV bet
	OpportunityTypeMiddle OpportunityType = "middle" // Both sides of market are +EV
	OpportunityTypeScalp  OpportunityType = "scalp"  // Guaranteed profit (arbitrage)
)

// Opportunity represents a detected betting opportunity
type Opportunity struct {
	// Core fields
	OpportunityType OpportunityType `json:"opportunity_type"`
	SportKey        string          `json:"sport_key"`
	EventID         string          `json:"event_id"`
	MarketKey       string          `json:"market_key"`
	EdgePercent     float64         `json:"edge_pct"`
	FairPrice       *int            `json:"fair_price,omitempty"` // American odds

	// Metadata
	DetectedAt     time.Time `json:"detected_at"`
	DataAgeSeconds int       `json:"data_age_seconds"`

	// Legs
	Legs []OpportunityLeg `json:"legs"`

	// Database ID (populated after write)
	ID int64 `json:"id,omitempty"`
}

// OpportunityLeg represents a single betting leg within an opportunity
type OpportunityLeg struct {
	BookKey      string   `json:"book_key"`
	OutcomeName  string   `json:"outcome_name"`
	Price        int      `json:"price"`             // American odds
	Point        *float64 `json:"point,omitempty"`   // For spreads/totals
	LegEdgePercent *float64 `json:"leg_edge_pct,omitempty"` // Edge for this specific leg
}

// NormalizedOdds matches the normalizer's output
// This is a copy to avoid circular dependencies
type NormalizedOdds struct {
	// Raw odds data
	EventID          string    `json:"event_id"`
	SportKey         string    `json:"sport_key"`
	MarketKey        string    `json:"market_key"`
	BookKey          string    `json:"book_key"`
	OutcomeName      string    `json:"outcome_name"`
	Price            int       `json:"price"`              // American odds
	Point            *float64  `json:"point,omitempty"`    // For spreads/totals
	VendorLastUpdate time.Time `json:"vendor_last_update"`
	ReceivedAt       time.Time `json:"received_at"`

	// Normalized values
	DecimalOdds        float64  `json:"decimal_odds"`
	ImpliedProbability float64  `json:"implied_probability"`
	NoVigProbability   *float64 `json:"novig_probability"`
	FairPrice          *int     `json:"fair_price"`
	Edge               *float64 `json:"edge"`

	// Sharp consensus
	SharpConsensus *float64 `json:"sharp_consensus"`

	// Market classification
	MarketType string `json:"market_type"`
	VigMethod  string `json:"vig_method"`

	// Metadata
	NormalizedAt      time.Time `json:"normalized_at"`
	ProcessingLatency int64     `json:"processing_latency_ms"`
}

// BookType classifies sportsbooks
type BookType string

const (
	BookTypeSharp BookType = "sharp" // Pinnacle, Circa, Bookmaker
	BookTypeSoft  BookType = "soft"  // FanDuel, DraftKings, etc.
)

// Book represents a sportsbook with its classification
type Book struct {
	BookKey     string   `json:"book_key"`
	DisplayName string   `json:"display_name"`
	BookType    BookType `json:"book_type"`
	Active      bool     `json:"active"`
}


