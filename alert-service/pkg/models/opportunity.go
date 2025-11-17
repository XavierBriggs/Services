package models

import "time"

// Opportunity represents a detected betting opportunity (copy from edge-detector)
type Opportunity struct {
	ID              int64           `json:"id"`
	OpportunityType string          `json:"opportunity_type"`
	SportKey        string          `json:"sport_key"`
	EventID         string          `json:"event_id"`
	MarketKey       string          `json:"market_key"`
	EdgePercent     float64         `json:"edge_pct"`
	FairPrice       *int            `json:"fair_price,omitempty"`
	DetectedAt      time.Time       `json:"detected_at"`
	DataAgeSeconds  int             `json:"data_age_seconds"`
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




