package models

import "time"

// BookStats represents aggregated statistics for a book within a time bucket
type BookStats struct {
	TimestampBucket  time.Time `json:"timestamp_bucket"`
	BookKey          string    `json:"book_key"`
	OpportunityType  string    `json:"opportunity_type"`
	GameStatus       string    `json:"game_status"` // "pregame" or "live"
	SportKey         string    `json:"sport_key"`   // Sport identifier (e.g., "basketball_nba")
	MarketKey        string    `json:"market_key"`  // Market type (e.g., "h2h", "spreads", "totals")
	OpportunityCount int       `json:"opportunity_count"`
	AvgEdgePct       float64   `json:"avg_edge_pct"`

	// Edge Distribution Metrics (Phase 3)
	MinEdgePct    float64 `json:"min_edge_pct"`
	MaxEdgePct    float64 `json:"max_edge_pct"`
	MedianEdgePct float64 `json:"median_edge_pct"`
	EdgeStddev    float64 `json:"edge_stddev"`

	// Data Quality Metrics
	AvgDataAgeSeconds int `json:"avg_data_age_seconds"`
	StaleDataCount    int `json:"stale_data_count"` // Count of opportunities with stale data (>30s)

	// Velocity Metrics
	OppsPerMinute float64 `json:"opps_per_minute"`

	// Hold Time & Execution Metrics (Phase 2)
	AvgHoldTimeSeconds  int     `json:"avg_hold_time_seconds"`
	MissedOpportunities int     `json:"missed_opportunities"`
	ExecutionRate       float64 `json:"execution_rate"` // % of opps converted to bets

	// Value Metrics
	TotalExpectedValue float64 `json:"total_expected_value"`
	AvgKellyFraction   float64 `json:"avg_kelly_fraction"`

	// Edge Threshold Counts
	Edge5PctCount  int `json:"edge_5pct_count"`  // Edges >= 5%
	Edge10PctCount int `json:"edge_10pct_count"` // Edges >= 10%
	Edge20PctCount int `json:"edge_20pct_count"` // Edges >= 20%

	// Bet Metrics (populated by enricher)
	TotalBets int     `json:"total_bets"`
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
	AvgCLV    float64 `json:"avg_clv"`
	NetProfit float64 `json:"net_profit"`
	ROI       float64 `json:"roi"`

	// Opportunity CLV Metrics (populated by enricher - validates edge detector)
	AvgOpportunityCLV      float64 `json:"avg_opportunity_clv"`
	OpportunityCLVCount    int     `json:"opportunity_clv_count"`
	OpportunityCLVAccuracy float64 `json:"opportunity_clv_accuracy"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Opportunity represents a detected betting opportunity (matches edge-detector model)
type Opportunity struct {
	ID              int64            `json:"id,omitempty"`
	OpportunityType string           `json:"opportunity_type"`
	SportKey        string           `json:"sport_key"`
	EventID         string           `json:"event_id"`
	MarketKey       string           `json:"market_key"`
	EdgePercent     float64          `json:"edge_pct"`
	FairPrice       *int             `json:"fair_price,omitempty"`
	DetectedAt      time.Time        `json:"detected_at"`
	DataAgeSeconds  int              `json:"data_age_seconds"`
	EventStatus     string           `json:"event_status"` // "upcoming" or "live"
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

// StatsRequest represents API request parameters
type StatsRequest struct {
	BookKey         string    `json:"book_key,omitempty"`
	OpportunityType string    `json:"opportunity_type,omitempty"`
	SportKey        string    `json:"sport_key,omitempty"`
	MarketKey       string    `json:"market_key,omitempty"`
	GameStatus      string    `json:"game_status,omitempty"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	Interval        string    `json:"interval,omitempty"` // "5m", "1h", "1d"
}

// StatsSummary represents aggregated summary response
type StatsSummary struct {
	TotalOpportunities int     `json:"total_opportunities"`
	TotalBets          int     `json:"total_bets"`
	NetProfit          float64 `json:"net_profit"`
	ROI                float64 `json:"roi"`
	WinRate            float64 `json:"win_rate"`
	AvgCLV             float64 `json:"avg_clv"`
	AvgEdgePct         float64 `json:"avg_edge_pct"`

	// Enhanced metrics
	AvgHoldTimeSeconds int     `json:"avg_hold_time_seconds"`
	ExecutionRate      float64 `json:"execution_rate"`
	OppsPerMinute      float64 `json:"opps_per_minute"`

	// Edge distribution
	MinEdgePct    float64 `json:"min_edge_pct"`
	MaxEdgePct    float64 `json:"max_edge_pct"`
	MedianEdgePct float64 `json:"median_edge_pct"`

	ByBook   map[string]Stats `json:"by_book"`
	ByType   map[string]Stats `json:"by_type"`
	BySport  map[string]Stats `json:"by_sport,omitempty"`
	ByMarket map[string]Stats `json:"by_market,omitempty"`
}

// Stats represents aggregated statistics
type Stats struct {
	OpportunityCount     int     `json:"opportunity_count"`
	LiveOpportunities    int     `json:"live_opportunities"`    // Opportunities detected during live games
	PregameOpportunities int     `json:"pregame_opportunities"` // Opportunities detected before game start
	AvgEdgePct           float64 `json:"avg_edge_pct"`
	TotalBets            int     `json:"total_bets"`
	Wins                 int     `json:"wins"`
	Losses               int     `json:"losses"`
	NetProfit            float64 `json:"net_profit"`
	ROI                  float64 `json:"roi"`
	AvgCLV               float64 `json:"avg_clv"`

	// Enhanced metrics
	AvgHoldTimeSeconds int     `json:"avg_hold_time_seconds,omitempty"`
	ExecutionRate      float64 `json:"execution_rate,omitempty"`
	MinEdgePct         float64 `json:"min_edge_pct,omitempty"`
	MaxEdgePct         float64 `json:"max_edge_pct,omitempty"`
	MedianEdgePct      float64 `json:"median_edge_pct,omitempty"`
}

// TimeSeriesPoint represents a single data point in time series
type TimeSeriesPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	BookKey          string    `json:"book_key"`
	OpportunityType  string    `json:"opportunity_type"`
	GameStatus       string    `json:"game_status,omitempty"`
	OpportunityCount int       `json:"opportunity_count"`
	AvgEdgePct       float64   `json:"avg_edge_pct"`
	TotalBets        int       `json:"total_bets"`
	NetProfit        float64   `json:"net_profit"`
	ROI              float64   `json:"roi"`

	// Enhanced metrics
	AvgHoldTimeSeconds int     `json:"avg_hold_time_seconds,omitempty"`
	ExecutionRate      float64 `json:"execution_rate,omitempty"`
	MinEdgePct         float64 `json:"min_edge_pct,omitempty"`
	MaxEdgePct         float64 `json:"max_edge_pct,omitempty"`
}

// EdgeDistribution represents edge distribution for charting
type EdgeDistribution struct {
	Buckets []EdgeBucket `json:"buckets"`
	Stats   EdgeStats    `json:"stats"`
}

// EdgeBucket represents a histogram bucket for edge distribution
type EdgeBucket struct {
	RangeStart float64 `json:"range_start"`
	RangeEnd   float64 `json:"range_end"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// EdgeStats represents summary statistics for edge distribution
type EdgeStats struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	Stddev float64 `json:"stddev"`
	P25    float64 `json:"p25"` // 25th percentile
	P75    float64 `json:"p75"` // 75th percentile
	P90    float64 `json:"p90"` // 90th percentile
	P95    float64 `json:"p95"` // 95th percentile
	Total  int     `json:"total"`
}

// ExecutionStats represents execution/hold time statistics
type ExecutionStats struct {
	AvgHoldTimeSeconds int                `json:"avg_hold_time_seconds"`
	MinHoldTimeSeconds int                `json:"min_hold_time_seconds"`
	MaxHoldTimeSeconds int                `json:"max_hold_time_seconds"`
	TotalOpportunities int                `json:"total_opportunities"`
	TotalBetsPlaced    int                `json:"total_bets_placed"`
	ExecutionRate      float64            `json:"execution_rate"`
	ConversionByBook   map[string]float64 `json:"conversion_by_book,omitempty"`
}

// BookComparison represents comparative analysis of sportsbooks
type BookComparison struct {
	BookKey                  string  `json:"book_key"`
	DisplayName              string  `json:"display_name,omitempty"`
	BookType                 string  `json:"book_type"` // "sharp" or "soft"
	OpportunityCount         int     `json:"opportunity_count"`
	TimesInScalp             int     `json:"times_in_scalp"`
	TimesInMiddle            int     `json:"times_in_middle"`
	TimesInEdge              int     `json:"times_in_edge"`
	TimesBestPrice           int     `json:"times_best_price"`
	AvgEdgePct               float64 `json:"avg_edge_pct"`
	ExecutionRate            float64 `json:"execution_rate"`
	StaleOddsRate            float64 `json:"stale_odds_rate"`
	OddsCompetitivenessScore float64 `json:"odds_competitiveness_score"`
	ReliabilityScore         float64 `json:"reliability_score"`
}

// HourlyPattern represents opportunity patterns by time of day
type HourlyPattern struct {
	HourOfDay               int     `json:"hour_of_day"`
	DayOfWeek               int     `json:"day_of_week"`
	AvgOpportunitiesPerHour float64 `json:"avg_opportunities_per_hour"`
	AvgEdgePct              float64 `json:"avg_edge_pct"`
	AvgExecutionRate        float64 `json:"avg_execution_rate"`
	BestROI                 float64 `json:"best_roi"`
	BestWinRate             float64 `json:"best_win_rate"`
	SampleCount             int     `json:"sample_count"`
}

// OptimalWindow represents optimal betting windows
type OptimalWindow struct {
	BestHours     []int    `json:"best_hours"`
	BestDays      []string `json:"best_days"`
	AvgEdge       float64  `json:"avg_edge"`
	AvgConversion float64  `json:"avg_conversion"`
	Confidence    string   `json:"confidence"` // "high", "medium", "low"
}

// BookPairStats represents aggregated statistics for book pairs (scalps/middles)
// This tracks opportunities that involve TWO books working together
type BookPairStats struct {
	TimestampBucket    time.Time `json:"timestamp_bucket"`
	BookKey1           string    `json:"book_key_1"`          // First book (alphabetically sorted)
	BookKey2           string    `json:"book_key_2"`          // Second book (alphabetically sorted)
	PairName           string    `json:"pair_name,omitempty"` // "pinnacle + draftkings"
	OpportunityType    string    `json:"opportunity_type"`    // "scalp" or "middle"
	GameStatus         string    `json:"game_status"`
	SportKey           string    `json:"sport_key,omitempty"`
	MarketKey          string    `json:"market_key,omitempty"`
	OpportunityCount   int       `json:"opportunity_count"`
	AvgEdgePct         float64   `json:"avg_edge_pct"`
	MinEdgePct         float64   `json:"min_edge_pct"`
	MaxEdgePct         float64   `json:"max_edge_pct"`
	TotalBets          int       `json:"total_bets"`   // Bets placed on BOTH legs
	PartialBets        int       `json:"partial_bets"` // Bets where only one leg succeeded
	Wins               int       `json:"wins"`
	Losses             int       `json:"losses"`
	NetProfit          float64   `json:"net_profit"`
	AvgHoldTimeSeconds int       `json:"avg_hold_time_seconds"`
	CreatedAt          time.Time `json:"created_at,omitempty"`
	UpdatedAt          time.Time `json:"updated_at,omitempty"`
}

// BookPairSummary represents aggregated summary for book pairs
type BookPairSummary struct {
	PairName             string  `json:"pair_name"` // "pinnacle + draftkings"
	BookKey1             string  `json:"book_key_1"`
	BookKey2             string  `json:"book_key_2"`
	OpportunityType      string  `json:"opportunity_type"`
	TotalOpportunities   int     `json:"total_opportunities"`
	LiveOpportunities    int     `json:"live_opportunities"`    // Opportunities detected during live games
	PregameOpportunities int     `json:"pregame_opportunities"` // Opportunities detected before game start
	AvgEdgePct           float64 `json:"avg_edge_pct"`
	BestEdgePct          float64 `json:"best_edge_pct"`
	TotalBets            int     `json:"total_bets"`
	TotalProfit          float64 `json:"total_profit"`
	ROI                  float64 `json:"roi"`
	AvgHoldTimeSeconds   int     `json:"avg_hold_time_seconds"`
	MinHoldTimeSeconds   int     `json:"min_hold_time_seconds"` // Shortest opportunity duration
	MaxHoldTimeSeconds   int     `json:"max_hold_time_seconds"` // Longest opportunity duration
	ExecutionRate        float64 `json:"execution_rate"`        // % of opportunities converted to bets
}

// PairPerformance represents historical performance metrics for a book pair
// Used to decide where to push volume and where to back off
type PairPerformance struct {
	BookKey1           string    `json:"book_key_1"`
	BookKey2           string    `json:"book_key_2"`
	PairName           string    `json:"pair_name"` // "betmgm × espnbet"
	OpportunityType    string    `json:"opportunity_type"`
	TotalOpportunities int       `json:"total_opportunities"`
	TotalBetsPlaced    int       `json:"total_bets_placed"` // Full pair bets (both legs)
	TotalHandle        float64   `json:"total_handle"`      // Sum of all stakes
	RealizedProfit     float64   `json:"realized_profit"`   // Net P/L from settled bets
	ROIPct             float64   `json:"roi_pct"`           // (profit / handle) * 100
	WinCount           int       `json:"win_count"`
	LossCount          int       `json:"loss_count"`
	PushCount          int       `json:"push_count"`
	PendingCount       int       `json:"pending_count"`
	ExecutionRate      float64   `json:"execution_rate"` // % of opps bet on
	UpdatedAt          time.Time `json:"updated_at"`
}

// =========================================================================
// OPPORTUNITY CLV - Edge Detector Validation Models
// =========================================================================

// OpportunityCLVSummary represents aggregated opportunity CLV statistics
// This validates how accurate the edge detector is at finding real edges
type OpportunityCLVSummary struct {
	TotalOpportunities int     `json:"total_opportunities"`
	AvgCLV             float64 `json:"avg_clv"`               // Average CLV in cents
	MinCLV             float64 `json:"min_clv"`               // Minimum CLV
	MaxCLV             float64 `json:"max_clv"`               // Maximum CLV
	AvgEdgeAtDetection float64 `json:"avg_edge_at_detection"` // Average edge when detected
	AvgEdgeAtClose     float64 `json:"avg_edge_at_close"`     // Average edge at closing line
	AvgEdgeDecay       float64 `json:"avg_edge_decay"`        // How much edge decays on average
	PositiveCLVCount   int     `json:"positive_clv_count"`    // Opportunities with CLV > 0
	NegativeCLVCount   int     `json:"negative_clv_count"`    // Opportunities with CLV <= 0
	EdgeAccuracy       float64 `json:"edge_accuracy"`         // % of opportunities with positive CLV
	FalsePositiveRate  float64 `json:"false_positive_rate"`   // % of opportunities where edge evaporated

	// Breakdowns
	ByBook map[string]OpportunityCLVStats `json:"by_book,omitempty"`
	ByType map[string]OpportunityCLVStats `json:"by_type,omitempty"`
}

// OpportunityCLVStats represents CLV statistics for a dimension (book, type, etc.)
type OpportunityCLVStats struct {
	TotalOpportunities int     `json:"total_opportunities"`
	AvgCLV             float64 `json:"avg_clv"`
	MinCLV             float64 `json:"min_clv"`
	MaxCLV             float64 `json:"max_clv"`
	AvgEdgeAtDetection float64 `json:"avg_edge_at_detection"`
	PositiveCLVCount   int     `json:"positive_clv_count"`
	NegativeCLVCount   int     `json:"negative_clv_count"`
	EdgeAccuracy       float64 `json:"edge_accuracy"`
}

// EdgeAccuracyPoint represents a time series point for edge accuracy
type EdgeAccuracyPoint struct {
	Timestamp          time.Time `json:"timestamp"`
	TotalOpportunities int       `json:"total_opportunities"`
	AvgCLV             float64   `json:"avg_clv"`
	AvgEdgeAtDetection float64   `json:"avg_edge_at_detection"`
	EdgeAccuracy       float64   `json:"edge_accuracy"`
}

// OpportunityCLVResponse is the API response for opportunity CLV endpoint
type OpportunityCLVResponse struct {
	Summary    *OpportunityCLVSummary `json:"summary"`
	TimeSeries []EdgeAccuracyPoint    `json:"time_series,omitempty"`
	Message    string                 `json:"message,omitempty"`
}
