# Bet Type Execution Strategy - Edge, Middle, Scalp

## Overview

This document defines how the automated betting system should handle different opportunity types. Each type has unique characteristics and requires different execution strategies.

---

## 1. Opportunity Type Characteristics

### 1.1 EDGE - Single Positive EV Bet

**Definition**: One side of a market offers positive expected value based on our fair price calculation.

**Structure**:
- **1 leg** (single bet)
- One book, one market, one outcome
- EV comes from book offering better odds than fair price

**Example**:
```
Game: Lakers vs Celtics
Market: Spread
Book: FanDuel
Fair Price: Lakers -5.5 @ 50% (true odds: 2.00)
FanDuel Odds: Lakers -5.5 @ -105 (1.952 decimal)
Book Implied: 51.2%
Edge: (50% / 51.2%) - 1 = -2.3% ❌ (not an edge)

Actually an edge:
Fair Price: Lakers -5.5 @ 52% (true odds: 1.923)
FanDuel Odds: Lakers -5.5 @ +105 (2.05 decimal)
Book Implied: 48.8%
Edge: (52% / 48.8%) - 1 = +6.6% ✅
```

**Kelly Sizing**:
- Standard Kelly formula: `Kelly% = (bp - q) / b`
- Where: b = net odds, p = win probability, q = 1-p
- Apply fractional Kelly (e.g., 0.25 for quarter Kelly)

**Execution Strategy**:
- Place single bet
- Simple, straightforward
- Can be placed immediately

---

### 1.2 MIDDLE - Dual Positive EV with Win-Both Potential

**Definition**: Both sides of the same market have positive EV, creating potential to win both bets if result lands in the "middle."

**Structure**:
- **2 legs** (two separate bets)
- Usually different books
- Same game, same market, opposite sides
- Each leg has independent positive EV

**Example**:
```
Game: Lakers vs Celtics
Market: Spread

Leg 1 (Book A - DraftKings):
  Lakers -4.5 @ +105 (2.05 decimal)
  Fair price: Lakers -4.5 @ 48% (true odds: 2.083)
  Edge: (48% / 48.8%) - 1 = -1.6% (wait, let me recalc)

Actually:
Leg 1 (DraftKings):
  Lakers -4.5 @ +110 (2.10 decimal, implied 47.6%)
  Fair probability: 50%
  Edge: (50% / 47.6%) - 1 = +5.0% ✅

Leg 2 (FanDuel):
  Celtics +6.5 @ -105 (1.952 decimal, implied 51.2%)
  Fair probability: 54%
  Edge: (54% / 51.2%) - 1 = +5.5% ✅

Middle Window: If Lakers win by 5 or 6, BOTH bets win
```

**Key Characteristics**:
- Each leg is profitable on its own
- Middle window (Lakers win by 5-6) = win both bets
- Outside middle = win one, lose one (but still profit due to edges)
- Lower variance than single edges

**Kelly Sizing**:
- **Treat each leg independently** with full Kelly calculation
- Each bet has its own edge, so size separately
- Common approach: Size each leg as if it's a standalone edge bet
- Alternative: Reduce size slightly due to correlation (same game)

**Execution Strategy**:
- **Simultaneous placement critical** - odds can move
- Place both legs as close together as possible (parallel execution)
- If one leg fails, decision on second leg:
  - **Option A**: Cancel second leg (no longer a middle)
  - **Option B**: Place second leg if it still has edge (becomes standalone edge bet)
  - **Recommended**: Make this configurable per user

**Execution Order**:
- Place leg with higher edge first (more important to lock in)
- OR place leg at softer book first (less likely to limit you)
- OR place both simultaneously if using multiple bot instances - Multiple bot instances 

---

### 1.3 SCALP - Guaranteed Profit Arbitrage

**Definition**: Bet all outcomes of a market such that you profit regardless of result. Pure arbitrage with no risk.

**Structure**:
- **2+ legs** (depends on market)
- Different books (must be, or same book wouldn't offer arb)
- All possible outcomes covered
- No edge calculation needed - guaranteed profit

**Example 1 - Two-Way Market (Spread/Total)**:
```
Game: Lakers vs Celtics
Market: Total

Leg 1 (FanDuel):
  Over 220.5 @ +105 (2.05 decimal, implied 48.8%)

Leg 2 (DraftKings):
  Under 220.5 @ +105 (2.05 decimal, implied 48.8%)

Total Implied: 48.8% + 48.8% = 97.6%
Arb %: 100% - 97.6% = 2.4% profit guaranteed ✅
```

**Example 2 - Three-Way Market (Moneyline with Draw)**:
```
Soccer: Man City vs Arsenal
Market: Match Result (1X2)

Leg 1 (Bet365): Man City @ 2.10 (implied 47.6%)
Leg 2 (FanDuel): Draw @ 3.80 (implied 26.3%)
Leg 3 (DraftKings): Arsenal @ 3.60 (implied 27.8%)

Total Implied: 47.6% + 26.3% + 27.8% = 101.7%
No arb (overround) ❌

Better example:
Leg 1: Man City @ 2.20 (implied 45.5%)
Leg 2: Draw @ 4.00 (implied 25.0%)
Leg 3: Arsenal @ 3.80 (implied 26.3%)
Total Implied: 96.8%
Arb %: 3.2% profit ✅
```

**Stake Distribution**:
- NOT Kelly Criterion - different math
- Goal: Equal profit on all outcomes
- Formula for each leg: `Stake_i = (Total_Stake / Decimal_Odds_i) / Sum(1/Decimal_Odds_j)`

**Example Calculation**:
```
Total bankroll to deploy: $1,000
Over 220.5 @ 2.05
Under 220.5 @ 2.05

Stake_Over = $1,000 / (1/2.05 + 1/2.05) × (1/2.05)
           = $1,000 / 0.976 × 0.488
           = $500

Stake_Under = $1,000 / 0.976 × 0.488
            = $500

If Over hits: $500 × 2.05 = $1,025 (profit $25)
If Under hits: $500 × 2.05 = $1,025 (profit $25)
ROI: 2.5% guaranteed
```

**Execution Strategy**:
- **MUST place all legs** - placing only some legs creates risk
- **Simultaneous execution critical** - odds move fast on arbs
- **Transaction must be atomic**:
  - All legs succeed → complete the scalp
  - Any leg fails → cancel/hedge remaining legs immediately
- **Speed is everything** - arbs disappear in seconds
- **Higher priority than edges/middles** in execution queue

**Risk Management**:
- Limit exposure to any single scalp (what if you can't get all legs down?)
- Have contingency plan if a leg fails mid-execution:
  - Try to place at next best book
  - Accept worse odds if still profitable
  - Cancel entire arb if can't complete profitably
  - Manual intervention alert

---

## 2. Database Schema Considerations

### 2.1 Opportunity Legs Table (Already Exists)

Looking at the existing schema:
```sql
CREATE TABLE opportunity_legs (
  id BIGSERIAL PRIMARY KEY,
  opportunity_id BIGINT REFERENCES opportunities(id),
  book_key VARCHAR(50),
  outcome_name VARCHAR(100),
  price DECIMAL(8,3),
  point DECIMAL(8,3),
  leg_edge_pct DECIMAL(8,4),
  -- ... other fields
);
```

This already supports multi-leg opportunities. Good!

### 2.2 Auto Betting Execution Tracking

We need to track execution status per leg:

```sql
CREATE TABLE auto_bet_execution_tracking (
  id BIGSERIAL PRIMARY KEY,
  auto_decision_id BIGINT REFERENCES auto_betting_decisions(id),
  opportunity_id BIGINT REFERENCES opportunities(id),
  opportunity_type VARCHAR(20), -- 'edge', 'middle', 'scalp'

  -- Execution plan
  total_legs INTEGER,
  legs_required_for_completion INTEGER, -- edge: 1, middle: 2, scalp: all
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

  created_at TIMESTAMPTZ DEFAULT NOW()
);

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

  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## 3. Execution Engine Design

### 3.1 Abstract Execution Strategy Pattern

```go
type ExecutionStrategy interface {
    // Plan how to execute this opportunity
    Plan(opp Opportunity, settings UserSettings) (*ExecutionPlan, error)

    // Execute the plan
    Execute(plan *ExecutionPlan) (*ExecutionResult, error)

    // Handle partial execution failure
    Rollback(plan *ExecutionPlan, result *ExecutionResult) error
}

type ExecutionPlan struct {
    OpportunityID   int64
    OpportunityType string
    Legs            []LegPlan
    Strategy        string // "sequential", "parallel", "parallel_with_fallback"
    RequiredLegs    int    // How many legs must succeed
    TotalStake      float64
}

type LegPlan struct {
    LegNumber       int
    OpportunityLegID int64
    BookKey         string
    OutcomeName     string
    Stake           float64
    Priority        int // For ordered execution
    MaxRetries      int
}

type ExecutionResult struct {
    Status          string // "success", "partial", "failed"
    LegsPlaced      []LegResult
    LegsFailed      []LegResult
    TotalDuration   time.Duration
    RollbackNeeded  bool
}

type LegResult struct {
    LegNumber   int
    BetID       int64
    Status      string
    Error       error
    Duration    time.Duration
}
```

### 3.2 Edge Strategy Implementation

```go
type EdgeStrategy struct {
    botService  *BotServiceClient
    kellyCalc   *KellyCalculatorClient
}

func (s *EdgeStrategy) Plan(opp Opportunity, settings UserSettings) (*ExecutionPlan, error) {
    if len(opp.Legs) != 1 {
        return nil, fmt.Errorf("edge opportunity must have exactly 1 leg, got %d", len(opp.Legs))
    }

    leg := opp.Legs[0]

    // Calculate stake using Kelly
    bankroll := getBankroll(settings, leg.BookKey)
    stake := s.kellyCalc.CalculateKelly(
        leg.LegEdgePct,
        leg.Price,
        bankroll,
        settings.AutoKellyFraction,
    )

    // Apply limits
    stake = applyLimits(stake, settings)

    return &ExecutionPlan{
        OpportunityID:   opp.ID,
        OpportunityType: "edge",
        Legs: []LegPlan{
            {
                LegNumber:        1,
                OpportunityLegID: leg.ID,
                BookKey:          leg.BookKey,
                OutcomeName:      leg.OutcomeName,
                Stake:            stake,
                Priority:         1,
                MaxRetries:       3,
            },
        },
        Strategy:     "sequential", // Only one leg, so sequential is fine
        RequiredLegs: 1,
        TotalStake:   stake,
    }, nil
}

func (s *EdgeStrategy) Execute(plan *ExecutionPlan) (*ExecutionResult, error) {
    result := &ExecutionResult{
        LegsPlaced: make([]LegResult, 0),
        LegsFailed: make([]LegResult, 0),
    }

    startTime := time.Now()

    // Execute single leg
    legPlan := plan.Legs[0]
    legResult := s.executeLeg(legPlan)

    if legResult.Status == "success" {
        result.LegsPlaced = append(result.LegsPlaced, legResult)
        result.Status = "success"
    } else {
        result.LegsFailed = append(result.LegsFailed, legResult)
        result.Status = "failed"
    }

    result.TotalDuration = time.Since(startTime)
    return result, nil
}

func (s *EdgeStrategy) Rollback(plan *ExecutionPlan, result *ExecutionResult) error {
    // Edges don't need rollback - single bet either worked or didn't
    return nil
}
```

### 3.3 Middle Strategy Implementation

```go
type MiddleStrategy struct {
    botService  *BotServiceClient
    kellyCalc   *KellyCalculatorClient
}

func (s *MiddleStrategy) Plan(opp Opportunity, settings UserSettings) (*ExecutionPlan, error) {
    if len(opp.Legs) != 2 {
        return nil, fmt.Errorf("middle opportunity must have exactly 2 legs, got %d", len(opp.Legs))
    }

    legs := make([]LegPlan, 2)
    totalStake := 0.0

    // Size each leg independently using Kelly
    for i, leg := range opp.Legs {
        bankroll := getBankroll(settings, leg.BookKey)

        // Calculate independent Kelly for this leg
        stake := s.kellyCalc.CalculateKelly(
            leg.LegEdgePct,
            leg.Price,
            bankroll,
            settings.AutoKellyFraction,
        )

        // Optional: Apply correlation discount
        if settings.AutoCorrelationDiscount > 0 {
            stake = stake * (1 - settings.AutoCorrelationDiscount)
        }

        stake = applyLimits(stake, settings)

        legs[i] = LegPlan{
            LegNumber:        i + 1,
            OpportunityLegID: leg.ID,
            BookKey:          leg.BookKey,
            OutcomeName:      leg.OutcomeName,
            Stake:            stake,
            Priority:         determinePriority(leg, opp.Legs), // Higher edge = higher priority
            MaxRetries:       3,
        }

        totalStake += stake
    }

    // Determine execution strategy
    strategy := "parallel" // Default: try to place both at once
    if settings.AutoMiddleExecutionStrategy == "sequential" {
        strategy = "sequential_with_fallback"
    }

    return &ExecutionPlan{
        OpportunityID:   opp.ID,
        OpportunityType: "middle",
        Legs:            legs,
        Strategy:        strategy,
        RequiredLegs:    settings.AutoMiddleRequiredLegs, // Config: 1 or 2
        TotalStake:      totalStake,
    }, nil
}

func (s *MiddleStrategy) Execute(plan *ExecutionPlan) (*ExecutionResult, error) {
    result := &ExecutionResult{
        LegsPlaced: make([]LegResult, 0),
        LegsFailed: make([]LegResult, 0),
    }

    startTime := time.Now()

    if plan.Strategy == "parallel" {
        // Execute both legs simultaneously
        result = s.executeParallel(plan.Legs)
    } else {
        // Execute sequentially (higher priority first)
        result = s.executeSequential(plan.Legs)
    }

    result.TotalDuration = time.Since(startTime)

    // Determine overall status
    if len(result.LegsPlaced) == 2 {
        result.Status = "success" // Got the full middle
    } else if len(result.LegsPlaced) == 1 && plan.RequiredLegs == 1 {
        result.Status = "partial" // Acceptable partial fill
    } else {
        result.Status = "failed"
        result.RollbackNeeded = true // Need to cancel the leg we did place
    }

    return result, nil
}

func (s *MiddleStrategy) executeParallel(legs []LegPlan) *ExecutionResult {
    result := &ExecutionResult{}

    // Use goroutines to place both legs simultaneously
    resultChan := make(chan LegResult, len(legs))

    for _, leg := range legs {
        go func(l LegPlan) {
            resultChan <- s.executeLeg(l)
        }(leg)
    }

    // Collect results
    for i := 0; i < len(legs); i++ {
        legResult := <-resultChan
        if legResult.Status == "success" {
            result.LegsPlaced = append(result.LegsPlaced, legResult)
        } else {
            result.LegsFailed = append(result.LegsFailed, legResult)
        }
    }

    return result
}

func (s *MiddleStrategy) executeSequential(legs []LegPlan) *ExecutionResult {
    result := &ExecutionResult{}

    // Sort by priority
    sort.Slice(legs, func(i, j int) bool {
        return legs[i].Priority > legs[j].Priority
    })

    // Execute in order
    for _, leg := range legs {
        legResult := s.executeLeg(leg)

        if legResult.Status == "success" {
            result.LegsPlaced = append(result.LegsPlaced, legResult)
        } else {
            result.LegsFailed = append(result.LegsFailed, legResult)
            // Continue to next leg (user config determines if we stop or continue)
        }
    }

    return result
}

func (s *MiddleStrategy) Rollback(plan *ExecutionPlan, result *ExecutionResult) error {
    // If we got partial fill but need full middle, need to hedge/cancel

    if len(result.LegsPlaced) == 1 && plan.RequiredLegs == 2 {
        // We placed one leg but wanted both
        // Options:
        // 1. Try to cancel the bet (if book allows)
        // 2. Place a hedge bet on the other side to minimize loss
        // 3. Accept the single leg as an edge bet (if it has edge)

        placedLeg := result.LegsPlaced[0]

        // Log the partial execution
        log.Warnf("Middle opportunity %d only placed 1 of 2 legs (bet_id: %d). Leaving as edge bet.",
            plan.OpportunityID, placedLeg.BetID)

        // Update opportunity status to "partial_middle" or similar
        // This is acceptable - we still have an edge bet
    }

    return nil
}
```

### 3.4 Scalp Strategy Implementation

```go
type ScalpStrategy struct {
    botService *BotServiceClient
}

func (s *ScalpStrategy) Plan(opp Opportunity, settings UserSettings) (*ExecutionPlan, error) {
    if len(opp.Legs) < 2 {
        return nil, fmt.Errorf("scalp opportunity must have at least 2 legs, got %d", len(opp.Legs))
    }

    // Determine total stake to deploy
    // For scalps, we don't use Kelly - we use a fixed % of bankroll
    totalBankroll := getTotalBankroll(settings)
    totalStake := totalBankroll * settings.AutoScalpBankrollPct // e.g., 0.05 = 5%

    // Cap at max exposure settings
    if totalStake > settings.AutoMaxStakePerBet {
        totalStake = settings.AutoMaxStakePerBet
    }

    // Calculate stake distribution for equal profit on all outcomes
    stakes := s.calculateScalpStakes(opp.Legs, totalStake)

    legs := make([]LegPlan, len(opp.Legs))
    for i, leg := range opp.Legs {
        legs[i] = LegPlan{
            LegNumber:        i + 1,
            OpportunityLegID: leg.ID,
            BookKey:          leg.BookKey,
            OutcomeName:      leg.OutcomeName,
            Stake:            stakes[i],
            Priority:         1, // All equal priority for scalps
            MaxRetries:       2, // Fewer retries - speed matters
        }
    }

    return &ExecutionPlan{
        OpportunityID:   opp.ID,
        OpportunityType: "scalp",
        Legs:            legs,
        Strategy:        "parallel", // MUST be parallel for scalps
        RequiredLegs:    len(legs),  // ALL legs required
        TotalStake:      totalStake,
    }, nil
}

func (s *ScalpStrategy) calculateScalpStakes(legs []OpportunityLeg, totalStake float64) []float64 {
    // Formula: Stake_i = (Total / Sum(1/Odds_j)) * (1/Odds_i)

    sumInverseOdds := 0.0
    for _, leg := range legs {
        sumInverseOdds += 1.0 / leg.Price
    }

    stakes := make([]float64, len(legs))
    for i, leg := range legs {
        stakes[i] = (totalStake / sumInverseOdds) * (1.0 / leg.Price)
        stakes[i] = math.Round(stakes[i]*100) / 100 // Round to cents
    }

    return stakes
}

func (s *ScalpStrategy) Execute(plan *ExecutionPlan) (*ExecutionResult, error) {
    result := &ExecutionResult{
        LegsPlaced: make([]LegResult, 0),
        LegsFailed: make([]LegResult, 0),
    }

    startTime := time.Now()

    // MUST execute in parallel for scalps
    resultChan := make(chan LegResult, len(plan.Legs))

    for _, leg := range plan.Legs {
        go func(l LegPlan) {
            resultChan <- s.executeLegFast(l) // Use faster execution with fewer retries
        }(leg)
    }

    // Collect results with timeout
    timeout := time.After(30 * time.Second) // Scalps must complete fast

    for i := 0; i < len(plan.Legs); i++ {
        select {
        case legResult := <-resultChan:
            if legResult.Status == "success" {
                result.LegsPlaced = append(result.LegsPlaced, legResult)
            } else {
                result.LegsFailed = append(result.LegsFailed, legResult)
            }
        case <-timeout:
            result.LegsFailed = append(result.LegsFailed, LegResult{
                Status: "timeout",
                Error:  fmt.Errorf("execution timeout"),
            })
        }
    }

    result.TotalDuration = time.Since(startTime)

    // ALL legs must succeed for scalp
    if len(result.LegsPlaced) == len(plan.Legs) {
        result.Status = "success"
    } else {
        result.Status = "failed"
        result.RollbackNeeded = true // CRITICAL: must rollback partial scalps
    }

    return result, nil
}

func (s *ScalpStrategy) Rollback(plan *ExecutionPlan, result *ExecutionResult) error {
    // CRITICAL: If we didn't get all legs, we have risk exposure

    if len(result.LegsPlaced) > 0 && len(result.LegsPlaced) < len(plan.Legs) {
        // Partial scalp is VERY DANGEROUS

        log.Errorf("CRITICAL: Scalp opportunity %d partial execution - placed %d of %d legs",
            plan.OpportunityID, len(result.LegsPlaced), len(plan.Legs))

        // Options (in order of preference):
        // 1. Try to place missing legs at slightly worse odds if still profitable
        // 2. Place hedge bets to minimize loss
        // 3. Alert human immediately for manual intervention
        // 4. Try to cancel placed bets (if book allows)

        // For now: Alert and accept the risk
        s.alertCriticalFailure(plan, result)

        // Try to find alternate books to complete the scalp
        for _, failedLeg := range result.LegsFailed {
            alternateResult := s.tryAlternateBooks(failedLeg, plan)
            if alternateResult.Status == "success" {
                result.LegsPlaced = append(result.LegsPlaced, alternateResult)
            }
        }

        // If still not complete, we have exposure
        if len(result.LegsPlaced) < len(plan.Legs) {
            return fmt.Errorf("failed to complete scalp - partial execution creates risk")
        }
    }

    return nil
}
```

---

## 4. Configuration Settings

Add to user settings:

```sql
ALTER TABLE user_settings
  -- Middle-specific settings
  ADD COLUMN auto_middle_execution_strategy VARCHAR(20) DEFAULT 'parallel', -- 'parallel' or 'sequential'
  ADD COLUMN auto_middle_required_legs INTEGER DEFAULT 2, -- 1 = accept single leg, 2 = need both
  ADD COLUMN auto_middle_max_time_between_legs_sec INTEGER DEFAULT 10, -- Max delay between legs

  -- Scalp-specific settings
  ADD COLUMN auto_scalp_enabled BOOLEAN DEFAULT FALSE, -- Separate toggle for arbs
  ADD COLUMN auto_scalp_bankroll_pct DECIMAL(5,2) DEFAULT 5.00, -- % of bankroll per scalp
  ADD COLUMN auto_scalp_min_profit_pct DECIMAL(5,2) DEFAULT 0.5, -- Minimum arb percentage
  ADD COLUMN auto_scalp_execution_timeout_sec INTEGER DEFAULT 30, -- Must complete fast

  -- Edge-specific settings
  ADD COLUMN auto_edge_allow_live_games BOOLEAN DEFAULT FALSE; -- Bet on live games?
```

---

## 5. Execution Priority Queue

When multiple opportunities arrive simultaneously:

```
Priority 1: SCALP (time-sensitive, need all legs fast)
Priority 2: MIDDLE (time-sensitive, 2 legs to coordinate)
Priority 3: EDGE (single leg, less time pressure)
```

Implementation:

```go
type OpportunityQueue struct {
    scalps  chan Opportunity
    middles chan Opportunity
    edges   chan Opportunity
}

func (q *OpportunityQueue) Process() {
    for {
        select {
        case scalp := <-q.scalps:
            // Process scalp immediately
            go q.processScalp(scalp)

        case middle := <-q.middles:
            // Process middle (but scalps take precedence)
            go q.processMiddle(middle)

        case edge := <-q.edges:
            // Process edge (lowest priority)
            go q.processEdge(edge)

        case <-time.After(1 * time.Second):
            // Idle
        }
    }
}
```

---

## 6. User Interface Considerations

### Dashboard View

```
+---------------------------------------------------+
|  Auto-Betting Activity                            |
+---------------------------------------------------+
|  EDGE BETS        │  MIDDLES       │  SCALPS      |
|  Placed: 45       │  Placed: 12    │  Placed: 3   |
|  Win Rate: 54%    │  Win Rate: 67% │  Success: 3  |
|  ROI: +3.2%       │  ROI: +4.1%    │  ROI: +2.1%  |
+---------------------------------------------------+

Recent Executions:
┌────────────┬──────┬────────┬─────────┬──────────┐
│ Time       │ Type │ Status │ Legs    │ Result   │
├────────────┼──────┼────────┼─────────┼──────────┤
│ 10:45:32   │ EDGE │ ✓      │ 1/1     │ Pending  │
│ 10:44:18   │ MID  │ ✓      │ 2/2     │ Pending  │
│ 10:43:05   │ SCALP│ ✓      │ 3/3     │ +$12.50  │
│ 10:42:41   │ MID  │ ⚠      │ 1/2     │ Partial  │
│ 10:41:22   │ SCALP│ ✗      │ 2/3     │ Failed   │
└────────────┴──────┴────────┴─────────┴──────────┘
```

### Settings Toggles

```
Auto-Betting Configuration
├─ [✓] Enable Edge Bets
│  ├─ Min Edge: 2.0%
│  ├─ Kelly Fraction: 0.25
│  └─ Max Stake: $100
│
├─ [✓] Enable Middles
│  ├─ Execution: ○ Parallel  ● Sequential
│  ├─ Required Legs: ○ Both  ● At Least One
│  └─ Correlation Discount: 50%
│
└─ [✗] Enable Scalps (Arbitrage)
   ├─ Bankroll %: 5.0%
   ├─ Min Profit %: 0.5%
   └─ Timeout: 30 seconds
```

---

## 7. Testing Strategy

### Unit Tests

```go
func TestScalpStakeCalculation(t *testing.T) {
    legs := []OpportunityLeg{
        {Price: 2.05}, // Over
        {Price: 2.05}, // Under
    }

    totalStake := 1000.0

    stakes := calculateScalpStakes(legs, totalStake)

    // Verify equal profit on both outcomes
    profitIfOver := stakes[0] * 2.05 - totalStake
    profitIfUnder := stakes[1] * 2.05 - totalStake

    assert.InDelta(t, profitIfOver, profitIfUnder, 0.01)
}

func TestMiddlePartialExecution(t *testing.T) {
    strategy := &MiddleStrategy{}

    plan := &ExecutionPlan{
        RequiredLegs: 1, // Accept single leg
        Legs: []LegPlan{
            {LegNumber: 1, Stake: 50.0},
            {LegNumber: 2, Stake: 50.0},
        },
    }

    // Simulate partial execution
    result := &ExecutionResult{
        LegsPlaced: []LegResult{{LegNumber: 1, Status: "success"}},
        LegsFailed: []LegResult{{LegNumber: 2, Status: "failed"}},
    }

    err := strategy.Rollback(plan, result)
    assert.NoError(t, err) // Should accept partial
}
```

---

## 8. Monitoring & Alerts

### Critical Alerts

1. **Scalp Partial Execution**: Immediate Slack/email alert
2. **Middle Leg Failure**: Warning if second leg fails
3. **Execution Timeout**: Alert if bets taking >30s
4. **Bot Failures**: Alert if specific book repeatedly fails

### Metrics

```
auto_bettor_execution_success_rate{type="edge"}
auto_bettor_execution_success_rate{type="middle"}
auto_bettor_execution_success_rate{type="scalp"}

auto_bettor_execution_duration{type="edge",percentile="p95"}
auto_bettor_execution_duration{type="middle",percentile="p95"}
auto_bettor_execution_duration{type="scalp",percentile="p95"}

auto_bettor_partial_executions_total{type="middle"}
auto_bettor_partial_executions_total{type="scalp"}

auto_bettor_rollback_required_total{type="scalp"}
```

---

## 9. Summary

### Key Differences

| Aspect | EDGE | MIDDLE | SCALP |
|--------|------|--------|-------|
| Legs | 1 | 2 | 2+ |
| Sizing | Kelly | Independent Kelly per leg | Equal profit distribution |
| Execution | Simple | Parallel preferred | MUST be parallel |
| Failure Tolerance | N/A | Accept 1 leg (configurable) | Zero tolerance |
| Rollback | None needed | Optional (accept partial) | Critical (must complete or cancel) |
| Time Sensitivity | Low | Medium | Very High |
| Priority | Low | Medium | High |

### Risk Levels

- **Edge**: Low (single bet, sized properly)
- **Middle**: Low-Medium (correlated but diversified)
- **Scalp**: HIGH if partial execution (unhedged exposure)

### Recommendations

1. **Start with Edges Only**: Safest, easiest to implement
2. **Add Middles**: Once edge execution is stable
3. **Add Scalps Last**: Requires most sophisticated execution logic
4. **Always Allow Manual Override**: For any bet type
5. **Comprehensive Logging**: Every decision, every leg, every outcome
