package execution

import (
	"fmt"
	"sync"
	"time"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// ScalpStrategy handles multi-leg scalp (arbitrage) execution
type ScalpStrategy struct {
	botClient *BotServiceClient
}

// NewScalpStrategy creates a new scalp strategy
func NewScalpStrategy(botClient *BotServiceClient) *ScalpStrategy {
	return &ScalpStrategy{
		botClient: botClient,
	}
}

// Plan creates an execution plan for a scalp opportunity
func (ss *ScalpStrategy) Plan(
	opp models.Opportunity,
	settings models.UserSettings,
	state models.AutoBettingState,
	bankroll float64,
	stakes map[string]float64,
) (*models.ExecutionPlan, error) {
	if len(opp.Legs) < 2 {
		return nil, fmt.Errorf("scalp opportunity must have at least 2 legs, got %d", len(opp.Legs))
	}

	legs := make([]models.LegPlan, len(opp.Legs))
	totalStake := 0.0

	for i, leg := range opp.Legs {
		stake, exists := stakes[leg.BookKey]
		if !exists {
			return nil, fmt.Errorf("no stake calculated for book %s", leg.BookKey)
		}

		legs[i] = models.LegPlan{
			LegNumber:        i + 1,
			OpportunityLegID: leg.ID,
			BookKey:          leg.BookKey,
			OutcomeName:      leg.OutcomeName,
			Stake:            stake,
			Price:            leg.Price,
			Point:            leg.Point,
			Priority:         1, // All equal priority for scalps
			MaxRetries:       2, // Fewer retries for speed
		}

		totalStake += stake
	}

	return &models.ExecutionPlan{
		OpportunityID:   opp.ID,
		OpportunityType: "scalp",
		UserID:          "default",
		Legs:            legs,
		Strategy:        "parallel", // MUST be parallel for scalps
		RequiredLegs:    len(legs),  // ALL legs required
		TotalStake:      totalStake,
		Bankroll:        bankroll,
	}, nil
}

// Execute executes the scalp bet (all-or-nothing)
func (ss *ScalpStrategy) Execute(plan *models.ExecutionPlan) (*models.ExecutionResult, error) {
	result := &models.ExecutionResult{
		LegsPlaced: make([]models.LegResult, 0),
		LegsFailed: make([]models.LegResult, 0),
	}

	startTime := time.Now()

	// MUST execute in parallel for scalps (time-critical)
	result = ss.executeParallelWithTimeout(
		plan.OpportunityID,
		plan.Legs,
		30*time.Second, // 30 second timeout for scalps
	)

	result.TotalDuration = time.Since(startTime)

	// ALL legs must succeed for scalp
	if len(result.LegsPlaced) == len(plan.Legs) {
		result.Status = "success"
		result.RollbackNeeded = false
	} else {
		result.Status = "failed"
		result.RollbackNeeded = len(result.LegsPlaced) > 0 // Need rollback if any legs placed
	}

	return result, nil
}

// Rollback handles critical partial execution for scalps
func (ss *ScalpStrategy) Rollback(plan *models.ExecutionPlan, result *models.ExecutionResult) error {
	// CRITICAL: Partial scalp is very dangerous - unhedged exposure

	if len(result.LegsPlaced) > 0 && len(result.LegsPlaced) < len(plan.Legs) {
		fmt.Printf("🚨 CRITICAL: Scalp opportunity %d partial execution - placed %d of %d legs\n",
			plan.OpportunityID, len(result.LegsPlaced), len(plan.Legs))

		// Log details of placed and failed legs
		for _, placed := range result.LegsPlaced {
			fmt.Printf("   ✓ Placed: %s $%.2f (bet_id: %d)\n", 
				placed.BookKey, placed.Stake, *placed.BetID)
		}
		for _, failed := range result.LegsFailed {
			fmt.Printf("   ✗ Failed: %s $%.2f (error: %v)\n", 
				failed.BookKey, failed.Stake, failed.Error)
		}

		// TODO: Advanced rollback strategies:
		// 1. Try alternate books for missing legs
		// 2. Place hedge bets to minimize loss
		// 3. Send alert for manual intervention

		return fmt.Errorf("CRITICAL: failed to complete scalp - %d legs placed, %d failed", 
			len(result.LegsPlaced), len(result.LegsFailed))
	}

	return nil
}

// executeParallelWithTimeout executes all legs simultaneously with timeout
func (ss *ScalpStrategy) executeParallelWithTimeout(
	opportunityID int64,
	legs []models.LegPlan,
	timeout time.Duration,
) *models.ExecutionResult {
	result := &models.ExecutionResult{
		LegsPlaced: make([]models.LegResult, 0),
		LegsFailed: make([]models.LegResult, 0),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	resultChan := make(chan models.LegResult, len(legs))
	doneChan := make(chan struct{})

	// Launch all legs in parallel
	for _, leg := range legs {
		wg.Add(1)
		go func(l models.LegPlan) {
			defer wg.Done()
			legResult := ss.executeLeg(opportunityID, l)
			resultChan <- legResult
		}(leg)
	}

	// Wait for all goroutines in separate goroutine
	go func() {
		wg.Wait()
		close(resultChan)
		close(doneChan)
	}()

	// Collect results with timeout
	timeoutChan := time.After(timeout)
	completed := false

	for !completed {
		select {
		case legResult, ok := <-resultChan:
			if !ok {
				completed = true
				break
			}

			mu.Lock()
			if legResult.Status == "success" {
				result.LegsPlaced = append(result.LegsPlaced, legResult)
			} else {
				result.LegsFailed = append(result.LegsFailed, legResult)
			}
			mu.Unlock()

		case <-timeoutChan:
			// Timeout - mark remaining legs as failed
			mu.Lock()
			remainingLegs := len(legs) - (len(result.LegsPlaced) + len(result.LegsFailed))
			for i := 0; i < remainingLegs; i++ {
				result.LegsFailed = append(result.LegsFailed, models.LegResult{
					Status:   "timeout",
					Error:    fmt.Errorf("execution timeout after %v", timeout),
					Duration: timeout,
				})
			}
			mu.Unlock()
			completed = true

		case <-doneChan:
			completed = true
		}
	}

	return result
}

// executeLeg executes a single bet leg (optimized for speed)
func (ss *ScalpStrategy) executeLeg(opportunityID int64, legPlan models.LegPlan) models.LegResult {
	startTime := time.Now()

	botReq := PlaceBetRequest{
		OpportunityID: opportunityID,
		Legs: []BotServiceLeg{
			{
				BookKey:      legPlan.BookKey,
				OutcomeName:  legPlan.OutcomeName,
				Stake:        legPlan.Stake,
				ExpectedOdds: legPlan.Price,
			},
		},
	}

	botResp, err := ss.botClient.PlaceBet(botReq)
	duration := time.Since(startTime)

	if err != nil {
		return models.LegResult{
			LegNumber: legPlan.LegNumber,
			BookKey:   legPlan.BookKey,
			Stake:     legPlan.Stake,
			Status:    "failed",
			Error:     err,
			Duration:  duration,
		}
	}

	if !botResp.Success || len(botResp.Results) == 0 {
		return models.LegResult{
			LegNumber: legPlan.LegNumber,
			BookKey:   legPlan.BookKey,
			Stake:     legPlan.Stake,
			Status:    "failed",
			Error:     fmt.Errorf("bot service returned failure"),
			Duration:  duration,
		}
	}

	legResult := botResp.Results[0]
	if legResult.Success {
		return models.LegResult{
			LegNumber: legPlan.LegNumber,
			BetID:     legResult.BetID,
			BookKey:   legPlan.BookKey,
			Stake:     legPlan.Stake,
			Status:    "success",
			Duration:  duration,
		}
	}

	return models.LegResult{
		LegNumber: legPlan.LegNumber,
		BookKey:   legPlan.BookKey,
		Stake:     legPlan.Stake,
		Status:    "failed",
		Error:     fmt.Errorf("%s", legResult.Error),
		Duration:  duration,
	}
}


