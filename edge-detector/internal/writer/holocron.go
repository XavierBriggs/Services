package writer

import (
	"context"
	"database/sql"
	"fmt"

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
			edge_pct, fair_price, detected_at, data_age_seconds
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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



