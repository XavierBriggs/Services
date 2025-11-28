package logger

import (
	"fmt"
	"time"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// DecisionLogger logs auto-betting decisions
type DecisionLogger struct{}

// NewDecisionLogger creates a new decision logger
func NewDecisionLogger() *DecisionLogger {
	return &DecisionLogger{}
}

// LogOpportunityReceived logs when an opportunity is received
func (dl *DecisionLogger) LogOpportunityReceived(opp models.Opportunity) {
	fmt.Printf("[%s] 📨 Opportunity received: ID=%d Type=%s Sport=%s Event=%s Market=%s Edge=%.2f%%\n",
		time.Now().Format("15:04:05"),
		opp.ID,
		opp.OpportunityType,
		opp.SportKey,
		opp.EventID,
		opp.MarketKey,
		opp.EdgePct,
	)
}

// LogFilterResult logs a filter evaluation result
func (dl *DecisionLogger) LogFilterResult(filterName string, result models.FilterResult) {
	if result.Passed {
		fmt.Printf("  ✓ %s: %s\n", filterName, result.Reason)
	} else {
		fmt.Printf("  ✗ %s: %s\n", filterName, result.Reason)
	}
}

// LogSkipped logs when an opportunity is skipped
func (dl *DecisionLogger) LogSkipped(opp models.Opportunity, reason string) {
	fmt.Printf("[%s] ⏭  Skipped opportunity %d: %s\n",
		time.Now().Format("15:04:05"),
		opp.ID,
		reason,
	)
}

// LogExecutionPlan logs the execution plan
func (dl *DecisionLogger) LogExecutionPlan(plan *models.ExecutionPlan) {
	fmt.Printf("[%s] 📋 Execution Plan: Type=%s Strategy=%s Legs=%d/%d TotalStake=$%.2f Bankroll=$%.2f\n",
		time.Now().Format("15:04:05"),
		plan.OpportunityType,
		plan.Strategy,
		plan.RequiredLegs,
		len(plan.Legs),
		plan.TotalStake,
		plan.Bankroll,
	)

	for _, leg := range plan.Legs {
		fmt.Printf("    Leg %d: %s %s $%.2f @ %d\n",
			leg.LegNumber,
			leg.BookKey,
			leg.OutcomeName,
			leg.Stake,
			leg.Price,
		)
	}
}

// LogExecutionResult logs the execution result
func (dl *DecisionLogger) LogExecutionResult(result *models.ExecutionResult) {
	statusIcon := "✅"
	if result.Status == "failed" {
		statusIcon = "❌"
	} else if result.Status == "partial" {
		statusIcon = "⚠️ "
	}

	fmt.Printf("[%s] %s Execution %s: Placed=%d Failed=%d Duration=%v\n",
		time.Now().Format("15:04:05"),
		statusIcon,
		result.Status,
		len(result.LegsPlaced),
		len(result.LegsFailed),
		result.TotalDuration,
	)

	for _, leg := range result.LegsPlaced {
		betIDStr := "N/A"
		if leg.BetID != nil {
			betIDStr = fmt.Sprintf("%d", *leg.BetID)
		}
		fmt.Printf("    ✓ %s: $%.2f (bet_id: %s, %v)\n",
			leg.BookKey,
			leg.Stake,
			betIDStr,
			leg.Duration,
		)
	}

	for _, leg := range result.LegsFailed {
		fmt.Printf("    ✗ %s: $%.2f (error: %v, %v)\n",
			leg.BookKey,
			leg.Stake,
			leg.Error,
			leg.Duration,
		)
	}
}

// LogError logs an error
func (dl *DecisionLogger) LogError(opp models.Opportunity, stage string, err error) {
	fmt.Printf("[%s] ❌ Error in %s for opportunity %d: %v\n",
		time.Now().Format("15:04:05"),
		stage,
		opp.ID,
		err,
	)
}


