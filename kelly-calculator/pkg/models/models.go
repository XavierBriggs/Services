package models

// OpportunityRequest is the request for Kelly calculations
type OpportunityRequest struct {
	Opportunity  Opportunity `json:"opportunity"`
	Bankroll     float64     `json:"bankroll"`
	KellyFraction float64    `json:"kelly_fraction"`
}

// Opportunity represents a betting opportunity
type Opportunity struct {
	ID              int64            `json:"id"`
	OpportunityType string           `json:"opportunity_type"` // edge, middle, scalp
	EdgePercent     float64          `json:"edge_pct"`
	Legs            []OpportunityLeg `json:"legs"`
}

// OpportunityLeg represents a single leg of an opportunity
type OpportunityLeg struct {
	BookKey       string   `json:"book_key"`
	OutcomeName   string   `json:"outcome_name"`
	Price         int      `json:"price"`          // American odds
	Point         *float64 `json:"point"`          // Optional
	LegEdgePercent *float64 `json:"leg_edge_pct"`  // Optional
}

// KellyResponse is the unified response for all opportunity types
type KellyResponse struct {
	Type            string               `json:"type"`
	TotalStake      float64              `json:"total_stake"`
	GuaranteedProfit *float64            `json:"guaranteed_profit,omitempty"` // For scalps
	ProfitPercent   *float64             `json:"profit_pct,omitempty"`        // For scalps
	Legs            []LegRecommendation  `json:"legs"`
	BestCase        *string              `json:"best_case,omitempty"`    // For middles
	WorstCase       *string              `json:"worst_case,omitempty"`   // For middles
	Instructions    *string              `json:"instructions,omitempty"`
	Confidence      *string              `json:"confidence,omitempty"`   // For edges
	Warnings        []string             `json:"warnings"`
}

// LegRecommendation represents stake recommendation for a single leg
type LegRecommendation struct {
	Book             string   `json:"book"`
	Outcome          string   `json:"outcome"`
	Stake            float64  `json:"stake"`
	PotentialReturn  *float64 `json:"potential_return,omitempty"`  // For scalps
	FullKelly        *float64 `json:"full_kelly,omitempty"`        // For edges/middles
	FractionalKelly  *float64 `json:"fractional_kelly,omitempty"`  // For edges/middles
	EdgePercent      *float64 `json:"edge_pct,omitempty"`          // For edges/middles
	EVPerDollar      *float64 `json:"ev_per_dollar,omitempty"`     // For edges/middles
	Explanation      string   `json:"explanation"`
}


