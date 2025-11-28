package execution

import (
	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// Strategy defines the interface for execution strategies
type Strategy interface {
	// Plan creates an execution plan for this opportunity
	Plan(
		opp models.Opportunity,
		settings models.UserSettings,
		state models.AutoBettingState,
		bankroll float64,
		stakes map[string]float64,
	) (*models.ExecutionPlan, error)

	// Execute carries out the execution plan
	Execute(plan *models.ExecutionPlan) (*models.ExecutionResult, error)

	// Rollback handles partial execution failures
	Rollback(plan *models.ExecutionPlan, result *models.ExecutionResult) error
}


