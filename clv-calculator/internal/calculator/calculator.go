package calculator

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CLVCalculator calculates CLV for bets
type CLVCalculator struct {
	alexandriaDB *sql.DB
	holocronDB   *sql.DB
}

// NewCLVCalculator creates a new CLV calculator
func NewCLVCalculator(alexandriaDB, holocronDB *sql.DB) *CLVCalculator {
	return &CLVCalculator{
		alexandriaDB: alexandriaDB,
		holocronDB:   holocronDB,
	}
}

// ProcessEvent calculates CLV for all pending bets on an event
func (c *CLVCalculator) ProcessEvent(ctx context.Context, eventID string) error {
	// Get closing lines for event
	closingLines, err := c.getClosingLines(ctx, eventID)
	if err != nil {
		return fmt.Errorf("get closing lines: %w", err)
	}

	if len(closingLines) == 0 {
		return fmt.Errorf("no closing lines found for event %s", eventID)
	}

	// Get pending bets for event
	bets, err := c.getPendingBets(ctx, eventID)
	if err != nil {
		return fmt.Errorf("get pending bets: %w", err)
	}

	if len(bets) == 0 {
		// No pending bets, nothing to do
		return nil
	}

	// Calculate CLV for each bet
	processed := 0
	for _, bet := range bets {
		// Find matching closing line
		closingLine := findMatchingLine(closingLines, bet)
		if closingLine == nil {
			fmt.Printf("[CLV] No matching closing line for bet %d (%s, %s, %s)\n",
				bet.ID, bet.MarketKey, bet.BookKey, bet.OutcomeName)
			continue
		}

		// Calculate CLV
		clv := calculateCLV(bet.BetPrice, closingLine.ClosingPrice)

		// Calculate hold time
		holdTime := int(closingLine.ClosedAt.Sub(bet.PlacedAt).Seconds())

		// Update bet_performance
		err := c.updatePerformance(ctx, bet.ID, clv, closingLine.ClosingPrice, holdTime)
		if err != nil {
			fmt.Printf("[CLV] Failed to update performance for bet %d: %v\n", bet.ID, err)
			continue
		}

		processed++
	}

	fmt.Printf("[CLV] Processed %d/%d bets for event %s\n", processed, len(bets), eventID)

	return nil
}

// ClosingLine represents a closing line
type ClosingLine struct {
	EventID      string
	MarketKey    string
	BookKey      string
	OutcomeName  string
	ClosingPrice int
	Point        *float64
	ClosedAt     time.Time
}

// Bet represents a bet
type Bet struct {
	ID          int64
	MarketKey   string
	BookKey     string
	OutcomeName string
	BetPrice    int
	Point       *float64
	PlacedAt    time.Time
}

func (c *CLVCalculator) getClosingLines(ctx context.Context, eventID string) ([]ClosingLine, error) {
	query := `
		SELECT event_id, market_key, book_key, outcome_name, closing_price, point, closed_at
		FROM closing_lines
		WHERE event_id = $1
	`

	rows, err := c.alexandriaDB.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []ClosingLine
	for rows.Next() {
		var line ClosingLine
		err := rows.Scan(
			&line.EventID,
			&line.MarketKey,
			&line.BookKey,
			&line.OutcomeName,
			&line.ClosingPrice,
			&line.Point,
			&line.ClosedAt,
		)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}

	return lines, nil
}

func (c *CLVCalculator) getPendingBets(ctx context.Context, eventID string) ([]Bet, error) {
	query := `
		SELECT id, market_key, book_key, outcome_name, bet_price, point, placed_at
		FROM bets
		WHERE event_id = $1 AND result = 'pending'
	`

	rows, err := c.holocronDB.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bets []Bet
	for rows.Next() {
		var bet Bet
		err := rows.Scan(
			&bet.ID,
			&bet.MarketKey,
			&bet.BookKey,
			&bet.OutcomeName,
			&bet.BetPrice,
			&bet.Point,
			&bet.PlacedAt,
		)
		if err != nil {
			return nil, err
		}
		bets = append(bets, bet)
	}

	return bets, nil
}

func (c *CLVCalculator) updatePerformance(ctx context.Context, betID int64, clv float64, closingPrice, holdTime int) error {
	query := `
		INSERT INTO bet_performance (bet_id, closing_line_price, clv_cents, hold_time_seconds, recorded_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (bet_id) DO UPDATE SET
			closing_line_price = EXCLUDED.closing_line_price,
			clv_cents = EXCLUDED.clv_cents,
			hold_time_seconds = EXCLUDED.hold_time_seconds,
			recorded_at = EXCLUDED.recorded_at
	`

	_, err := c.holocronDB.ExecContext(ctx, query, betID, closingPrice, clv, holdTime)
	return err
}

// findMatchingLine finds the closing line that matches a bet
func findMatchingLine(lines []ClosingLine, bet Bet) *ClosingLine {
	for i := range lines {
		line := &lines[i]
		if line.MarketKey == bet.MarketKey &&
			line.BookKey == bet.BookKey &&
			line.OutcomeName == bet.OutcomeName {
			// Points match (or both nil)
			if (line.Point == nil && bet.Point == nil) ||
				(line.Point != nil && bet.Point != nil && *line.Point == *bet.Point) {
				return line
			}
		}
	}
	return nil
}

// calculateCLV calculates CLV in cents per dollar
// CLV = (1/close_decimal - 1/bet_decimal) * 100
func calculateCLV(betPrice, closingPrice int) float64 {
	betDecimal := americanToDecimal(betPrice)
	closeDecimal := americanToDecimal(closingPrice)

	betProb := 1.0 / betDecimal
	closeProb := 1.0 / closeDecimal

	clvCents := (closeProb - betProb) * 100.0
	return clvCents
}

func americanToDecimal(american int) float64 {
	if american > 0 {
		return (float64(american) / 100.0) + 1.0
	}
	return (100.0 / float64(-american)) + 1.0
}


