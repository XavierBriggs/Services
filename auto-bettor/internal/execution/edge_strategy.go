package execution

import (
	"fmt"
	"time"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// EdgeStrategy handles single-leg edge bet execution
type EdgeStrategy struct {
	botClient *BotServiceClient
}

// NewEdgeStrategy creates a new edge strategy
func NewEdgeStrategy(botClient *BotServiceClient) *EdgeStrategy {
	return &EdgeStrategy{
		botClient: botClient,
	}
}

// Plan creates an execution plan for an edge opportunity
func (es *EdgeStrategy) Plan(
	opp models.Opportunity,
	settings models.UserSettings,
	state models.AutoBettingState,
	bankroll float64,
	stakes map[string]float64,
) (*models.ExecutionPlan, error) {
	if len(opp.Legs) != 1 {
		return nil, fmt.Errorf("edge opportunity must have exactly 1 leg, got %d", len(opp.Legs))
	}

	leg := opp.Legs[0]
	stake, exists := stakes[leg.BookKey]
	if !exists {
		return nil, fmt.Errorf("no stake calculated for book %s", leg.BookKey)
	}

	legPlan := models.LegPlan{
		LegNumber:        1,
		OpportunityLegID: leg.ID,
		BookKey:          leg.BookKey,
		OutcomeName:      leg.OutcomeName,
		Stake:            stake,
		Price:            leg.Price,
		Point:            leg.Point,
		Priority:         1,
		MaxRetries:       3,
	}

	return &models.ExecutionPlan{
		OpportunityID:   opp.ID,
		OpportunityType: "edge",
		UserID:          "default", // TODO: Support multiple users
		Legs:            []models.LegPlan{legPlan},
		Strategy:        "sequential", // Only one leg
		RequiredLegs:    1,
		TotalStake:      stake,
		Bankroll:        bankroll,
	}, nil
}

// Execute executes the edge bet
func (es *EdgeStrategy) Execute(plan *models.ExecutionPlan) (*models.ExecutionResult, error) {
	result := &models.ExecutionResult{
		LegsPlaced: make([]models.LegResult, 0),
		LegsFailed: make([]models.LegResult, 0),
	}

	startTime := time.Now()

	// Execute the single leg
	legPlan := plan.Legs[0]
	legResult := es.executeLeg(plan.OpportunityID, legPlan)

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

// Rollback handles rollback for edge bets (no rollback needed for single bet)
func (es *EdgeStrategy) Rollback(plan *models.ExecutionPlan, result *models.ExecutionResult) error {
	// Edge bets don't need rollback - single bet either worked or didn't
	return nil
}

// executeLeg executes a single bet leg
func (es *EdgeStrategy) executeLeg(opportunityID int64, legPlan models.LegPlan) models.LegResult {
	startTime := time.Now()

	// Build bot service request
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

	// Call bot service
	botResp, err := es.botClient.PlaceBet(botReq)
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


