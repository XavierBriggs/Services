package writer

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/models"
)

// HolocronWriter writes opportunities to the Holocron database
type HolocronWriter struct {
	db *sql.DB
}

// NewHolocronWriter creates a new Holocron writer
func NewHolocronWriter(db *sql.DB) *HolocronWriter {
	return &HolocronWriter{
		db: db,
	}
}

// WriteOpportunity writes an opportunity and its legs to Holocron
// Returns the opportunity ID on success
func (w *HolocronWriter) WriteOpportunity(ctx context.Context, opportunity models.Opportunity) (int64, error) {
	// Start transaction
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if commit doesn't happen

	// Insert opportunity
	opportunityQuery := `
		INSERT INTO opportunities (
			opportunity_type, sport_key, event_id, market_key,
			edge_pct, fair_price, detected_at, data_age_seconds, game_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	var opportunityID int64
	err = tx.QueryRowContext(
		ctx,
		opportunityQuery,
		string(opportunity.OpportunityType),
		opportunity.SportKey,
		opportunity.EventID,
		opportunity.MarketKey,
		opportunity.EdgePercent,
		opportunity.FairPrice,
		opportunity.DetectedAt,
		opportunity.DataAgeSeconds,
		opportunity.EventStatus,
	).Scan(&opportunityID)

	if err != nil {
		return 0, fmt.Errorf("failed to insert opportunity: %w", err)
	}

	// Insert opportunity legs
	legQuery := `
		INSERT INTO opportunity_legs (
			opportunity_id, book_key, outcome_name, price, point, leg_edge_pct
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	for _, leg := range opportunity.Legs {
		_, err = tx.ExecContext(
			ctx,
			legQuery,
			opportunityID,
			leg.BookKey,
			leg.OutcomeName,
			leg.Price,
			leg.Point,
			leg.LegEdgePercent,
		)

		if err != nil {
			return 0, fmt.Errorf("failed to insert opportunity leg: %w", err)
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return opportunityID, nil
}

// WriteOpportunities writes multiple opportunities in a batch
func (w *HolocronWriter) WriteOpportunities(ctx context.Context, opportunities []models.Opportunity) ([]int64, error) {
	if len(opportunities) == 0 {
		return nil, nil
	}

	ids := make([]int64, 0, len(opportunities))

	for _, opp := range opportunities {
		id, err := w.WriteOpportunity(ctx, opp)
		if err != nil {
			return ids, fmt.Errorf("failed to write opportunity: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// GetOpportunityByID retrieves an opportunity by ID (with legs)
func (w *HolocronWriter) GetOpportunityByID(ctx context.Context, id int64) (*models.Opportunity, error) {
	// Query opportunity
	opportunityQuery := `
		SELECT id, opportunity_type, sport_key, event_id, market_key,
		       edge_pct, fair_price, detected_at, data_age_seconds
		FROM opportunities
		WHERE id = $1
	`

	var opp models.Opportunity
	err := w.db.QueryRowContext(ctx, opportunityQuery, id).Scan(
		&opp.ID,
		&opp.OpportunityType,
		&opp.SportKey,
		&opp.EventID,
		&opp.MarketKey,
		&opp.EdgePercent,
		&opp.FairPrice,
		&opp.DetectedAt,
		&opp.DataAgeSeconds,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("opportunity not found: %d", id)
		}
		return nil, fmt.Errorf("failed to query opportunity: %w", err)
	}

	// Query legs
	legsQuery := `
		SELECT book_key, outcome_name, price, point, leg_edge_pct
		FROM opportunity_legs
		WHERE opportunity_id = $1
		ORDER BY id
	`

	rows, err := w.db.QueryContext(ctx, legsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query legs: %w", err)
	}
	defer rows.Close()

	var legs []models.OpportunityLeg
	for rows.Next() {
		var leg models.OpportunityLeg
		err := rows.Scan(
			&leg.BookKey,
			&leg.OutcomeName,
			&leg.Price,
			&leg.Point,
			&leg.LegEdgePercent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan leg: %w", err)
		}
		legs = append(legs, leg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating legs: %w", err)
	}

	opp.Legs = legs
	return &opp, nil
}

// UpsertOpportunity inserts a new opportunity or returns existing one if signature matches
// Returns (opportunityID, isNew, error)
func (w *HolocronWriter) UpsertOpportunity(ctx context.Context, opportunity models.Opportunity, signature string) (int64, bool, error) {
	// First, try to find existing opportunity by signature
	existingID, err := w.FindBySignature(ctx, signature)
	if err != nil && err != sql.ErrNoRows {
		return 0, false, fmt.Errorf("failed to check existing opportunity: %w", err)
	}

	if existingID > 0 {
		// Opportunity exists - update last_seen_at and emission_count
		err = w.UpdateLastSeen(ctx, existingID)
		if err != nil {
			return 0, false, fmt.Errorf("failed to update last_seen: %w", err)
		}
		return existingID, false, nil
	}

	// New opportunity - insert it
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert opportunity with signature and duration tracking fields
	opportunityQuery := `
		INSERT INTO opportunities (
			opportunity_type, sport_key, event_id, market_key,
			edge_pct, fair_price, detected_at, data_age_seconds,
			signature, first_seen_at, last_seen_at, emission_count, game_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, 1, $11)
		RETURNING id
	`

	now := time.Now()
	var opportunityID int64
	err = tx.QueryRowContext(
		ctx,
		opportunityQuery,
		string(opportunity.OpportunityType),
		opportunity.SportKey,
		opportunity.EventID,
		opportunity.MarketKey,
		opportunity.EdgePercent,
		opportunity.FairPrice,
		opportunity.DetectedAt,
		opportunity.DataAgeSeconds,
		signature,
		now,
		opportunity.EventStatus,
	).Scan(&opportunityID)

	if err != nil {
		return 0, false, fmt.Errorf("failed to insert opportunity: %w", err)
	}

	// Insert opportunity legs
	legQuery := `
		INSERT INTO opportunity_legs (
			opportunity_id, book_key, outcome_name, price, point, leg_edge_pct
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	for _, leg := range opportunity.Legs {
		_, err = tx.ExecContext(
			ctx,
			legQuery,
			opportunityID,
			leg.BookKey,
			leg.OutcomeName,
			leg.Price,
			leg.Point,
			leg.LegEdgePercent,
		)

		if err != nil {
			return 0, false, fmt.Errorf("failed to insert opportunity leg: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return opportunityID, true, nil
}

// FindBySignature finds an opportunity by its unique signature
// Returns the opportunity ID or sql.ErrNoRows if not found
// If a finalized opportunity reappears, we reactivate it
func (w *HolocronWriter) FindBySignature(ctx context.Context, signature string) (int64, error) {
	query := `
		SELECT id FROM opportunities 
		WHERE signature = $1 
		LIMIT 1
	`

	var id int64
	err := w.db.QueryRowContext(ctx, query, signature).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateLastSeen updates the last_seen_at timestamp and increments emission_count
// Also clears duration_seconds to reactivate a finalized opportunity
func (w *HolocronWriter) UpdateLastSeen(ctx context.Context, opportunityID int64) error {
	query := `
		UPDATE opportunities 
		SET last_seen_at = NOW(),
		    emission_count = emission_count + 1,
		    duration_seconds = NULL
		WHERE id = $1
	`

	_, err := w.db.ExecContext(ctx, query, opportunityID)
	if err != nil {
		return fmt.Errorf("failed to update last_seen: %w", err)
	}
	return nil
}

// FinalizeOpportunity marks an opportunity as ended by calculating its duration
func (w *HolocronWriter) FinalizeOpportunity(ctx context.Context, opportunityID int64) error {
	query := `
		UPDATE opportunities 
		SET duration_seconds = EXTRACT(EPOCH FROM (last_seen_at - first_seen_at))::int
		WHERE id = $1 AND duration_seconds IS NULL
	`

	_, err := w.db.ExecContext(ctx, query, opportunityID)
	if err != nil {
		return fmt.Errorf("failed to finalize opportunity: %w", err)
	}
	return nil
}

// FinalizeStaleOpportunities finalizes any opportunities not seen in the last N seconds
func (w *HolocronWriter) FinalizeStaleOpportunities(ctx context.Context, staleThresholdSeconds int) (int64, error) {
	query := `
		UPDATE opportunities 
		SET duration_seconds = EXTRACT(EPOCH FROM (last_seen_at - first_seen_at))::int
		WHERE duration_seconds IS NULL
		  AND last_seen_at < NOW() - ($1 || ' seconds')::INTERVAL
	`

	result, err := w.db.ExecContext(ctx, query, staleThresholdSeconds)
	if err != nil {
		return 0, fmt.Errorf("failed to finalize stale opportunities: %w", err)
	}

	count, _ := result.RowsAffected()
	return count, nil
}
