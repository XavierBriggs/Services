package models

import (
	"time"
)

// UserSettings contains all auto-betting configuration for a user
type UserSettings struct {
	UserID string

	// Master toggle
	AutoBettingEnabled bool

	// Edge & opportunity filters
	AutoMinEdgePct              float64
	AutoEnabledOpportunityTypes []string
	AutoEnabledMarkets          []string
	AutoEnabledBooks            []string
	AutoDisabledBooks           []string

	// Risk management
	AutoMaxStakePerBet      float64
	AutoMaxExposurePerEvent float64
	AutoMaxExposureTotal    float64
	AutoMaxBetsPerHour      int
	AutoMaxBetsPerDay       int

	// Kelly sizing
	KellyFraction   float64 // From existing user_settings table
	AutoMaxKellyPct float64
	AutoMinStake    float64

	// Timing controls
	AutoMaxDataAgeSeconds   int
	AutoMinTimeToStartHours int
	AutoMaxTimeToStartHours int

	// Safety features
	AutoPauseOnLossStreak   int
	AutoPauseOnDailyLoss    float64
	AutoCorrelationDiscount float64

	// Bet type specific - MIDDLE
	AutoMiddleEnabled               bool
	AutoMiddleExecutionStrategy     string
	AutoMiddleRequiredLegs          int
	AutoMiddleMaxTimeBetweenLegsSec int

	// Bet type specific - SCALP
	AutoScalpEnabled             bool
	AutoScalpBankrollPct         float64
	AutoScalpMinProfitPct        float64
	AutoScalpExecutionTimeoutSec int

	// Bet type specific - EDGE
	AutoEdgeAllowLiveGames bool
}

// AutoBettingState tracks real-time state per user
type AutoBettingState struct {
	UserID string

	// Current exposure tracking
	TotalExposure   float64
	ExposureByEvent map[string]float64
	ExposureByBook  map[string]float64

	// Rate limiting
	BetsPlacedLastHour int
	BetsPlacedToday    int
	LastBetPlacedAt    time.Time

	// Performance tracking
	TodaysPnL         float64
	CurrentLossStreak int
	TotalBetsPlaced   int64
	TotalBetsWon      int64
	TotalBetsLost     int64

	// Safety circuit breakers
	IsPaused    bool
	PauseReason string
	PausedAt    time.Time
	PausedUntil time.Time

	LastUpdated time.Time
}

// Opportunity represents a detected betting opportunity
type Opportunity struct {
	ID              int64            `json:"id,omitempty"`
	OpportunityType string           `json:"opportunity_type"` // edge, middle, scalp
	SportKey        string           `json:"sport_key"`
	EventID         string           `json:"event_id"`
	MarketKey       string           `json:"market_key"`
	EdgePct         float64          `json:"edge_pct"`
	FairPrice       int              `json:"fair_price,omitempty"`
	DetectedAt      time.Time        `json:"detected_at"`
	DataAgeSeconds  int              `json:"data_age_seconds"`
	GameStartTime   time.Time        `json:"game_start_time,omitempty"`
	Legs            []OpportunityLeg `json:"legs"`
}

// OpportunityLeg represents a single bet leg within an opportunity
type OpportunityLeg struct {
	ID          int64    `json:"id,omitempty"`
	BookKey     string   `json:"book_key"`
	OutcomeName string   `json:"outcome_name"`
	Price       int      `json:"price"`           // American odds
	Point       *float64 `json:"point,omitempty"` // Spread/total line (can be nil for moneyline)
	LegEdgePct  float64  `json:"leg_edge_pct,omitempty"`
}

// FilterResult represents the result of a filter evaluation
type FilterResult struct {
	Passed   bool
	Reason   string
	Metadata map[string]interface{}
}

// ExecutionPlan defines how an opportunity should be executed
type ExecutionPlan struct {
	OpportunityID   int64
	OpportunityType string
	UserID          string
	Legs            []LegPlan
	Strategy        string // sequential, parallel, priority_ordered
	RequiredLegs    int
	TotalStake      float64
	Bankroll        float64 // Live balance from bot
}

// LegPlan defines execution plan for a single leg
type LegPlan struct {
	LegNumber        int
	OpportunityLegID int64
	BookKey          string
	OutcomeName      string
	Stake            float64
	Price            int
	Point            *float64
	Priority         int // For ordered execution (1 = highest)
	MaxRetries       int
}

// ExecutionResult contains the outcome of bet execution
type ExecutionResult struct {
	Status         string // success, partial, failed
	LegsPlaced     []LegResult
	LegsFailed     []LegResult
	TotalDuration  time.Duration
	RollbackNeeded bool
}

// LegResult contains the result for a single leg execution
type LegResult struct {
	LegNumber int
	BetID     *int64
	BookKey   string
	Stake     float64
	Status    string
	Error     error
	Duration  time.Duration
}

// BotBalance represents a bot's current balance
type BotBalance struct {
	BookKey     string
	Balance     float64
	FetchedAt   time.Time
	IsAvailable bool
}

// AutoBettingDecision represents a logged decision
type AutoBettingDecision struct {
	ID                int64
	UserID            string
	OpportunityID     int64
	Decision          string
	DecisionReason    string
	FilterResults     map[string]FilterResult
	CalculatedStake   *float64
	CalculatedEdge    *float64
	KellyFractionUsed *float64
	FullKellyPct      *float64
	BetIDs            []int64
	ExecutionTimeMs   *int
	CurrentExposure   float64
	CurrentBankroll   float64
	BetsPlacedToday   int
	CreatedAt         time.Time
}
