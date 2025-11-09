package models

import "time"

// Bet represents a bet entry
type Bet struct {
	ID            int64      `json:"id"`
	OpportunityID *int64     `json:"opportunity_id"`
	SportKey      string     `json:"sport_key"`
	EventID       string     `json:"event_id"`
	MarketKey     string     `json:"market_key"`
	BookKey       string     `json:"book_key"`
	OutcomeName   string     `json:"outcome_name"`
	BetType       string     `json:"bet_type"`
	StakeAmount   float64    `json:"stake_amount"`
	BetPrice      int        `json:"bet_price"`
	Point         *float64   `json:"point"`
	PlacedAt      time.Time  `json:"placed_at"`
	SettledAt     *time.Time `json:"settled_at"`
	Result        string     `json:"result"`
	PayoutAmount  *float64   `json:"payout_amount"`
}

// BetWithPerformance includes bet and CLV performance metrics
type BetWithPerformance struct {
	Bet
	ClosingLinePrice       *int       `json:"closing_line_price"`
	CLVCents               *float64   `json:"clv_cents"`
	HoldTimeSeconds        *int       `json:"hold_time_seconds"`
	PerformanceRecordedAt  *time.Time `json:"performance_recorded_at"`
}

// BetFilters defines filters for bet queries
type BetFilters struct {
	SportKey string
	BookKey  string
	Result   string
	Since    *time.Time
	Until    *time.Time
	Limit    int
	Offset   int
}

// BetSummary provides aggregate P&L statistics
type BetSummary struct {
	TotalBets     int                       `json:"total_bets"`
	TotalWagered  float64                   `json:"total_wagered"`
	TotalReturned float64                   `json:"total_returned"`
	NetProfit     float64                   `json:"net_profit"`
	ROIPct        float64                   `json:"roi_pct"`
	AvgCLVCents   float64                   `json:"avg_clv_cents"`
	WinRatePct    float64                   `json:"win_rate_pct"`
	BySport       map[string]SportSummary   `json:"by_sport"`
	ByBook        map[string]BookSummary    `json:"by_book"`
}

// SportSummary provides per-sport statistics
type SportSummary struct {
	Count     int     `json:"count"`
	Wagered   float64 `json:"wagered"`
	Returned  float64 `json:"returned"`
	NetProfit float64 `json:"net_profit"`
	ROIPct    float64 `json:"roi_pct"`
}

// BookSummary provides per-book statistics
type BookSummary struct {
	Count     int     `json:"count"`
	Wagered   float64 `json:"wagered"`
	Returned  float64 `json:"returned"`
	NetProfit float64 `json:"net_profit"`
	ROIPct    float64 `json:"roi_pct"`
	WinRate   float64 `json:"win_rate_pct"`
}

