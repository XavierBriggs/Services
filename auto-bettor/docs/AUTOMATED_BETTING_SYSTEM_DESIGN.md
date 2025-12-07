# Automated Betting System - Complete Architecture & Implementation Plan

## Executive Summary

This document outlines the design and implementation of an **automated betting system** for Fortuna that builds on your existing infrastructure. The system will consume detected opportunities, apply sophisticated filtering and risk management rules, calculate optimal bet sizes using Kelly Criterion, and automatically place bets via your Talos bot infrastructure.

---

## 1. System Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                    AUTOMATED BETTING PIPELINE                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  opportunities.detected stream (existing)                           │
│           ↓                                                           │
│  ┌──────────────────────────────────────────────────────┐          │
│  │  AUTO-BETTOR SERVICE (NEW)                           │          │
│  ├──────────────────────────────────────────────────────┤          │
│  │  1. Stream Consumer (consumer group)                 │          │
│  │  2. Multi-Layer Filtering System                     │          │
│  │     ├─ User Preferences Filter                       │          │
│  │     ├─ Risk Management Filter                        │          │
│  │     ├─ Book Health Filter                            │          │
│  │     ├─ Exposure Limit Filter                         │          │
│  │     └─ Timing & Recency Filter                       │          │
│  │  3. Position Sizing Engine                           │          │
│  │     ├─ Kelly Calculation (existing service)          │          │
│  │     ├─ Bankroll Management                           │          │
│  │     ├─ Correlation Adjustments                       │          │
│  │     └─ Max Bet Caps                                  │          │
│  │  4. Execution Manager                                │          │
│  │     ├─ Bot Health Check                              │          │
│  │     ├─ Rate Limiting                                 │          │
│  │     ├─ Retry Logic                                   │          │
│  │     └─ Calls Bot Service (existing)                  │          │
│  │  5. Decision Logger                                  │          │
│  │     ├─ All decisions (placed/skipped)                │          │
│  │     ├─ Reasoning & metadata                          │          │
│  │     └─ Performance tracking                          │          │
│  └──────────────────────────────────────────────────────┘          │
│           ↓                                                           │
│  Bot Service (existing) → Talos Bots → Sportsbooks                 │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Bet Type Execution Strategies

### 2.1 Opportunity Type Comparison

The system handles three distinct opportunity types, each requiring different execution logic:

| Aspect | **EDGE** | **MIDDLE** | **SCALP** |
|--------|----------|-----------|----------|
| **Definition** | Single bet with +EV | Both sides have +EV | Guaranteed profit arbitrage |
| **Legs** | 1 | 2 | 2+ |
| **Sizing** | Kelly Criterion | Independent Kelly per leg | Equal profit distribution |
| **Execution** | Simple, single bet | Parallel preferred | MUST be parallel |
| **Failure Tolerance** | N/A (single bet) | Configurable (accept 1 or need 2) | Zero tolerance (all or none) |
| **Rollback** | None needed | Optional (accept partial) | **CRITICAL** (must complete or cancel) |
| **Time Sensitivity** | Low | Medium | **Very High** |
| **Priority** | Low | Medium | **Highest** |
| **Risk Level** | Low | Low-Medium | **HIGH if partial** |

### 2.2 EDGE Bets - Single Positive EV

**Characteristics**:
- One side of a market offers better odds than fair price
- 1 leg only
- Standard Kelly sizing
- Straightforward execution

**Example**:
```
Game: Lakers vs Celtics, Spread
Fair Price: Lakers -5.5 @ 52% true probability
FanDuel Odds: Lakers -5.5 @ +105 (2.05 decimal, 48.8% implied)
Edge: (52% / 48.8%) - 1 = +6.6% ✓
```

**Execution Strategy**:
- Place single bet immediately
- Standard retry logic (3 attempts)
- No coordination needed

### 2.3 MIDDLE Bets - Dual Positive EV with Win-Both Potential

**Characteristics**:
- Both sides of same market have +EV
- 2 legs, usually different books
- Potential to win both bets if result lands in "middle"
- Each leg profitable independently

**Example**:
```
Game: Lakers vs Celtics, Spread

Leg 1 (DraftKings): Lakers -4.5 @ +110 (edge: +5.0%)
Leg 2 (FanDuel): Celtics +6.5 @ -105 (edge: +5.5%)

Middle Window: Lakers win by 5 or 6 = BOTH BETS WIN
Outside Middle: Win one, lose one (still profit due to edges)
```

**Sizing Strategy**:
- Size each leg independently using Kelly
- Optional: Apply correlation discount (e.g., 50% reduction due to same-game exposure)
- User configurable

**Execution Strategy**:
- **Parallel Execution** (recommended): Place both legs simultaneously
  - Faster, locks in both prices
  - Requires multiple bot instances or goroutines
- **Sequential Execution** (alternative): Place higher-edge leg first
  - Safer if bot capacity limited
  - Risk: second leg odds may move

**Partial Fill Handling** (user configurable):
- **Option A**: `RequiredLegs = 2` - Need both legs or cancel
- **Option B**: `RequiredLegs = 1` - Accept single leg as edge bet
- **Recommended**: Option B (each leg is +EV independently)

### 2.4 SCALP Bets - Guaranteed Profit Arbitrage

**Characteristics**:
- Bet all outcomes to guarantee profit regardless of result
- 2+ legs (depends on market)
- NO Kelly - use equal profit distribution formula
- **Time critical** - arbitrage disappears fast

**Example**:
```
Game: Lakers vs Celtics, Total

Leg 1 (FanDuel): Over 220.5 @ +105 (2.05, 48.8% implied)
Leg 2 (DraftKings): Under 220.5 @ +105 (2.05, 48.8% implied)

Total Implied: 48.8% + 48.8% = 97.6%
Arb Profit: 100% - 97.6% = 2.4% guaranteed ✓
```

**Stake Distribution Formula**:
```
For each leg i:
Stake_i = (Total_Bankroll / Sum(1/Odds_j)) × (1/Odds_i)

Example with $1,000 total:
Stake_Over = $1,000 / (1/2.05 + 1/2.05) × (1/2.05) = $500
Stake_Under = $1,000 / (1/2.05 + 1/2.05) × (1/2.05) = $500

Profit if Over: $500 × 2.05 - $1,000 = $25
Profit if Under: $500 × 2.05 - $1,000 = $25
```

**Execution Strategy**:
- **MUST be parallel** - all legs simultaneously
- **30 second timeout** - speed critical
- **All-or-nothing** - partial execution creates risk

**Partial Fill = CRITICAL FAILURE**:
- If any leg fails, immediate rollback required:
  1. Try alternate books for missing legs
  2. Place hedge bets to minimize loss
  3. Alert human for manual intervention
  4. Last resort: Accept the unhedged exposure

### 2.5 Execution Priority Queue

When multiple opportunities arrive:

```
Priority 1 (CRITICAL): SCALP
  - Process immediately in parallel
  - 30s timeout
  - All-or-nothing execution

Priority 2 (HIGH): MIDDLE
  - Process with coordination
  - Configurable execution mode
  - Accept partial if configured

Priority 3 (NORMAL): EDGE
  - Standard processing
  - Single bet, no coordination
  - Lower time pressure
```

---

## 3. Database Schema Design

### 3.1 User Settings Extension (Holocron)

```sql
-- Extend user_settings table with auto-betting configuration
ALTER TABLE user_settings
  -- Master toggle
  ADD COLUMN auto_betting_enabled BOOLEAN DEFAULT FALSE,

  -- Edge & opportunity filters
  ADD COLUMN auto_min_edge_pct DECIMAL(6,3) DEFAULT 2.0,
  ADD COLUMN auto_enabled_opportunity_types VARCHAR[] DEFAULT ARRAY['edge']::VARCHAR[],
  ADD COLUMN auto_enabled_markets VARCHAR[] DEFAULT ARRAY['spreads']::VARCHAR[],
  ADD COLUMN auto_enabled_books VARCHAR[] DEFAULT ARRAY[]::VARCHAR[],
  ADD COLUMN auto_disabled_books VARCHAR[] DEFAULT ARRAY[]::VARCHAR[],

  -- Risk management
  ADD COLUMN auto_max_stake_per_bet DECIMAL(10,2) DEFAULT 100.00,
  ADD COLUMN auto_max_exposure_per_event DECIMAL(10,2) DEFAULT 200.00,
  ADD COLUMN auto_max_exposure_total DECIMAL(10,2) DEFAULT 1000.00,
  ADD COLUMN auto_max_bets_per_hour INTEGER DEFAULT 10,
  ADD COLUMN auto_max_bets_per_day INTEGER DEFAULT 50,

  -- Kelly sizing parameters
  ADD COLUMN auto_kelly_fraction DECIMAL(4,3) DEFAULT 0.250,
  ADD COLUMN auto_max_kelly_pct DECIMAL(5,2) DEFAULT 5.00,
  ADD COLUMN auto_min_stake DECIMAL(6,2) DEFAULT 10.00,

  -- Timing controls
  ADD COLUMN auto_max_data_age_seconds INTEGER DEFAULT 30,
  ADD COLUMN auto_min_time_to_start_hours INTEGER DEFAULT 1,
  ADD COLUMN auto_max_time_to_start_hours INTEGER DEFAULT 72,

  -- Safety features
  ADD COLUMN auto_require_confirmation BOOLEAN DEFAULT TRUE,
  ADD COLUMN auto_pause_on_loss_streak INTEGER DEFAULT 5,
  ADD COLUMN auto_pause_on_daily_loss DECIMAL(10,2) DEFAULT 500.00,

  -- Advanced features
  ADD COLUMN auto_correlation_discount DECIMAL(4,3) DEFAULT 0.500,
  ADD COLUMN auto_enable_hedging BOOLEAN DEFAULT FALSE,
  ADD COLUMN auto_hedge_threshold_pct DECIMAL(5,2) DEFAULT 10.00,

  -- Bet type specific settings
  -- Middle-specific
  ADD COLUMN auto_middle_execution_strategy VARCHAR(20) DEFAULT 'parallel', -- 'parallel' or 'sequential'
  ADD COLUMN auto_middle_required_legs INTEGER DEFAULT 1, -- 1 = accept single leg, 2 = need both
  ADD COLUMN auto_middle_max_time_between_legs_sec INTEGER DEFAULT 10, -- Max delay between legs

  -- Scalp-specific
  ADD COLUMN auto_scalp_enabled BOOLEAN DEFAULT FALSE, -- Separate toggle for arbs
  ADD COLUMN auto_scalp_bankroll_pct DECIMAL(5,2) DEFAULT 5.00, -- % of bankroll per scalp
  ADD COLUMN auto_scalp_min_profit_pct DECIMAL(5,2) DEFAULT 0.5, -- Minimum arb percentage
  ADD COLUMN auto_scalp_execution_timeout_sec INTEGER DEFAULT 30, -- Must complete fast

  -- Edge-specific
  ADD COLUMN auto_edge_allow_live_games BOOLEAN DEFAULT FALSE; -- Bet on live games?
```

### 2.2 Auto Betting Decisions Table

```sql
-- Track every automated betting decision
CREATE TABLE auto_betting_decisions (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL,
  opportunity_id BIGINT REFERENCES opportunities(id),

  -- Decision outcome
  decision VARCHAR(50) NOT NULL, -- 'placed', 'skipped_filter', 'skipped_risk', 'error', 'pending_confirmation'
  decision_reason TEXT,

  -- Filter results (JSON for detailed tracking)
  filter_results JSONB NOT NULL DEFAULT '{}'::JSONB,

  -- Bet details (if placed)
  calculated_stake DECIMAL(10,2),
  calculated_edge DECIMAL(8,4),
  kelly_fraction_used DECIMAL(4,3),
  full_kelly_pct DECIMAL(8,4),

  -- Execution details
  bet_id BIGINT REFERENCES bets(id),
  execution_time_ms INTEGER,

  -- Context
  current_exposure DECIMAL(10,2),
  current_bankroll DECIMAL(12,2),
  bets_placed_today INTEGER,

  -- Metadata
  created_at TIMESTAMPTZ DEFAULT NOW(),

  -- Indexes
  INDEX idx_auto_decisions_user_created (user_id, created_at DESC),
  INDEX idx_auto_decisions_opportunity (opportunity_id),
  INDEX idx_auto_decisions_decision (decision, created_at DESC)
);
```

### 2.3 Auto Betting State Table

```sql
-- Track current automated betting state per user
CREATE TABLE auto_betting_state (
  user_id UUID PRIMARY KEY,

  -- Current exposure tracking
  total_exposure DECIMAL(12,2) DEFAULT 0.00,
  exposure_by_event JSONB DEFAULT '{}'::JSONB, -- {"event_id": amount}
  exposure_by_book JSONB DEFAULT '{}'::JSONB,  -- {"book_key": amount}

  -- Rate limiting
  bets_placed_last_hour INTEGER DEFAULT 0,
  bets_placed_today INTEGER DEFAULT 0,
  last_bet_placed_at TIMESTAMPTZ,

  -- Performance tracking
  todays_pnl DECIMAL(10,2) DEFAULT 0.00,
  current_loss_streak INTEGER DEFAULT 0,
  total_bets_placed BIGINT DEFAULT 0,
  total_bets_won BIGINT DEFAULT 0,
  total_bets_lost BIGINT DEFAULT 0,

  -- Safety circuit breakers
  is_paused BOOLEAN DEFAULT FALSE,
  pause_reason TEXT,
  paused_at TIMESTAMPTZ,
  paused_until TIMESTAMPTZ,

  -- Metadata
  last_updated TIMESTAMPTZ DEFAULT NOW(),

  -- Constraints
  CHECK (total_exposure >= 0),
  CHECK (bets_placed_last_hour >= 0),
  CHECK (bets_placed_today >= 0)
);
```

### 2.4 Auto Betting Logs Table

```sql
-- Detailed audit trail for debugging and compliance
CREATE TABLE auto_betting_logs (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL,
  opportunity_id BIGINT,

  log_level VARCHAR(20) NOT NULL, -- 'debug', 'info', 'warning', 'error'
  log_type VARCHAR(50) NOT NULL,  -- 'opportunity_received', 'filter_check', 'kelly_calc', 'execution', etc.
  message TEXT NOT NULL,

  -- Structured data for analysis
  context JSONB DEFAULT '{}'::JSONB,

  created_at TIMESTAMPTZ DEFAULT NOW(),

  INDEX idx_auto_logs_user_created (user_id, created_at DESC),
  INDEX idx_auto_logs_type (log_type, created_at DESC)
);
```

### 3.5 Execution Tracking Tables

These tables track multi-leg bet execution for middles and scalps:

```sql
-- Track overall execution of multi-leg opportunities
CREATE TABLE auto_bet_execution_tracking (
  id BIGSERIAL PRIMARY KEY,
  auto_decision_id BIGINT REFERENCES auto_betting_decisions(id),
  opportunity_id BIGINT REFERENCES opportunities(id),
  opportunity_type VARCHAR(20), -- 'edge', 'middle', 'scalp'

  -- Execution plan
  total_legs INTEGER,
  legs_required_for_completion INTEGER, -- edge: 1, middle: 1 or 2, scalp: all
  execution_strategy VARCHAR(50), -- 'sequential', 'parallel', 'priority_ordered'

  -- Execution state
  status VARCHAR(50), -- 'pending', 'in_progress', 'completed', 'partial', 'failed', 'rolled_back'
  legs_placed INTEGER DEFAULT 0,
  legs_failed INTEGER DEFAULT 0,

  -- Timing
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  total_duration_ms INTEGER,

  -- Metadata
  execution_metadata JSONB DEFAULT '{}'::JSONB,

  created_at TIMESTAMPTZ DEFAULT NOW(),

  INDEX idx_execution_tracking_status (status, created_at DESC),
  INDEX idx_execution_tracking_opportunity (opportunity_id)
);

-- Track individual leg execution within multi-leg bets
CREATE TABLE auto_bet_leg_execution (
  id BIGSERIAL PRIMARY KEY,
  execution_tracking_id BIGINT REFERENCES auto_bet_execution_tracking(id),
  leg_number INTEGER, -- 1, 2, 3... (execution order)
  opportunity_leg_id BIGINT REFERENCES opportunity_legs(id),

  -- Leg details
  book_key VARCHAR(50),
  outcome_name VARCHAR(100),
  calculated_stake DECIMAL(10,2),

  -- Execution
  status VARCHAR(50), -- 'pending', 'in_progress', 'success', 'failed', 'cancelled'
  bet_id BIGINT REFERENCES bets(id),

  -- Retry tracking
  attempt_number INTEGER DEFAULT 1,
  max_attempts INTEGER DEFAULT 3,

  -- Timing
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  duration_ms INTEGER,

  -- Error handling
  error_message TEXT,
  should_retry BOOLEAN DEFAULT TRUE,

  created_at TIMESTAMPTZ DEFAULT NOW(),

  INDEX idx_leg_execution_tracking (execution_tracking_id, leg_number),
  INDEX idx_leg_execution_status (status, created_at DESC)
);
```

---

## 4. Service Implementation Design

### 4.1 Auto Bettor Service Structure

```
services/auto-bettor/
├── cmd/
│   └── auto-bettor/
│       └── main.go                    # Service entry point
├── internal/
│   ├── consumer/
│   │   └── stream_consumer.go         # Redis stream consumer
│   ├── filters/
│   │   ├── filter.go                  # Filter interface
│   │   ├── user_preferences.go        # User settings filter
│   │   ├── risk_management.go         # Risk/exposure limits
│   │   ├── book_health.go             # Bot availability check
│   │   ├── timing.go                  # Data age, game start time
│   │   └── correlation.go             # Same-game parlay detection
│   ├── sizing/
│   │   ├── kelly.go                   # Kelly calculation wrapper
│   │   ├── scalp_sizing.go            # Equal profit distribution
│   │   ├── bankroll.go                # Bankroll tracking
│   │   └── correlation_adjuster.go    # Correlated bet sizing
│   ├── execution/
│   │   ├── strategy.go                # ExecutionStrategy interface
│   │   ├── edge_strategy.go           # Single bet execution
│   │   ├── middle_strategy.go         # 2-leg coordinated execution
│   │   ├── scalp_strategy.go          # Multi-leg parallel execution
│   │   ├── executor.go                # Bet placement manager
│   │   ├── parallel_executor.go       # Parallel leg execution (goroutines)
│   │   ├── rate_limiter.go            # Rate limiting logic
│   │   └── retry.go                   # Retry with backoff
│   ├── queue/
│   │   └── priority_queue.go          # Priority-based opportunity queue
│   ├── state/
│   │   ├── state_manager.go           # Auto betting state CRUD
│   │   └── exposure_tracker.go        # Real-time exposure tracking
│   ├── logger/
│   │   └── decision_logger.go         # Log all decisions
│   ├── config/
│   │   └── config.go                  # Service configuration
│   └── models/
│       └── models.go                  # Data structures
├── pkg/
│   └── circuit_breaker/
│       └── breaker.go                 # Circuit breaker implementation
├── go.mod
└── go.sum
```

### 4.2 Core Logic Flow with Strategy Pattern

```go
// High-level pseudocode for main processing loop

func (ab *AutoBettor) ProcessOpportunity(opp Opportunity) {
    // 1. Load user settings
    settings := ab.stateManager.GetUserSettings(opp.UserID)

    if !settings.AutoBettingEnabled {
        return // Auto-betting disabled
    }

    // 2. Check opportunity type specific toggles
    if opp.OpportunityType == "scalp" && !settings.AutoScalpEnabled {
        ab.logger.LogSkipped(opp, "scalp_disabled", "scalp auto-betting disabled")
        return
    }

    // 3. Check circuit breakers
    state := ab.stateManager.GetState(opp.UserID)
    if state.IsPaused {
        ab.logger.LogSkipped(opp, "circuit_breaker", state.PauseReason)
        return
    }

    // 4. Run filter chain
    filterResults := make(map[string]FilterResult)

    filters := []Filter{
        ab.filters.UserPreferences,
        ab.filters.RiskManagement,
        ab.filters.BookHealth,
        ab.filters.Timing,
        ab.filters.Correlation,
    }

    for _, filter := range filters {
        result := filter.Evaluate(opp, settings, state)
        filterResults[filter.Name()] = result

        if !result.Passed {
            ab.logger.LogSkipped(opp, filter.Name(), result.Reason)
            ab.saveDecision(opp, "skipped_filter", result.Reason, filterResults)
            return
        }
    }

    // 5. Select execution strategy based on opportunity type
    var strategy ExecutionStrategy
    switch opp.OpportunityType {
    case "edge":
        strategy = ab.strategies.Edge
    case "middle":
        strategy = ab.strategies.Middle
    case "scalp":
        strategy = ab.strategies.Scalp
    default:
        ab.logger.LogError(opp, "unknown_type", fmt.Errorf("unknown opportunity type: %s", opp.OpportunityType))
        return
    }

    // 6. Create execution plan (strategy-specific sizing)
    plan, err := strategy.Plan(opp, settings, state)
    if err != nil {
        ab.logger.LogError(opp, "planning_error", err)
        ab.saveDecision(opp, "error", err.Error(), filterResults)
        return
    }

    // 7. Validate stake is above minimum
    if plan.TotalStake < settings.AutoMinStake {
        ab.logger.LogSkipped(opp, "stake_too_small",
            fmt.Sprintf("%.2f < %.2f", plan.TotalStake, settings.AutoMinStake))
        ab.saveDecision(opp, "skipped_risk", "stake below minimum", filterResults)
        return
    }

    // 8. Check if confirmation required
    if settings.AutoRequireConfirmation && !opp.IsConfirmed {
        ab.saveDecision(opp, "pending_confirmation", "awaiting user confirmation", filterResults)
        ab.notifyUser(opp, plan)
        return
    }

    // 9. Execute the plan (strategy-specific)
    executionResult, err := strategy.Execute(plan)
    if err != nil || executionResult.Status == "failed" {
        ab.logger.LogError(opp, "execution_error", err)
        ab.saveDecision(opp, "error", err.Error(), filterResults)
        ab.checkErrorCircuitBreaker(opp.UserID, err)

        // Attempt rollback if needed
        if executionResult != nil && executionResult.RollbackNeeded {
            strategy.Rollback(plan, executionResult)
        }
        return
    }

    // 10. Handle partial execution
    if executionResult.Status == "partial" {
        ab.logger.LogWarning(opp, "partial_execution",
            fmt.Sprintf("placed %d of %d legs", len(executionResult.LegsPlaced), plan.TotalLegs))

        // Update state for placed legs
        for _, legResult := range executionResult.LegsPlaced {
            ab.stateManager.RecordBetPlaced(opp.UserID, legResult.Stake, opp.EventID, legResult.BookKey)
        }

        ab.saveDecision(opp, "partial", "partial execution", filterResults, executionResult)
        return
    }

    // 11. Success - update state for all legs
    for _, legResult := range executionResult.LegsPlaced {
        ab.stateManager.RecordBetPlaced(opp.UserID, legResult.Stake, opp.EventID, legResult.BookKey)
    }

    // 12. Log successful placement
    ab.logger.LogSuccess(opp, executionResult)
    ab.saveDecision(opp, "placed", "all legs placed successfully", filterResults, executionResult)
}
```

### 4.3 Execution Strategy Interface

```go
type ExecutionStrategy interface {
    // Plan creates an execution plan for this opportunity
    Plan(opp Opportunity, settings UserSettings, state AutoBettingState) (*ExecutionPlan, error)

    // Execute carries out the execution plan
    Execute(plan *ExecutionPlan) (*ExecutionResult, error)

    // Rollback handles partial execution failures
    Rollback(plan *ExecutionPlan, result *ExecutionResult) error
}

type ExecutionPlan struct {
    OpportunityID   int64
    OpportunityType string          // "edge", "middle", "scalp"
    Legs            []LegPlan
    Strategy        string          // "sequential", "parallel", "priority_ordered"
    RequiredLegs    int             // How many legs must succeed
    TotalLegs       int
    TotalStake      float64
}

type LegPlan struct {
    LegNumber        int
    OpportunityLegID int64
    BookKey          string
    OutcomeName      string
    Stake            float64
    Priority         int    // For ordered execution (1 = highest)
    MaxRetries       int
}

type ExecutionResult struct {
    Status          string      // "success", "partial", "failed"
    LegsPlaced      []LegResult
    LegsFailed      []LegResult
    TotalDuration   time.Duration
    RollbackNeeded  bool
}

type LegResult struct {
    LegNumber   int
    BetID       int64
    BookKey     string
    Stake       float64
    Status      string
    Error       error
    Duration    time.Duration
}
```

---

## 4. Filter Implementation Details

### 4.1 User Preferences Filter

**Purpose**: Ensure opportunity matches user's configured preferences

**Checks**:
- Opportunity type (edge, middle, scalp) in `auto_enabled_opportunity_types`
- Market type in `auto_enabled_markets`
- Book in `auto_enabled_books` OR not in `auto_disabled_books`
- Edge percentage >= `auto_min_edge_pct`

```go
func (f *UserPreferencesFilter) Evaluate(opp Opportunity, settings UserSettings, state State) FilterResult {
    // Check opportunity type
    if !contains(settings.AutoEnabledOpportunityTypes, opp.OpportunityType) {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("opportunity type '%s' not enabled", opp.OpportunityType),
        }
    }

    // Check market type
    if !contains(settings.AutoEnabledMarkets, opp.MarketKey) {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("market '%s' not enabled", opp.MarketKey),
        }
    }

    // Check book (whitelist if provided, otherwise blacklist)
    if len(settings.AutoEnabledBooks) > 0 {
        if !contains(settings.AutoEnabledBooks, opp.BookKey) {
            return FilterResult{
                Passed: false,
                Reason: fmt.Sprintf("book '%s' not in whitelist", opp.BookKey),
            }
        }
    } else {
        if contains(settings.AutoDisabledBooks, opp.BookKey) {
            return FilterResult{
                Passed: false,
                Reason: fmt.Sprintf("book '%s' is blacklisted", opp.BookKey),
            }
        }
    }

    // Check minimum edge
    if opp.EdgePct < settings.AutoMinEdgePct {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("edge %.2f%% < minimum %.2f%%", opp.EdgePct, settings.AutoMinEdgePct),
        }
    }

    return FilterResult{Passed: true, Reason: "passed all preference checks"}
}
```

### 4.2 Risk Management Filter

**Purpose**: Enforce exposure limits and betting frequency caps

**Checks**:
- Total exposure < `auto_max_exposure_total`
- Event exposure < `auto_max_exposure_per_event`
- Proposed stake < `auto_max_stake_per_bet`
- Bets this hour < `auto_max_bets_per_hour`
- Bets today < `auto_max_bets_per_day`
- Not in loss streak pause (if enabled)
- Daily loss not exceeded

```go
func (f *RiskManagementFilter) Evaluate(opp Opportunity, settings UserSettings, state State) FilterResult {
    // Check total exposure
    if state.TotalExposure >= settings.AutoMaxExposureTotal {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("total exposure $%.2f >= limit $%.2f", state.TotalExposure, settings.AutoMaxExposureTotal),
        }
    }

    // Check per-event exposure
    eventExposure := state.ExposureByEvent[opp.EventID]
    if eventExposure >= settings.AutoMaxExposurePerEvent {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("event exposure $%.2f >= limit $%.2f", eventExposure, settings.AutoMaxExposurePerEvent),
        }
    }

    // Check rate limits
    if state.BetsPlacedLastHour >= settings.AutoMaxBetsPerHour {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("hourly bet limit reached (%d/%d)", state.BetsPlacedLastHour, settings.AutoMaxBetsPerHour),
        }
    }

    if state.BetsPlacedToday >= settings.AutoMaxBetsPerDay {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("daily bet limit reached (%d/%d)", state.BetsPlacedToday, settings.AutoMaxBetsPerDay),
        }
    }

    // Check loss streak circuit breaker
    if settings.AutoPauseOnLossStreak > 0 && state.CurrentLossStreak >= settings.AutoPauseOnLossStreak {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("paused due to loss streak (%d losses)", state.CurrentLossStreak),
        }
    }

    // Check daily loss limit
    if settings.AutoPauseOnDailyLoss > 0 && state.TodaysPnL <= -settings.AutoPauseOnDailyLoss {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("paused due to daily loss $%.2f", state.TodaysPnL),
        }
    }

    return FilterResult{Passed: true, Reason: "passed all risk checks"}
}
```

### 4.3 Book Health Filter

**Purpose**: Only bet on books with healthy bot connections

**Checks**:
- Bot manager is reachable
- Specific book bot is online
- Book bot is logged in
- No recent execution errors (circuit breaker)

```go
func (f *BookHealthFilter) Evaluate(opp Opportunity, settings UserSettings, state State) FilterResult {
    // Check bot manager health
    health, err := f.botClient.CheckHealth()
    if err != nil {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("bot manager unreachable: %v", err),
        }
    }

    // Check specific book bot
    bookHealth, exists := health.Bots[opp.BookKey]
    if !exists {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("no bot configured for book '%s'", opp.BookKey),
        }
    }

    if !bookHealth.LoggedIn {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("bot for '%s' not logged in", opp.BookKey),
        }
    }

    // Check circuit breaker for this book
    if f.circuitBreaker.IsOpen(opp.BookKey) {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("circuit breaker open for '%s' due to recent errors", opp.BookKey),
        }
    }

    return FilterResult{Passed: true, Reason: "book bot healthy"}
}
```

### 4.4 Timing Filter

**Purpose**: Ensure data freshness and appropriate game timing

**Checks**:
- Data age < `auto_max_data_age_seconds`
- Time to game start >= `auto_min_time_to_start_hours`
- Time to game start <= `auto_max_time_to_start_hours`
- Game not already started

```go
func (f *TimingFilter) Evaluate(opp Opportunity, settings UserSettings, state State) FilterResult {
    now := time.Now()

    // Check data age
    dataAge := now.Sub(opp.DetectedAt).Seconds()
    if dataAge > float64(settings.AutoMaxDataAgeSeconds) {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("data %.0fs old > limit %ds", dataAge, settings.AutoMaxDataAgeSeconds),
        }
    }

    // Check game hasn't started
    if now.After(opp.GameStartTime) {
        return FilterResult{
            Passed: false,
            Reason: "game already started",
        }
    }

    // Check minimum time to start
    hoursToStart := opp.GameStartTime.Sub(now).Hours()
    if hoursToStart < float64(settings.AutoMinTimeToStartHours) {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("only %.1fh to start < minimum %dh", hoursToStart, settings.AutoMinTimeToStartHours),
        }
    }

    // Check maximum time to start
    if hoursToStart > float64(settings.AutoMaxTimeToStartHours) {
        return FilterResult{
            Passed: false,
            Reason: fmt.Sprintf("%.1fh to start > maximum %dh", hoursToStart, settings.AutoMaxTimeToStartHours),
        }
    }

    return FilterResult{Passed: true, Reason: "timing checks passed"}
}
```

### 4.5 Correlation Filter

**Purpose**: Detect and adjust for correlated bets (same game)

**Checks**:
- Check if we already have bets on this game
- Apply correlation discount to Kelly fraction if correlated
- Prevent over-concentration on single game

```go
func (f *CorrelationFilter) Evaluate(opp Opportunity, settings UserSettings, state State) FilterResult {
    // Check existing bets on this event
    eventExposure := state.ExposureByEvent[opp.EventID]

    if eventExposure > 0 {
        // We have correlated exposure
        correlationFactor := settings.AutoCorrelationDiscount

        return FilterResult{
            Passed: true,
            Reason: fmt.Sprintf("correlated bet (existing exposure $%.2f), applying %.1f%% discount", eventExposure, correlationFactor*100),
            Metadata: map[string]interface{}{
                "correlation_discount": correlationFactor,
                "existing_exposure":    eventExposure,
            },
        }
    }

    return FilterResult{Passed: true, Reason: "no correlation detected"}
}
```

---

## 5. Position Sizing Implementation

### 5.1 Kelly Calculation Wrapper

```go
func (s *SizingEngine) CalculateStake(req SizingRequest) (*SizingResult, error) {
    // Call existing Kelly Calculator service
    kellyResp, err := s.kellyClient.CalculateFromOpportunity(KellyRequest{
        OpportunityID: req.Opportunity.ID,
        Bankroll:      req.Bankroll,
        KellyFraction: req.Settings.AutoKellyFraction,
    })
    if err != nil {
        return nil, fmt.Errorf("kelly calculation failed: %w", err)
    }

    // Apply correlation discount if present
    stake := kellyResp.Stake
    if req.CorrelationDiscount > 0 {
        stake = stake * (1 - req.CorrelationDiscount)
    }

    // Cap at max Kelly percentage
    maxStake := req.Bankroll * (req.Settings.AutoMaxKellyPct / 100.0)
    if stake > maxStake {
        stake = maxStake
    }

    // Enforce absolute max stake per bet
    if stake > req.Settings.AutoMaxStakePerBet {
        stake = req.Settings.AutoMaxStakePerBet
    }

    // Round to nearest dollar
    stake = math.Round(stake)

    return &SizingResult{
        Stake:           stake,
        FullKellyPct:    kellyResp.FullKellyPct,
        FractionalKelly: kellyResp.FractionalKelly,
        ExpectedValue:   kellyResp.ExpectedValue,
        EVPerDollar:     kellyResp.EVPerDollar,
        CapReason:       s.determineCapReason(stake, req),
    }, nil
}
```

---

## 6. Execution Manager Implementation

### 6.1 Rate Limiting

```go
type RateLimiter struct {
    redisClient *redis.Client
}

func (rl *RateLimiter) CheckRateLimit(userID string, limit int, window time.Duration) (bool, error) {
    key := fmt.Sprintf("ratelimit:%s:%s", userID, window.String())

    // Increment counter
    count, err := rl.redisClient.Incr(context.Background(), key).Result()
    if err != nil {
        return false, err
    }

    // Set expiry on first increment
    if count == 1 {
        rl.redisClient.Expire(context.Background(), key, window)
    }

    return count <= int64(limit), nil
}
```

### 6.2 Bet Execution with Retry

```go
func (e *Executor) PlaceBet(req BetPlacementRequest) (*BetResult, error) {
    var lastErr error

    for attempt := 1; attempt <= e.maxRetries; attempt++ {
        result, err := e.executeBet(req)
        if err == nil {
            return result, nil
        }

        lastErr = err

        // Don't retry on user errors (invalid bet, insufficient funds)
        if isUserError(err) {
            return nil, fmt.Errorf("user error (no retry): %w", err)
        }

        // Exponential backoff
        if attempt < e.maxRetries {
            backoff := time.Duration(attempt*attempt) * time.Second
            time.Sleep(backoff)
        }
    }

    return nil, fmt.Errorf("failed after %d attempts: %w", e.maxRetries, lastErr)
}

func (e *Executor) executeBet(req BetPlacementRequest) (*BetResult, error) {
    // Call bot-service (existing)
    resp, err := e.botServiceClient.PlaceBet(BotServiceRequest{
        BookKey:    req.BookKey,
        OpportunityID: req.OpportunityID,
        Stake:      req.Stake,
        // ... map other fields
    })
    if err != nil {
        return nil, err
    }

    return &BetResult{
        BetID:        resp.BetID,
        TicketNumber: resp.TicketNumber,
        PlacedAt:     resp.PlacedAt,
    }, nil
}
```

---

## 7. State Management

### 7.1 Real-Time Exposure Tracking

```go
func (sm *StateManager) RecordBetPlaced(userID string, stake float64, eventID int64, bookKey string) error {
    state, err := sm.GetState(userID)
    if err != nil {
        return err
    }

    // Update total exposure
    state.TotalExposure += stake

    // Update per-event exposure
    if state.ExposureByEvent == nil {
        state.ExposureByEvent = make(map[int64]float64)
    }
    state.ExposureByEvent[eventID] += stake

    // Update per-book exposure
    if state.ExposureByBook == nil {
        state.ExposureByBook = make(map[string]float64)
    }
    state.ExposureByBook[bookKey] += stake

    // Update rate limiting counters
    state.BetsPlacedLastHour++
    state.BetsPlacedToday++
    state.LastBetPlacedAt = time.Now()

    // Persist to database
    return sm.SaveState(state)
}

func (sm *StateManager) RecordBetSettled(userID string, stake float64, payout float64, eventID int64, bookKey string, result string) error {
    state, err := sm.GetState(userID)
    if err != nil {
        return err
    }

    // Update exposure (reduce by stake)
    state.TotalExposure -= stake
    state.ExposureByEvent[eventID] -= stake
    state.ExposureByBook[bookKey] -= stake

    // Update P&L
    profit := payout - stake
    state.TodaysPnL += profit

    // Update win/loss tracking
    if result == "win" {
        state.TotalBetsWon++
        state.CurrentLossStreak = 0
    } else if result == "loss" {
        state.TotalBetsLost++
        state.CurrentLossStreak++

        // Check if we should pause
        if state.CurrentLossStreak >= sm.pauseOnLossStreak {
            state.IsPaused = true
            state.PauseReason = fmt.Sprintf("loss streak of %d", state.CurrentLossStreak)
            state.PausedAt = time.Now()
        }
    }

    return sm.SaveState(state)
}
```

---

## 8. Safety Features & Circuit Breakers

### 8.1 Circuit Breaker Implementation

```go
type CircuitBreaker struct {
    redis       *redis.Client
    maxFailures int
    resetAfter  time.Duration
}

func (cb *CircuitBreaker) RecordFailure(bookKey string) error {
    key := fmt.Sprintf("circuit:%s:failures", bookKey)

    count, err := cb.redis.Incr(context.Background(), key).Result()
    if err != nil {
        return err
    }

    if count == 1 {
        cb.redis.Expire(context.Background(), key, cb.resetAfter)
    }

    if count >= int64(cb.maxFailures) {
        // Open circuit
        cb.redis.Set(context.Background(),
            fmt.Sprintf("circuit:%s:open", bookKey),
            "1",
            cb.resetAfter,
        )
    }

    return nil
}

func (cb *CircuitBreaker) IsOpen(bookKey string) bool {
    val, err := cb.redis.Get(context.Background(),
        fmt.Sprintf("circuit:%s:open", bookKey),
    ).Result()

    return err == nil && val == "1"
}
```

### 8.2 Auto-Pause Triggers

1. **Loss Streak**: Pause after N consecutive losses
2. **Daily Loss Limit**: Pause if daily P&L drops below threshold
3. **Error Rate**: Pause if execution errors exceed threshold
4. **Manual Override**: User can pause via UI
5. **Bankroll Depletion**: Pause if bankroll drops below minimum

---

## 9. Frontend Integration

### 9.1 Settings Page Component

```tsx
// web/fortuna_client/src/components/AutoBettingSettings.tsx

interface AutoBettingSettingsProps {
  userID: string;
}

export function AutoBettingSettings({ userID }: AutoBettingSettingsProps) {
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [state, setState] = useState<AutoBettingState | null>(null);

  return (
    <div className="auto-betting-settings">
      <h2>Automated Betting Configuration</h2>

      {/* Master Toggle */}
      <div className="setting-group">
        <label>
          <input
            type="checkbox"
            checked={settings?.autoBettingEnabled}
            onChange={(e) => updateSetting('autoBettingEnabled', e.target.checked)}
          />
          Enable Automated Betting
        </label>
        {settings?.autoBettingEnabled && (
          <div className="status-badge">
            {state?.isPaused ? (
              <span className="paused">⏸ PAUSED: {state.pauseReason}</span>
            ) : (
              <span className="active">✓ ACTIVE</span>
            )}
          </div>
        )}
      </div>

      {/* Opportunity Filters */}
      <div className="setting-group">
        <h3>Opportunity Filters</h3>

        <label>
          Minimum Edge %
          <input
            type="number"
            step="0.1"
            value={settings?.autoMinEdgePct}
            onChange={(e) => updateSetting('autoMinEdgePct', parseFloat(e.target.value))}
          />
        </label>

        <label>
          Enabled Opportunity Types
          <select
            multiple
            value={settings?.autoEnabledOpportunityTypes}
            onChange={(e) => updateSetting('autoEnabledOpportunityTypes', getSelectedOptions(e))}
          >
            <option value="edge">Edge Bets</option>
            <option value="middle">Middles</option>
            <option value="scalp">Scalps</option>
          </select>
        </label>

        <label>
          Enabled Markets
          <select
            multiple
            value={settings?.autoEnabledMarkets}
            onChange={(e) => updateSetting('autoEnabledMarkets', getSelectedOptions(e))}
          >
            <option value="h2h">Moneyline</option>
            <option value="spreads">Spreads</option>
            <option value="totals">Totals</option>
          </select>
        </label>
      </div>

      {/* Risk Management */}
      <div className="setting-group">
        <h3>Risk Management</h3>

        <label>
          Max Stake Per Bet ($)
          <input
            type="number"
            value={settings?.autoMaxStakePerBet}
            onChange={(e) => updateSetting('autoMaxStakePerBet', parseFloat(e.target.value))}
          />
        </label>

        <label>
          Max Total Exposure ($)
          <input
            type="number"
            value={settings?.autoMaxExposureTotal}
            onChange={(e) => updateSetting('autoMaxExposureTotal', parseFloat(e.target.value))}
          />
          <span className="current-value">
            Current: ${state?.totalExposure.toFixed(2)}
          </span>
        </label>

        <label>
          Max Bets Per Day
          <input
            type="number"
            value={settings?.autoMaxBetsPerDay}
            onChange={(e) => updateSetting('autoMaxBetsPerDay', parseInt(e.target.value))}
          />
          <span className="current-value">
            Today: {state?.betsPlacedToday}
          </span>
        </label>
      </div>

      {/* Kelly Parameters */}
      <div className="setting-group">
        <h3>Kelly Sizing</h3>

        <label>
          Kelly Fraction
          <input
            type="number"
            step="0.05"
            min="0.05"
            max="1.0"
            value={settings?.autoKellyFraction}
            onChange={(e) => updateSetting('autoKellyFraction', parseFloat(e.target.value))}
          />
          <span className="help-text">
            0.25 = Quarter Kelly (conservative), 1.0 = Full Kelly (aggressive)
          </span>
        </label>

        <label>
          Max Kelly % of Bankroll
          <input
            type="number"
            step="0.5"
            value={settings?.autoMaxKellyPct}
            onChange={(e) => updateSetting('autoMaxKellyPct', parseFloat(e.target.value))}
          />
        </label>
      </div>

      {/* Safety Features */}
      <div className="setting-group">
        <h3>Safety Features</h3>

        <label>
          <input
            type="checkbox"
            checked={settings?.autoRequireConfirmation}
            onChange={(e) => updateSetting('autoRequireConfirmation', e.target.checked)}
          />
          Require confirmation before placing bets
        </label>

        <label>
          Pause After Loss Streak
          <input
            type="number"
            value={settings?.autoPauseOnLossStreak}
            onChange={(e) => updateSetting('autoPauseOnLossStreak', parseInt(e.target.value))}
          />
          <span className="help-text">0 = disabled</span>
        </label>

        <label>
          Pause After Daily Loss ($)
          <input
            type="number"
            value={settings?.autoPauseOnDailyLoss}
            onChange={(e) => updateSetting('autoPauseOnDailyLoss', parseFloat(e.target.value))}
          />
          <span className="current-value">
            Today's P&L: ${state?.todaysPnL.toFixed(2)}
          </span>
        </label>
      </div>

      {/* Manual Controls */}
      <div className="setting-group">
        <h3>Manual Controls</h3>

        {state?.isPaused ? (
          <button onClick={resumeAutoBetting} className="btn-primary">
            Resume Auto-Betting
          </button>
        ) : (
          <button onClick={pauseAutoBetting} className="btn-warning">
            Pause Auto-Betting
          </button>
        )}
      </div>
    </div>
  );
}
```

### 9.2 Auto Betting Dashboard

```tsx
// web/fortuna_client/src/components/AutoBettingDashboard.tsx

export function AutoBettingDashboard({ userID }: { userID: string }) {
  const [decisions, setDecisions] = useState<AutoBettingDecision[]>([]);
  const [stats, setStats] = useState<AutoBettingStats | null>(null);

  return (
    <div className="auto-betting-dashboard">
      <h2>Automated Betting Activity</h2>

      {/* Stats Overview */}
      <div className="stats-grid">
        <div className="stat-card">
          <h3>Total Auto Bets</h3>
          <div className="stat-value">{stats?.totalBets}</div>
        </div>

        <div className="stat-card">
          <h3>Win Rate</h3>
          <div className="stat-value">
            {((stats?.betsWon / stats?.totalBets) * 100).toFixed(1)}%
          </div>
        </div>

        <div className="stat-card">
          <h3>Auto ROI</h3>
          <div className={`stat-value ${stats?.roi >= 0 ? 'positive' : 'negative'}`}>
            {stats?.roi.toFixed(2)}%
          </div>
        </div>

        <div className="stat-card">
          <h3>Total Profit</h3>
          <div className={`stat-value ${stats?.totalProfit >= 0 ? 'positive' : 'negative'}`}>
            ${stats?.totalProfit.toFixed(2)}
          </div>
        </div>
      </div>

      {/* Recent Decisions */}
      <div className="decisions-table">
        <h3>Recent Auto-Betting Decisions</h3>
        <table>
          <thead>
            <tr>
              <th>Time</th>
              <th>Opportunity</th>
              <th>Decision</th>
              <th>Reason</th>
              <th>Stake</th>
              <th>Edge</th>
              <th>Result</th>
            </tr>
          </thead>
          <tbody>
            {decisions.map(decision => (
              <tr key={decision.id} className={`decision-${decision.decision}`}>
                <td>{formatTime(decision.createdAt)}</td>
                <td>
                  <Link to={`/opportunities/${decision.opportunityId}`}>
                    {decision.opportunityType} - {decision.marketKey}
                  </Link>
                </td>
                <td>
                  <span className={`badge badge-${decision.decision}`}>
                    {decision.decision}
                  </span>
                </td>
                <td className="reason-cell">{decision.decisionReason}</td>
                <td>${decision.calculatedStake?.toFixed(2)}</td>
                <td>{decision.calculatedEdge?.toFixed(2)}%</td>
                <td>
                  {decision.betId && (
                    <BetResult betId={decision.betId} />
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Filter Breakdown Chart */}
      <div className="filter-breakdown">
        <h3>Why Opportunities Were Skipped</h3>
        <PieChart data={stats?.skipReasons} />
      </div>
    </div>
  );
}
```

---

## 10. API Endpoints

### 10.1 Auto Betting Settings API

```go
// GET /api/v1/auto-betting/settings
func (h *Handler) GetAutoBettingSettings(w http.ResponseWriter, r *http.Request) {
    userID := getUserIDFromContext(r.Context())

    settings, err := h.db.GetUserSettings(userID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    state, err := h.stateManager.GetState(userID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    response := AutoBettingSettingsResponse{
        Settings: settings,
        State:    state,
    }

    json.NewEncoder(w).Encode(response)
}

// PUT /api/v1/auto-betting/settings
func (h *Handler) UpdateAutoBettingSettings(w http.ResponseWriter, r *http.Request) {
    userID := getUserIDFromContext(r.Context())

    var req UpdateSettingsRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Validate settings
    if err := h.validator.Validate(req.Settings); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Update in database
    if err := h.db.UpdateUserSettings(userID, req.Settings); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}
```

### 10.2 State Management API

```go
// POST /api/v1/auto-betting/pause
func (h *Handler) PauseAutoBetting(w http.ResponseWriter, r *http.Request) {
    userID := getUserIDFromContext(r.Context())

    var req PauseRequest
    json.NewDecoder(r.Body).Decode(&req)

    err := h.stateManager.Pause(userID, req.Reason, req.Duration)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}

// POST /api/v1/auto-betting/resume
func (h *Handler) ResumeAutoBetting(w http.ResponseWriter, r *http.Request) {
    userID := getUserIDFromContext(r.Context())

    err := h.stateManager.Resume(userID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}
```

### 10.3 Decision History API

```go
// GET /api/v1/auto-betting/decisions
func (h *Handler) GetAutoBettingDecisions(w http.ResponseWriter, r *http.Request) {
    userID := getUserIDFromContext(r.Context())

    // Parse query params
    limit := getIntParam(r, "limit", 50)
    offset := getIntParam(r, "offset", 0)
    decisionFilter := r.URL.Query().Get("decision") // "", "placed", "skipped_filter", etc.

    decisions, total, err := h.db.GetAutoBettingDecisions(userID, limit, offset, decisionFilter)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    response := DecisionsResponse{
        Decisions: decisions,
        Total:     total,
        Limit:     limit,
        Offset:    offset,
    }

    json.NewEncoder(w).Encode(response)
}
```

---

## 11. Monitoring & Observability

### 11.1 Key Metrics to Track

```go
// Prometheus metrics
var (
    opportunitiesReceived = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "auto_bettor_opportunities_received_total",
            Help: "Total opportunities received from stream",
        },
        []string{"sport", "opportunity_type"},
    )

    betsPlaced = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "auto_bettor_bets_placed_total",
            Help: "Total bets placed automatically",
        },
        []string{"sport", "book", "market"},
    )

    betsSkipped = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "auto_bettor_bets_skipped_total",
            Help: "Total bets skipped",
        },
        []string{"filter_name", "reason"},
    )

    executionLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "auto_bettor_execution_latency_seconds",
            Help: "Time from opportunity detection to bet placement",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
        },
        []string{"book"},
    )

    filterLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "auto_bettor_filter_latency_seconds",
            Help: "Time spent in each filter",
            Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1},
        },
        []string{"filter_name"},
    )

    totalExposure = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "auto_bettor_total_exposure_dollars",
            Help: "Current total exposure in dollars",
        },
        []string{"user_id"},
    )
)
```

### 11.2 Logging Strategy

```go
// Structured logging with context
type DecisionLogger struct {
    logger *zap.Logger
}

func (dl *DecisionLogger) LogOpportunityReceived(opp Opportunity) {
    dl.logger.Info("opportunity_received",
        zap.Int64("opportunity_id", opp.ID),
        zap.String("sport", opp.SportKey),
        zap.String("type", opp.OpportunityType),
        zap.Float64("edge_pct", opp.EdgePct),
        zap.Time("detected_at", opp.DetectedAt),
    )
}

func (dl *DecisionLogger) LogFilterResult(opp Opportunity, filterName string, result FilterResult) {
    dl.logger.Info("filter_result",
        zap.Int64("opportunity_id", opp.ID),
        zap.String("filter", filterName),
        zap.Bool("passed", result.Passed),
        zap.String("reason", result.Reason),
        zap.Any("metadata", result.Metadata),
    )
}

func (dl *DecisionLogger) LogBetPlaced(opp Opportunity, stake float64, betID int64) {
    dl.logger.Info("bet_placed",
        zap.Int64("opportunity_id", opp.ID),
        zap.Int64("bet_id", betID),
        zap.Float64("stake", stake),
        zap.String("book", opp.BookKey),
        zap.String("market", opp.MarketKey),
    )
}
```

---

## 12. Deployment Strategy

### 12.1 Docker Compose Addition

```yaml
# Add to deploy/docker-compose.yml

services:
  auto-bettor:
    build:
      context: ../services/auto-bettor
      dockerfile: Dockerfile
    container_name: auto-bettor
    environment:
      - REDIS_URL=redis:6379
      - ALEXANDRIA_DSN=postgres://user:pass@alexandria:5432/alexandria
      - HOLOCRON_DSN=postgres://user:pass@holocron:5432/holocron
      - KELLY_CALCULATOR_URL=http://kelly-calculator:8080
      - BOT_SERVICE_URL=http://bot-service:8080
      - LOG_LEVEL=info
    depends_on:
      - redis
      - holocron
      - kelly-calculator
      - bot-service
    restart: unless-stopped
    networks:
      - fortuna
```

### 12.2 Service Configuration

```yaml
# services/auto-bettor/config.yaml

redis:
  url: "redis:6379"
  consumer_group: "auto-bettor"
  consumer_name: "auto-bettor-1"

streams:
  opportunities_detected: "opportunities.detected"
  batch_size: 10
  block_time: 5s

databases:
  alexandria:
    host: "alexandria"
    port: 5432
    database: "alexandria"
    user: "fortuna"
    max_connections: 10

  holocron:
    host: "holocron"
    port: 5432
    database: "holocron"
    user: "fortuna"
    max_connections: 20

services:
  kelly_calculator:
    url: "http://kelly-calculator:8080"
    timeout: 5s

  bot_service:
    url: "http://bot-service:8080"
    timeout: 60s

filters:
  enabled:
    - user_preferences
    - risk_management
    - book_health
    - timing
    - correlation

execution:
  max_retries: 3
  retry_backoff: exponential
  circuit_breaker:
    max_failures: 5
    reset_timeout: 5m

logging:
  level: "info"
  format: "json"
  output: "stdout"
```

---

## 13. Testing Strategy

### 13.1 Unit Tests

```go
// services/auto-bettor/internal/filters/user_preferences_test.go

func TestUserPreferencesFilter_EdgeThreshold(t *testing.T) {
    filter := NewUserPreferencesFilter()

    tests := []struct {
        name       string
        oppEdge    float64
        minEdge    float64
        shouldPass bool
    }{
        {"Above threshold", 2.5, 2.0, true},
        {"Exactly at threshold", 2.0, 2.0, true},
        {"Below threshold", 1.5, 2.0, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            opp := Opportunity{EdgePct: tt.oppEdge}
            settings := UserSettings{AutoMinEdgePct: tt.minEdge}

            result := filter.Evaluate(opp, settings, State{})

            if result.Passed != tt.shouldPass {
                t.Errorf("Expected passed=%v, got %v", tt.shouldPass, result.Passed)
            }
        })
    }
}
```

### 13.2 Integration Tests

```go
// services/auto-bettor/test/integration/end_to_end_test.go

func TestEndToEndAutoBetting(t *testing.T) {
    // Setup test environment
    ctx := setupTestEnvironment(t)
    defer ctx.Cleanup()

    // Create test user with auto-betting enabled
    user := createTestUser(t, ctx.DB, UserSettings{
        AutoBettingEnabled:      true,
        AutoMinEdgePct:          1.5,
        AutoEnabledMarkets:      []string{"spreads"},
        AutoMaxStakePerBet:      100.00,
        AutoKellyFraction:       0.25,
    })

    // Publish test opportunity to stream
    opportunity := createTestOpportunity(2.5, "spreads", "fanduel")
    publishToStream(ctx.Redis, "opportunities.detected", opportunity)

    // Wait for processing
    time.Sleep(2 * time.Second)

    // Verify bet was placed
    decisions, err := ctx.DB.GetAutoBettingDecisions(user.ID, 10, 0, "")
    require.NoError(t, err)
    require.Len(t, decisions, 1)

    decision := decisions[0]
    assert.Equal(t, "placed", decision.Decision)
    assert.NotNil(t, decision.BetID)
    assert.Greater(t, decision.CalculatedStake, 0.0)

    // Verify state was updated
    state, err := ctx.StateManager.GetState(user.ID)
    require.NoError(t, err)
    assert.Equal(t, 1, state.BetsPlacedToday)
    assert.Greater(t, state.TotalExposure, 0.0)
}
```

---

## 14. Rollout Plan

### Phase 1: Foundation (Week 1-2)
- [ ] Create database migrations for new tables
- [ ] Implement state management module
- [ ] Build filter framework and first 3 filters (preferences, risk, timing)
- [ ] Unit tests for all filters

### Phase 2: Core Service (Week 3-4)
- [ ] Implement auto-bettor service with stream consumer
- [ ] Integrate with Kelly Calculator
- [ ] Integrate with Bot Service
- [ ] Add decision logging
- [ ] Integration tests

### Phase 3: Safety Features (Week 5)
- [ ] Implement circuit breakers
- [ ] Add rate limiting
- [ ] Build pause/resume functionality
- [ ] Add correlation filter
- [ ] Stress testing

### Phase 4: UI & Monitoring (Week 6-7)
- [ ] Build settings page
- [ ] Build dashboard
- [ ] Add real-time status updates
- [ ] Implement metrics/observability
- [ ] User acceptance testing

### Phase 5: Beta Launch (Week 8)
- [ ] Deploy to staging
- [ ] Enable for beta users
- [ ] Monitor closely for 1 week
- [ ] Gather feedback
- [ ] Fix bugs

### Phase 6: Production (Week 9+)
- [ ] Deploy to production
- [ ] Gradual rollout (10% → 50% → 100%)
- [ ] Continuous monitoring
- [ ] Iterate based on data

---

## 15. Risk Mitigation

### Technical Risks
1. **Stream Processing Lag**: Monitor consumer lag, add horizontal scaling if needed
2. **Database Deadlocks**: Use proper transaction isolation, add timeouts
3. **Bot Failures**: Circuit breakers, comprehensive retry logic
4. **Race Conditions**: Use Redis locks for critical sections (bankroll updates)

### Financial Risks
1. **Over-Betting**: Multiple layers of limits (per-bet, per-event, total)
2. **Bad Data**: Stale data filters, timing checks
3. **Bot Errors**: Confirmation mode initially, gradual trust building
4. **Correlated Bets**: Correlation detection and discounting

### Operational Risks
1. **Runaway Automation**: Manual pause button, loss streak auto-pause
2. **Configuration Errors**: Settings validation, sane defaults
3. **Monitoring Gaps**: Comprehensive logging, alerts on anomalies

---

## 16. Success Metrics

### Performance Metrics
- **Latency**: Opportunity detected → bet placed < 5 seconds (p95)
- **Throughput**: Support 100+ opportunities/minute
- **Uptime**: 99.9% service availability

### Business Metrics
- **Conversion Rate**: % of opportunities that result in bets
- **Edge Capture**: Average edge on placed bets vs. all opportunities
- **Auto vs Manual ROI**: Compare automated vs manual betting performance
- **CLV**: Closing line value on auto-placed bets

### Safety Metrics
- **False Positives**: Opportunities incorrectly filtered (minimize)
- **Exposure Utilization**: Actual vs. allowed exposure
- **Circuit Breaker Activations**: Track frequency and causes

---

## 17. Future Enhancements

### V2 Features
1. **Machine Learning Filters**: Train models to predict +EV opportunities
2. **Dynamic Kelly**: Adjust Kelly fraction based on recent performance
3. **Multi-Leg Optimization**: Smart parlay builder
4. **Hedge Automation**: Auto-hedge when opportunities arise
5. **Arbitrage Mode**: Guaranteed profit scalps only

### V3 Features
1. **Portfolio Optimization**: Kelly across entire portfolio, not per-bet
2. **Predictive Models**: Build own fair price estimates
3. **Live Betting**: In-game auto-betting
4. **API for External Algorithms**: Let users plug in custom strategies

---

## Conclusion

This automated betting system is designed to:

1. ✅ **Integrate Seamlessly** - Uses existing infrastructure (streams, bot service, Kelly calculator)
2. ✅ **Prioritize Safety** - Multiple layers of limits, circuit breakers, manual overrides
3. ✅ **Maximize Performance** - Stream-based, <5s latency, optimized queries
4. ✅ **Enable Customization** - User-specific settings, granular controls
5. ✅ **Provide Visibility** - Comprehensive logging, dashboard, decision audit trail
6. ✅ **Scale Gracefully** - Horizontal scaling via consumer groups, connection pooling
7. ✅ **Handle All Bet Types** - Sophisticated strategy pattern for edge, middle, and scalp execution

### Key Architectural Decisions

**Strategy Pattern for Bet Types**:
- Edge bets: Simple single-leg execution with Kelly sizing
- Middle bets: Coordinated 2-leg execution with independent Kelly per leg, configurable partial fill handling
- Scalp bets: Critical all-or-nothing parallel execution with equal profit distribution

**Execution Priorities**:
- Scalps get highest priority (time-critical, must complete in <30s)
- Middles get medium priority (coordination needed but less time-sensitive)
- Edges get normal priority (single bet, straightforward)

**Rollback & Safety**:
- Edges: No rollback needed (single bet)
- Middles: Optional rollback (accept partial fill if each leg has edge)
- Scalps: **CRITICAL rollback** (partial execution = unhedged risk)

**Database Design**:
- Execution tracking tables for multi-leg coordination
- Bet-type specific settings per user
- Comprehensive audit trail for every decision

The system follows best practices from your existing codebase:
- Service-oriented architecture
- Redis streams for async communication
- Structured logging with request IDs
- Graceful shutdown and error handling
- Comprehensive testing
- Strategy pattern for extensibility

**Implementation Phasing**:
1. **Phase 1 (Weeks 1-2)**: Foundation - Database, state management, filters
2. **Phase 2 (Weeks 3-4)**: Edge bets only - Simplest execution path
3. **Phase 3 (Week 5)**: Middle bets - Add 2-leg coordination
4. **Phase 4 (Weeks 6-7)**: Scalp bets - Critical parallel execution
5. **Phase 5 (Week 8)**: Beta testing with real money (small stakes)
6. **Phase 6 (Week 9+)**: Production rollout

**Next Steps**: Review this design, ask questions about the bet type execution strategies, then I'll help implement starting with:
1. Database migrations (all tables including execution tracking)
2. Execution strategy interfaces and implementations
3. State management module
4. Filter framework