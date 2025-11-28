package execution

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// MiddleStrategy handles 2-leg middle bet execution
type MiddleStrategy struct {
	botClient *BotServiceClient
}

// NewMiddleStrategy creates a new middle strategy
func NewMiddleStrategy(botClient *BotServiceClient) *MiddleStrategy {
	return &MiddleStrategy{
		botClient: botClient,
	}
}

// Plan creates an execution plan for a middle opportunity
func (ms *MiddleStrategy) Plan(
	opp models.Opportunity,
	settings models.UserSettings,
	state models.AutoBettingState,
	bankroll float64,
	stakes map[string]float64,
) (*models.ExecutionPlan, error) {
	if len(opp.Legs) != 2 {
		return nil, fmt.Errorf("middle opportunity must have exactly 2 legs, got %d", len(opp.Legs))
	}

	legs := make([]models.LegPlan, 2)
	totalStake := 0.0

	// Create leg plans with priority based on edge
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
			Priority:         ms.calculatePriority(leg),
			MaxRetries:       3,
		}

		totalStake += stake
	}

	// Determine execution strategy from settings
	strategy := "parallel"
	if settings.AutoMiddleExecutionStrategy == "sequential" {
		strategy = "sequential"
	}

	return &models.ExecutionPlan{
		OpportunityID:   opp.ID,
		OpportunityType: "middle",
		UserID:          "default",
		Legs:            legs,
		Strategy:        strategy,
		RequiredLegs:    settings.AutoMiddleRequiredLegs, // 1 or 2
		TotalStake:      totalStake,
		Bankroll:        bankroll,
	}, nil
}

// Execute executes the middle bet
func (ms *MiddleStrategy) Execute(plan *models.ExecutionPlan) (*models.ExecutionResult, error) {
	result := &models.ExecutionResult{
		LegsPlaced: make([]models.LegResult, 0),
		LegsFailed: make([]models.LegResult, 0),
	}

	startTime := time.Now()

	if plan.Strategy == "parallel" {
		result = ms.executeParallel(plan.OpportunityID, plan.Legs)
	} else {
		result = ms.executeSequential(plan.OpportunityID, plan.Legs)
	}

	result.TotalDuration = time.Since(startTime)

	// Determine overall status
	if len(result.LegsPlaced) == 2 {
		result.Status = "success" // Got the full middle
	} else if len(result.LegsPlaced) == 1 && plan.RequiredLegs == 1 {
		result.Status = "partial" // Acceptable partial fill
		result.RollbackNeeded = false
	} else if len(result.LegsPlaced) == 1 && plan.RequiredLegs == 2 {
		result.Status = "partial"
		result.RollbackNeeded = true // Need both legs
	} else {
		result.Status = "failed"
		result.RollbackNeeded = len(result.LegsPlaced) > 0
	}

	return result, nil
}

// Rollback handles partial execution
func (ms *MiddleStrategy) Rollback(plan *models.ExecutionPlan, result *models.ExecutionResult) error {
	// For middles, partial execution is often acceptable since each leg has edge
	if len(result.LegsPlaced) == 1 && plan.RequiredLegs == 1 {
		// Acceptable partial - log it
		fmt.Printf("Middle opportunity %d placed 1 of 2 legs (bet_id: %d). Accepting as edge bet.\n",
			plan.OpportunityID, *result.LegsPlaced[0].BetID)
		return nil
	}

	if len(result.LegsPlaced) == 1 && plan.RequiredLegs == 2 {
		// Not acceptable - but we can't cancel already-placed bets
		// Log warning
		fmt.Printf("WARNING: Middle opportunity %d only placed 1 of 2 required legs (bet_id: %d).\n",
			plan.OpportunityID, *result.LegsPlaced[0].BetID)
		return fmt.Errorf("failed to place all required legs for middle")
	}

	return nil
}

// executeParallel executes both legs simultaneously
func (ms *MiddleStrategy) executeParallel(opportunityID int64, legs []models.LegPlan) *models.ExecutionResult {
	result := &models.ExecutionResult{
		LegsPlaced: make([]models.LegResult, 0),
		LegsFailed: make([]models.LegResult, 0),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	resultChan := make(chan models.LegResult, len(legs))

	for _, leg := range legs {
		wg.Add(1)
		go func(l models.LegPlan) {
			defer wg.Done()
			legResult := ms.executeLeg(opportunityID, l)
			resultChan <- legResult
		}(leg)
	}

	// Wait for all goroutines
	wg.Wait()
	close(resultChan)

	// Collect results
	for legResult := range resultChan {
		mu.Lock()
		if legResult.Status == "success" {
			result.LegsPlaced = append(result.LegsPlaced, legResult)
		} else {
			result.LegsFailed = append(result.LegsFailed, legResult)
		}
		mu.Unlock()
	}

	return result
}

// executeSequential executes legs one at a time by priority
func (ms *MiddleStrategy) executeSequential(opportunityID int64, legs []models.LegPlan) *models.ExecutionResult {
	result := &models.ExecutionResult{
		LegsPlaced: make([]models.LegResult, 0),
		LegsFailed: make([]models.LegResult, 0),
	}

	// Sort by priority (highest first)
	sortedLegs := make([]models.LegPlan, len(legs))
	copy(sortedLegs, legs)
	sort.Slice(sortedLegs, func(i, j int) bool {
		return sortedLegs[i].Priority > sortedLegs[j].Priority
	})

	// Execute in order
	for _, leg := range sortedLegs {
		legResult := ms.executeLeg(opportunityID, leg)

		if legResult.Status == "success" {
			result.LegsPlaced = append(result.LegsPlaced, legResult)
		} else {
			result.LegsFailed = append(result.LegsFailed, legResult)
			// Continue to try other legs even if one fails
		}
	}

	return result
}

// executeLeg executes a single bet leg
func (ms *MiddleStrategy) executeLeg(opportunityID int64, legPlan models.LegPlan) models.LegResult {
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

	botResp, err := ms.botClient.PlaceBet(botReq)
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

// calculatePriority calculates priority for a leg (higher edge = higher priority)
func (ms *MiddleStrategy) calculatePriority(leg models.OpportunityLeg) int {
	// Priority based on edge (scaled to int)
	return int(leg.LegEdgePct * 100)
}


