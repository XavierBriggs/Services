package enricher

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// BetEnricher updates analytics with settled bet results
type BetEnricher struct {
	db *sql.DB
}

// NewBetEnricher creates a new bet enricher
func NewBetEnricher(db *sql.DB) *BetEnricher {
	return &BetEnricher{
		db: db,
	}
}

// EnrichSettledBets updates analytics with results from recently settled bets
// This should run periodically (e.g., every 15 minutes) to update analytics
// with bet outcomes after games complete
func (e *BetEnricher) EnrichSettledBets(ctx context.Context, lookbackHours int) error {
	fmt.Printf("🔄 Enriching analytics with settled bets from last %d hours...\n", lookbackHours)

	// Query to find all buckets that have settled bets needing enrichment
	query := `
		WITH settled_bet_metrics AS (
			-- Get metrics for bets that settled recently
			SELECT 
				DATE_TRUNC('minute', b.placed_at) - 
					(EXTRACT(MINUTE FROM b.placed_at)::int % 5) * INTERVAL '1 minute' as timestamp_bucket,
				b.book_key,
				COALESCE(o.opportunity_type, 'edge') as opportunity_type,
				COALESCE(o.game_status, 'upcoming') as game_status,
				COUNT(*) as total_bets,
				SUM(CASE WHEN b.result = 'win' THEN 1 ELSE 0 END) as wins,
				SUM(CASE WHEN b.result = 'loss' THEN 1 ELSE 0 END) as losses,
				COALESCE(AVG(bp.clv_cents), 0) / 100.0 as avg_clv,
				COALESCE(SUM(CASE 
					WHEN b.result = 'win' THEN b.payout_amount - b.stake_amount
					WHEN b.result = 'loss' THEN -b.stake_amount
					ELSE 0
				END), 0) as net_profit,
				COALESCE(SUM(b.stake_amount), 1) as total_stake
			FROM bets b
			LEFT JOIN opportunities o ON b.opportunity_id = o.id
			LEFT JOIN bet_performance bp ON b.id = bp.bet_id
			WHERE b.result IN ('win', 'loss')
			  AND b.settled_at IS NOT NULL
			  AND b.settled_at > NOW() - ($1 || ' hours')::INTERVAL
			GROUP BY 
				DATE_TRUNC('minute', b.placed_at) - 
					(EXTRACT(MINUTE FROM b.placed_at)::int % 5) * INTERVAL '1 minute',
				b.book_key, 
				COALESCE(o.opportunity_type, 'edge'),
				COALESCE(o.game_status, 'upcoming')
		)
		UPDATE analytics_book_stats abs
		SET 
			total_bets = sbm.total_bets,
			wins = sbm.wins,
			losses = sbm.losses,
			avg_clv = sbm.avg_clv,
			net_profit = sbm.net_profit,
			roi = CASE 
				WHEN sbm.total_stake > 0 
				THEN (sbm.net_profit / sbm.total_stake) * 100
				ELSE 0
			END,
			execution_rate = CASE 
				WHEN abs.opportunity_count > 0 
				THEN (sbm.total_bets::float / abs.opportunity_count::float) * 100
				ELSE 0
			END,
			updated_at = NOW()
		FROM settled_bet_metrics sbm
		WHERE abs.timestamp_bucket = sbm.timestamp_bucket
		  AND abs.book_key = sbm.book_key
		  AND abs.opportunity_type = sbm.opportunity_type
		  AND abs.game_status = sbm.game_status
		  -- Only update if bet metrics changed
		  AND (
		    abs.total_bets != sbm.total_bets OR
		    abs.wins != sbm.wins OR
		    abs.losses != sbm.losses OR
		    ABS(COALESCE(abs.net_profit, 0) - sbm.net_profit) > 0.01
		  )
	`

	result, err := e.db.ExecContext(ctx, query, lookbackHours)
	if err != nil {
		return fmt.Errorf("failed to enrich analytics: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("✅ Enriched %d analytics buckets with settled bet data\n", rowsAffected)

	// Also insert any missing buckets from bets that don't have analytics records yet
	insertQuery := `
		WITH settled_bet_metrics AS (
			SELECT 
				DATE_TRUNC('minute', b.placed_at) - 
					(EXTRACT(MINUTE FROM b.placed_at)::int % 5) * INTERVAL '1 minute' as timestamp_bucket,
				b.book_key,
				COALESCE(o.opportunity_type, 'edge') as opportunity_type,
				COALESCE(o.game_status, 'upcoming') as game_status,
				COALESCE(o.sport_key, '') as sport_key,
				COALESCE(o.market_key, '') as market_key,
				COUNT(*) as total_bets,
				SUM(CASE WHEN b.result = 'win' THEN 1 ELSE 0 END) as wins,
				SUM(CASE WHEN b.result = 'loss' THEN 1 ELSE 0 END) as losses,
				COALESCE(AVG(bp.clv_cents), 0) / 100.0 as avg_clv,
				COALESCE(SUM(CASE 
					WHEN b.result = 'win' THEN b.payout_amount - b.stake_amount
					WHEN b.result = 'loss' THEN -b.stake_amount
					ELSE 0
				END), 0) as net_profit,
				COALESCE(SUM(b.stake_amount), 1) as total_stake
			FROM bets b
			LEFT JOIN opportunities o ON b.opportunity_id = o.id
			LEFT JOIN bet_performance bp ON b.id = bp.bet_id
			WHERE b.result IN ('win', 'loss')
			  AND b.settled_at IS NOT NULL
			  AND b.settled_at > NOW() - ($1 || ' hours')::INTERVAL
			GROUP BY 
				DATE_TRUNC('minute', b.placed_at) - 
					(EXTRACT(MINUTE FROM b.placed_at)::int % 5) * INTERVAL '1 minute',
				b.book_key, 
				COALESCE(o.opportunity_type, 'edge'),
				COALESCE(o.game_status, 'upcoming'),
				COALESCE(o.sport_key, ''),
				COALESCE(o.market_key, '')
		)
		INSERT INTO analytics_book_stats (
			timestamp_bucket, book_key, opportunity_type, game_status,
			sport_key, market_key,
			opportunity_count, avg_edge_pct,
			total_bets, wins, losses, avg_clv, net_profit, roi,
			updated_at
		)
		SELECT 
			sbm.timestamp_bucket,
			sbm.book_key,
			sbm.opportunity_type,
			sbm.game_status,
			sbm.sport_key,
			sbm.market_key,
			0,  -- We don't know opportunity count from bets alone
			0,  -- We don't know avg edge from bets alone
			sbm.total_bets,
			sbm.wins,
			sbm.losses,
			sbm.avg_clv,
			sbm.net_profit,
			CASE WHEN sbm.total_stake > 0 THEN (sbm.net_profit / sbm.total_stake) * 100 ELSE 0 END,
			NOW()
		FROM settled_bet_metrics sbm
		WHERE NOT EXISTS (
			SELECT 1 FROM analytics_book_stats abs
			WHERE abs.timestamp_bucket = sbm.timestamp_bucket
			  AND abs.book_key = sbm.book_key
			  AND abs.opportunity_type = sbm.opportunity_type
			  AND abs.game_status = sbm.game_status
		)
		ON CONFLICT (timestamp_bucket, book_key, opportunity_type, game_status) DO NOTHING
	`

	insertResult, err := e.db.ExecContext(ctx, insertQuery, lookbackHours)
	if err != nil {
		// Log but don't fail - the main update is more important
		fmt.Printf("⚠️  Warning: Could not insert missing buckets: %v\n", err)
	} else {
		insertedRows, _ := insertResult.RowsAffected()
		if insertedRows > 0 {
			fmt.Printf("📝 Created %d new analytics buckets from settled bets\n", insertedRows)
		}
	}

	return nil
}

// EnrichPendingBets updates analytics for bets that are still pending
// This helps track conversion rates and identify stuck bets
func (e *BetEnricher) EnrichPendingBets(ctx context.Context) error {
	fmt.Printf("📊 Analyzing pending bets...\n")

	query := `
		SELECT 
			COUNT(*) as pending_count,
			MAX(b.placed_at) as oldest_pending,
			AVG(EXTRACT(EPOCH FROM (NOW() - b.placed_at)) / 3600) as avg_hours_pending
		FROM bets b
		WHERE b.result IS NULL
		  AND b.settled_at IS NULL
		  AND b.placed_at < NOW() - INTERVAL '4 hours'
	`

	var pendingCount int
	var oldestPending sql.NullTime
	var avgHoursPending sql.NullFloat64

	err := e.db.QueryRowContext(ctx, query).Scan(&pendingCount, &oldestPending, &avgHoursPending)
	if err != nil {
		return fmt.Errorf("failed to query pending bets: %w", err)
	}

	if pendingCount > 0 {
		fmt.Printf("⚠️  WARNING: %d bets pending for >4 hours (avg: %.1f hours)\n",
			pendingCount, avgHoursPending.Float64)

		if oldestPending.Valid {
			fmt.Printf("   Oldest pending bet: %s\n", oldestPending.Time.Format(time.RFC3339))
		}
	} else {
		fmt.Printf("✅ No stuck pending bets found\n")
	}

	return nil
}

// CalculateMissedOpportunities calculates how many opportunities were not converted to bets
func (e *BetEnricher) CalculateMissedOpportunities(ctx context.Context, lookbackHours int) error {
	fmt.Printf("📉 Calculating missed opportunities from last %d hours...\n", lookbackHours)

	query := `
		UPDATE analytics_book_stats abs
		SET 
			missed_opportunities = abs.opportunity_count - COALESCE(abs.total_bets, 0),
			execution_rate = CASE 
				WHEN abs.opportunity_count > 0 
				THEN (COALESCE(abs.total_bets, 0)::float / abs.opportunity_count::float) * 100
				ELSE 0
			END,
			updated_at = NOW()
		WHERE abs.timestamp_bucket > NOW() - ($1 || ' hours')::INTERVAL
		  AND abs.opportunity_count > 0
		  AND (
		    abs.missed_opportunities IS NULL OR
		    abs.missed_opportunities != abs.opportunity_count - COALESCE(abs.total_bets, 0)
		  )
	`

	result, err := e.db.ExecContext(ctx, query, lookbackHours)
	if err != nil {
		return fmt.Errorf("failed to calculate missed opportunities: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("✅ Updated missed opportunities for %d buckets\n", rowsAffected)

	return nil
}

// BackfillHistoricalBets re-enriches all historical analytics with settled bet data
// This should be run once after deploying this feature, or as needed
func (e *BetEnricher) BackfillHistoricalBets(ctx context.Context, daysBack int) error {
	fmt.Printf("🔄 Backfilling analytics with %d days of historical bet data...\n", daysBack)

	// Process in chunks to avoid overwhelming the database
	chunkSize := 24 // 24 hours at a time
	totalChunks := (daysBack * 24) / chunkSize

	for chunk := 0; chunk < totalChunks; chunk++ {
		hoursBack := chunk * chunkSize
		fmt.Printf("  Processing chunk %d/%d (hours %d-%d)...\n", chunk+1, totalChunks, hoursBack, hoursBack+chunkSize)

		if err := e.EnrichSettledBets(ctx, chunkSize); err != nil {
			return fmt.Errorf("backfill failed at chunk %d: %w", chunk, err)
		}

		// Calculate missed opportunities for this time range
		if err := e.CalculateMissedOpportunities(ctx, chunkSize); err != nil {
			fmt.Printf("⚠️  Warning: Could not calculate missed opportunities: %v\n", err)
		}

		// Small delay to be gentle on the database
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("✅ Backfill complete!\n")
	return nil
}

// GetEnrichmentSummary returns a summary of the current enrichment state
func (e *BetEnricher) GetEnrichmentSummary(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_buckets,
			SUM(opportunity_count) as total_opportunities,
			SUM(total_bets) as total_bets,
			SUM(wins) as total_wins,
			SUM(losses) as total_losses,
			SUM(net_profit) as total_profit,
			AVG(execution_rate) as avg_execution_rate,
			COUNT(CASE WHEN total_bets > 0 THEN 1 END) as buckets_with_bets,
			MAX(updated_at) as last_enrichment
		FROM analytics_book_stats
		WHERE timestamp_bucket > NOW() - INTERVAL '7 days'
	`

	var totalBuckets, totalOpps, totalBets, totalWins, totalLosses, bucketsWithBets int
	var totalProfit, avgExecutionRate float64
	var lastEnrichment sql.NullTime

	err := e.db.QueryRowContext(ctx, query).Scan(
		&totalBuckets, &totalOpps, &totalBets, &totalWins, &totalLosses,
		&totalProfit, &avgExecutionRate, &bucketsWithBets, &lastEnrichment,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get enrichment summary: %w", err)
	}

	summary := map[string]interface{}{
		"total_buckets":       totalBuckets,
		"total_opportunities": totalOpps,
		"total_bets":          totalBets,
		"total_wins":          totalWins,
		"total_losses":        totalLosses,
		"total_profit":        totalProfit,
		"avg_execution_rate":  avgExecutionRate,
		"buckets_with_bets":   bucketsWithBets,
		"enrichment_coverage": float64(bucketsWithBets) / float64(max(totalBuckets, 1)) * 100,
	}

	if lastEnrichment.Valid {
		summary["last_enrichment"] = lastEnrichment.Time.Format(time.RFC3339)
	}

	return summary, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// =========================================================================
// OPPORTUNITY CLV ENRICHMENT - Validates Edge Detector Accuracy
// =========================================================================

// EnrichOpportunityCLV updates analytics_book_stats with opportunity CLV metrics
// This validates edge detector accuracy by comparing detected edges to closing lines
func (e *BetEnricher) EnrichOpportunityCLV(ctx context.Context, lookbackHours int) error {
	fmt.Printf("🔍 Enriching analytics with opportunity CLV from last %d hours...\n", lookbackHours)

	// Update existing buckets with opportunity CLV metrics
	query := `
		WITH opportunity_clv_metrics AS (
			-- Get CLV metrics for opportunities that have closing line data
			SELECT 
				DATE_TRUNC('minute', o.detected_at) - 
					(EXTRACT(MINUTE FROM o.detected_at)::int % 5) * INTERVAL '1 minute' as timestamp_bucket,
				ol.book_key,
				o.opportunity_type,
				COALESCE(o.game_status, 'upcoming') as game_status,
				COUNT(*) as clv_count,
				AVG(op.clv_cents) as avg_clv,
				SUM(CASE WHEN op.clv_cents > 0 THEN 1 ELSE 0 END)::float / 
					NULLIF(COUNT(*), 0) * 100 as clv_accuracy
			FROM opportunity_performance op
			JOIN opportunity_legs ol ON op.opportunity_leg_id = ol.id
			JOIN opportunities o ON ol.opportunity_id = o.id
			WHERE op.recorded_at > NOW() - ($1 || ' hours')::INTERVAL
			GROUP BY 
				DATE_TRUNC('minute', o.detected_at) - 
					(EXTRACT(MINUTE FROM o.detected_at)::int % 5) * INTERVAL '1 minute',
				ol.book_key, 
				o.opportunity_type,
				COALESCE(o.game_status, 'upcoming')
		)
		UPDATE analytics_book_stats abs
		SET 
			avg_opportunity_clv = COALESCE(ocm.avg_clv, 0),
			opportunity_clv_count = COALESCE(ocm.clv_count, 0),
			opportunity_clv_accuracy = COALESCE(ocm.clv_accuracy, 0),
			updated_at = NOW()
		FROM opportunity_clv_metrics ocm
		WHERE abs.timestamp_bucket = ocm.timestamp_bucket
		  AND abs.book_key = ocm.book_key
		  AND abs.opportunity_type = ocm.opportunity_type
		  AND abs.game_status = ocm.game_status
		  -- Only update if CLV metrics changed
		  AND (
		    COALESCE(abs.opportunity_clv_count, 0) != COALESCE(ocm.clv_count, 0) OR
		    ABS(COALESCE(abs.avg_opportunity_clv, 0) - COALESCE(ocm.avg_clv, 0)) > 0.01
		  )
	`

	result, err := e.db.ExecContext(ctx, query, lookbackHours)
	if err != nil {
		return fmt.Errorf("failed to enrich opportunity CLV: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("✅ Enriched %d analytics buckets with opportunity CLV data\n", rowsAffected)

	return nil
}

// EnrichBookPairCLV updates analytics_book_pairs with opportunity CLV for scalps/middles
func (e *BetEnricher) EnrichBookPairCLV(ctx context.Context, lookbackHours int) error {
	fmt.Printf("🔍 Enriching book pair analytics with opportunity CLV...\n")

	// For book pairs, we need to aggregate CLV for opportunities that involve both books
	query := `
		WITH pair_clv_metrics AS (
			-- Get CLV metrics for book pairs (scalps/middles)
			SELECT 
				DATE_TRUNC('minute', o.detected_at) - 
					(EXTRACT(MINUTE FROM o.detected_at)::int % 5) * INTERVAL '1 minute' as timestamp_bucket,
				LEAST(ol1.book_key, ol2.book_key) as book_key_1,
				GREATEST(ol1.book_key, ol2.book_key) as book_key_2,
				o.opportunity_type,
				COALESCE(o.game_status, 'upcoming') as game_status,
				COUNT(DISTINCT o.id) as clv_count,
				AVG((COALESCE(op1.clv_cents, 0) + COALESCE(op2.clv_cents, 0)) / 2) as avg_clv,
				SUM(CASE WHEN (COALESCE(op1.clv_cents, 0) + COALESCE(op2.clv_cents, 0)) / 2 > 0 THEN 1 ELSE 0 END)::float / 
					NULLIF(COUNT(DISTINCT o.id), 0) * 100 as clv_accuracy
			FROM opportunities o
			JOIN opportunity_legs ol1 ON ol1.opportunity_id = o.id
			JOIN opportunity_legs ol2 ON ol2.opportunity_id = o.id AND ol1.id < ol2.id
			LEFT JOIN opportunity_performance op1 ON op1.opportunity_leg_id = ol1.id
			LEFT JOIN opportunity_performance op2 ON op2.opportunity_leg_id = ol2.id
			WHERE o.opportunity_type IN ('scalp', 'middle')
			  AND o.detected_at > NOW() - ($1 || ' hours')::INTERVAL
			  AND (op1.clv_cents IS NOT NULL OR op2.clv_cents IS NOT NULL)
			GROUP BY 
				DATE_TRUNC('minute', o.detected_at) - 
					(EXTRACT(MINUTE FROM o.detected_at)::int % 5) * INTERVAL '1 minute',
				LEAST(ol1.book_key, ol2.book_key),
				GREATEST(ol1.book_key, ol2.book_key),
				o.opportunity_type,
				COALESCE(o.game_status, 'upcoming')
		)
		UPDATE analytics_book_pairs abp
		SET 
			avg_opportunity_clv = COALESCE(pcm.avg_clv, 0),
			opportunity_clv_count = COALESCE(pcm.clv_count, 0),
			opportunity_clv_accuracy = COALESCE(pcm.clv_accuracy, 0),
			updated_at = NOW()
		FROM pair_clv_metrics pcm
		WHERE abp.timestamp_bucket = pcm.timestamp_bucket
		  AND abp.book_key_1 = pcm.book_key_1
		  AND abp.book_key_2 = pcm.book_key_2
		  AND abp.opportunity_type = pcm.opportunity_type
		  AND abp.game_status = pcm.game_status
	`

	result, err := e.db.ExecContext(ctx, query, lookbackHours)
	if err != nil {
		// Log but don't fail - book pairs table might not have data yet
		fmt.Printf("⚠️  Warning: Could not enrich book pair CLV: %v\n", err)
		return nil
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		fmt.Printf("✅ Enriched %d book pair buckets with opportunity CLV data\n", rowsAffected)
	}

	return nil
}

// RunFullEnrichment runs all enrichment tasks
func (e *BetEnricher) RunFullEnrichment(ctx context.Context, lookbackHours int) error {
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  BET ENRICHER - Full Enrichment Cycle\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")

	// 1. Enrich settled bets
	if err := e.EnrichSettledBets(ctx, lookbackHours); err != nil {
		fmt.Printf("⚠️  Warning: Settled bet enrichment failed: %v\n", err)
	}

	// 2. Analyze pending bets
	if err := e.EnrichPendingBets(ctx); err != nil {
		fmt.Printf("⚠️  Warning: Pending bet analysis failed: %v\n", err)
	}

	// 3. Calculate missed opportunities
	if err := e.CalculateMissedOpportunities(ctx, lookbackHours); err != nil {
		fmt.Printf("⚠️  Warning: Missed opportunity calculation failed: %v\n", err)
	}

	// 4. Enrich opportunity CLV (validates edge detector)
	if err := e.EnrichOpportunityCLV(ctx, lookbackHours); err != nil {
		fmt.Printf("⚠️  Warning: Opportunity CLV enrichment failed: %v\n", err)
	}

	// 5. Enrich book pair CLV (validates scalp/middle detection)
	if err := e.EnrichBookPairCLV(ctx, lookbackHours); err != nil {
		fmt.Printf("⚠️  Warning: Book pair CLV enrichment failed: %v\n", err)
	}

	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  Enrichment cycle complete!\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")

	return nil
}

// GetOpportunityCLVSummary returns a summary of opportunity CLV enrichment
func (e *BetEnricher) GetOpportunityCLVSummary(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_buckets,
			SUM(opportunity_clv_count) as total_clv_records,
			AVG(avg_opportunity_clv) as overall_avg_clv,
			AVG(opportunity_clv_accuracy) as overall_accuracy,
			COUNT(CASE WHEN opportunity_clv_count > 0 THEN 1 END) as buckets_with_clv,
			MAX(updated_at) as last_enrichment
		FROM analytics_book_stats
		WHERE timestamp_bucket > NOW() - INTERVAL '7 days'
		  AND opportunity_clv_count > 0
	`

	var totalBuckets, totalCLVRecords, bucketsWithCLV int
	var overallAvgCLV, overallAccuracy sql.NullFloat64
	var lastEnrichment sql.NullTime

	err := e.db.QueryRowContext(ctx, query).Scan(
		&totalBuckets, &totalCLVRecords, &overallAvgCLV, &overallAccuracy,
		&bucketsWithCLV, &lastEnrichment,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get opportunity CLV summary: %w", err)
	}

	summary := map[string]interface{}{
		"total_buckets_with_clv": bucketsWithCLV,
		"total_clv_records":      totalCLVRecords,
		"overall_avg_clv":        overallAvgCLV.Float64,
		"overall_accuracy":       overallAccuracy.Float64,
		"interpretation": map[string]interface{}{
			"avg_clv_meaning":  "Average cents gained/lost vs closing line. Positive = edges are real.",
			"accuracy_meaning": "Percentage of opportunities where edge held at close.",
		},
	}

	if lastEnrichment.Valid {
		summary["last_enrichment"] = lastEnrichment.Time.Format(time.RFC3339)
	}

	return summary, nil
}
