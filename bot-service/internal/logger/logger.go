package logger

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// BetLogger logs bet execution attempts to bet_execution_logs table
type BetLogger struct {
	db *sql.DB
}

// ExecutionLog represents a bet execution log entry
type ExecutionLog struct {
	BetID          *int64
	OpportunityID  *int64
	BookKey        string
	TriggerSource  string // "manual" or "automated"
	ExecutionStage string // "login", "navigation", "bet_placement", "confirmation", "error"
	Status         string // "started", "success", "failed", "odds_moved", "timeout"
	LatencyMs      int
	ErrorMessage   string
	ScreenshotPath string
}

// NewBetLogger creates a new bet logger
func NewBetLogger(db *sql.DB) *BetLogger {
	return &BetLogger{
		db: db,
	}
}

// LogExecution logs a bet execution attempt
func (l *BetLogger) LogExecution(ctx context.Context, log *ExecutionLog) error {
	query := `
		INSERT INTO bet_execution_logs (
			bet_id, opportunity_id, book_key, trigger_source,
			execution_stage, status, latency_ms, error_message, screenshot_path
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := l.db.ExecContext(ctx, query,
		log.BetID,
		log.OpportunityID,
		log.BookKey,
		log.TriggerSource,
		log.ExecutionStage,
		log.Status,
		log.LatencyMs,
		log.ErrorMessage,
		log.ScreenshotPath,
	)

	if err != nil {
		return fmt.Errorf("failed to log execution: %w", err)
	}

	return nil
}

// LogStart logs the start of a bet execution
func (l *BetLogger) LogStart(ctx context.Context, opportunityID int64, bookKey string, triggerSource string) error {
	return l.LogExecution(ctx, &ExecutionLog{
		OpportunityID:  &opportunityID,
		BookKey:        bookKey,
		TriggerSource:  triggerSource,
		ExecutionStage: "bet_placement",
		Status:         "started",
		LatencyMs:      0,
	})
}

// LogSuccess logs a successful bet execution
func (l *BetLogger) LogSuccess(ctx context.Context, betID int64, opportunityID int64, bookKey string, triggerSource string, latencyMs int) error {
	return l.LogExecution(ctx, &ExecutionLog{
		BetID:          &betID,
		OpportunityID:  &opportunityID,
		BookKey:        bookKey,
		TriggerSource:  triggerSource,
		ExecutionStage: "bet_placement",
		Status:         "success",
		LatencyMs:      latencyMs,
	})
}

// LogFailure logs a failed bet execution
func (l *BetLogger) LogFailure(ctx context.Context, opportunityID int64, bookKey string, triggerSource string, latencyMs int, errorMsg string) error {
	return l.LogExecution(ctx, &ExecutionLog{
		OpportunityID:  &opportunityID,
		BookKey:        bookKey,
		TriggerSource:  triggerSource,
		ExecutionStage: "bet_placement",
		Status:         "failed",
		LatencyMs:      latencyMs,
		ErrorMessage:   errorMsg,
	})
}

