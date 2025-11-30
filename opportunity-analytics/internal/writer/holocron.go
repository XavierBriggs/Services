package writer

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/XavierBriggs/fortuna/services/opportunity-analytics/pkg/models"
)

// HolocronWriter queries statistics directly from source tables
type HolocronWriter struct {
	db          *sql.DB
	excludeLive bool // Default setting for excluding live games
}

// NewHolocronWriter creates a new Holocron writer
func NewHolocronWriter(db *sql.DB) *HolocronWriter {
	return &HolocronWriter{
		db:          db,
		excludeLive: true, // Default to excluding live games
	}
}

// SetExcludeLive sets the default for excluding live games
func (w *HolocronWriter) SetExcludeLive(exclude bool) {
	w.excludeLive = exclude
}

// liveFilter returns the SQL clause to filter live games if excludeLive is true
func (w *HolocronWriter) liveFilter(tableAlias string) string {
	if w.excludeLive {
		return fmt.Sprintf(" AND COALESCE(%s.game_status, 'upcoming') != 'live'", tableAlias)
	}
	return ""
}

// GetStatsSummary retrieves aggregated statistics directly from source tables
func (w *HolocronWriter) GetStatsSummary(ctx context.Context, startTime, endTime time.Time, bookKey, oppType, gameStatus string) (*models.StatsSummary, error) {
	var summary models.StatsSummary

	// Query opportunities directly
	oppQuery := `
		SELECT 
			COUNT(DISTINCT o.id) as total_opportunities,
			COALESCE(AVG(o.edge_pct), 0) as avg_edge_pct,
			COALESCE(MIN(o.edge_pct), 0) as min_edge,
			COALESCE(MAX(o.edge_pct), 0) as max_edge,
			COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY o.edge_pct), 0) as median_edge,
			COALESCE(AVG(o.duration_seconds), 0) as avg_duration
		FROM opportunities o
		WHERE o.signature IS NOT NULL
		  AND o.detected_at >= $1 AND o.detected_at <= $2
	`
	oppQuery += w.liveFilter("o")

	oppArgs := []interface{}{startTime, endTime}
	oppArgCount := 3

	if bookKey != "" {
		oppQuery += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM opportunity_legs ol WHERE ol.opportunity_id = o.id AND ol.book_key = $%d)", oppArgCount)
		oppArgs = append(oppArgs, bookKey)
		oppArgCount++
	}

	if oppType != "" {
		oppQuery += fmt.Sprintf(" AND o.opportunity_type = $%d", oppArgCount)
		oppArgs = append(oppArgs, oppType)
		oppArgCount++
	}

	if gameStatus != "" {
		oppQuery += fmt.Sprintf(" AND COALESCE(o.game_status, 'upcoming') = $%d", oppArgCount)
		oppArgs = append(oppArgs, gameStatus)
	}

	var totalOpps int
	var avgEdge, minEdge, maxEdge, medianEdge, avgDuration float64
	err := w.db.QueryRowContext(ctx, oppQuery, oppArgs...).Scan(
		&totalOpps, &avgEdge, &minEdge, &maxEdge, &medianEdge, &avgDuration,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query opportunities: %w", err)
	}

	summary.TotalOpportunities = totalOpps
	summary.AvgEdgePct = avgEdge
	summary.MinEdgePct = minEdge
	summary.MaxEdgePct = maxEdge
	summary.MedianEdgePct = medianEdge
	summary.AvgHoldTimeSeconds = int(avgDuration)

	// Query bets directly
	betQuery := `
		SELECT 
			COUNT(*) as total_bets,
			SUM(CASE WHEN result = 'win' THEN 1 ELSE 0 END) as wins,
			SUM(CASE WHEN result = 'loss' THEN 1 ELSE 0 END) as losses,
			COALESCE(SUM(CASE 
				WHEN result = 'win' THEN payout_amount - stake_amount
				WHEN result = 'loss' THEN -stake_amount
				ELSE 0
			END), 0) as net_profit,
			COALESCE(SUM(stake_amount), 0) as total_stake
		FROM bets b
		WHERE b.placed_at >= $1 AND b.placed_at <= $2
		  AND b.result IN ('win', 'loss')
	`

	betArgs := []interface{}{startTime, endTime}
	betArgCount := 3

	if bookKey != "" {
		betQuery += fmt.Sprintf(" AND b.book_key = $%d", betArgCount)
		betArgs = append(betArgs, bookKey)
		betArgCount++
	}

	if oppType != "" {
		betQuery += fmt.Sprintf(" AND b.bet_type = $%d", betArgCount)
		betArgs = append(betArgs, oppType)
	}

	var totalBets, wins, losses int
	var netProfit, totalStake float64
	err = w.db.QueryRowContext(ctx, betQuery, betArgs...).Scan(
		&totalBets, &wins, &losses, &netProfit, &totalStake,
	)
	if err != nil && err != sql.ErrNoRows {
		// Non-fatal, continue with zero values
		totalBets = 0
	}

	summary.TotalBets = totalBets
	summary.NetProfit = netProfit
	if totalStake > 0 {
		summary.ROI = (netProfit / totalStake) * 100
	}
	if wins+losses > 0 {
		summary.WinRate = float64(wins) / float64(wins+losses) * 100
	}

	// Query CLV from bet_performance
	clvQuery := `
		SELECT COALESCE(AVG(bp.clv_cents), 0)
		FROM bet_performance bp
		JOIN bets b ON bp.bet_id = b.id
		WHERE b.placed_at >= $1 AND b.placed_at <= $2
	`
	var avgCLV float64
	w.db.QueryRowContext(ctx, clvQuery, startTime, endTime).Scan(&avgCLV)
	summary.AvgCLV = avgCLV

	// Calculate execution rate
	if summary.TotalOpportunities > 0 {
		summary.ExecutionRate = float64(summary.TotalBets) / float64(summary.TotalOpportunities) * 100
	}

	// Get breakdowns
	summary.ByBook, _ = w.getStatsByDimension(ctx, "book_key", startTime, endTime, oppType, "")
	summary.ByType, _ = w.getStatsByDimension(ctx, "opportunity_type", startTime, endTime, "", bookKey)
	summary.BySport, _ = w.getStatsByDimension(ctx, "sport_key", startTime, endTime, oppType, bookKey)
	summary.ByMarket, _ = w.getStatsByDimension(ctx, "market_key", startTime, endTime, oppType, bookKey)

	return &summary, nil
}

// getStatsByDimension gets stats grouped by a dimension from source tables
func (w *HolocronWriter) getStatsByDimension(ctx context.Context, dimension string, startTime, endTime time.Time, oppTypeFilter, bookKeyFilter string) (map[string]models.Stats, error) {
	result := make(map[string]models.Stats)

	// Build query based on dimension
	var oppQuery string
	switch dimension {
	case "book_key":
		oppQuery = `
			SELECT 
				ol.book_key as key,
				COUNT(DISTINCT o.id) as opportunity_count,
				COUNT(DISTINCT CASE WHEN COALESCE(o.game_status, 'upcoming') = 'live' THEN o.id END) as live_count,
				COUNT(DISTINCT CASE WHEN COALESCE(o.game_status, 'upcoming') = 'upcoming' THEN o.id END) as pregame_count,
				COALESCE(AVG(o.edge_pct), 0) as avg_edge_pct,
				COALESCE(MIN(o.edge_pct), 0) as min_edge,
				COALESCE(MAX(o.edge_pct), 0) as max_edge,
				COALESCE(AVG(o.duration_seconds), 0) as avg_duration
			FROM opportunities o
			JOIN opportunity_legs ol ON ol.opportunity_id = o.id
			WHERE o.signature IS NOT NULL
			  AND o.detected_at >= $1 AND o.detected_at <= $2
			  AND ol.book_key IS NOT NULL AND ol.book_key != ''
		`
	case "opportunity_type":
		oppQuery = `
			SELECT 
				o.opportunity_type as key,
				COUNT(DISTINCT o.id) as opportunity_count,
				COALESCE(AVG(o.edge_pct), 0) as avg_edge_pct,
				COALESCE(MIN(o.edge_pct), 0) as min_edge,
				COALESCE(MAX(o.edge_pct), 0) as max_edge,
				COALESCE(AVG(o.duration_seconds), 0) as avg_duration
			FROM opportunities o
			WHERE o.signature IS NOT NULL
			  AND o.detected_at >= $1 AND o.detected_at <= $2
			  AND o.opportunity_type IS NOT NULL AND o.opportunity_type != ''
		`
	case "sport_key":
		oppQuery = `
			SELECT 
				o.sport_key as key,
				COUNT(DISTINCT o.id) as opportunity_count,
				COALESCE(AVG(o.edge_pct), 0) as avg_edge_pct,
				COALESCE(MIN(o.edge_pct), 0) as min_edge,
				COALESCE(MAX(o.edge_pct), 0) as max_edge,
				COALESCE(AVG(o.duration_seconds), 0) as avg_duration
			FROM opportunities o
			WHERE o.signature IS NOT NULL
			  AND o.detected_at >= $1 AND o.detected_at <= $2
			  AND o.sport_key IS NOT NULL AND o.sport_key != ''
		`
	case "market_key":
		oppQuery = `
			SELECT 
				o.market_key as key,
				COUNT(DISTINCT o.id) as opportunity_count,
				COALESCE(AVG(o.edge_pct), 0) as avg_edge_pct,
				COALESCE(MIN(o.edge_pct), 0) as min_edge,
				COALESCE(MAX(o.edge_pct), 0) as max_edge,
				COALESCE(AVG(o.duration_seconds), 0) as avg_duration
			FROM opportunities o
			WHERE o.signature IS NOT NULL
			  AND o.detected_at >= $1 AND o.detected_at <= $2
			  AND o.market_key IS NOT NULL AND o.market_key != ''
		`
	default:
		return result, nil
	}

	// Add live game filter
	oppQuery += w.liveFilter("o")

	args := []interface{}{startTime, endTime}
	argCount := 3

	if oppTypeFilter != "" && dimension != "opportunity_type" {
		oppQuery += fmt.Sprintf(" AND o.opportunity_type = $%d", argCount)
		args = append(args, oppTypeFilter)
		argCount++
	}

	if bookKeyFilter != "" && dimension != "book_key" {
		oppQuery += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM opportunity_legs ol2 WHERE ol2.opportunity_id = o.id AND ol2.book_key = $%d)", argCount)
		args = append(args, bookKeyFilter)
	}

	oppQuery += " GROUP BY 1 HAVING COUNT(DISTINCT o.id) > 0"

	rows, err := w.db.QueryContext(ctx, oppQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var stats models.Stats
		var avgDuration float64

		err := rows.Scan(&key, &stats.OpportunityCount, &stats.LiveOpportunities, &stats.PregameOpportunities, &stats.AvgEdgePct, &stats.MinEdgePct, &stats.MaxEdgePct, &avgDuration)
		if err != nil {
			continue
		}
		stats.AvgHoldTimeSeconds = int(avgDuration)
		result[key] = stats
	}

	// Enrich with bet stats
	w.enrichWithBetStats(ctx, result, dimension, startTime, endTime, oppTypeFilter, bookKeyFilter)

	return result, nil
}

// enrichWithBetStats adds bet statistics to the result map
func (w *HolocronWriter) enrichWithBetStats(ctx context.Context, result map[string]models.Stats, dimension string, startTime, endTime time.Time, oppTypeFilter, bookKeyFilter string) {
	var betDimension string
	switch dimension {
	case "book_key":
		betDimension = "b.book_key"
	case "opportunity_type":
		betDimension = "b.bet_type"
	case "sport_key":
		betDimension = "b.sport_key"
	case "market_key":
		betDimension = "b.market_key"
	default:
		return
	}

	betQuery := fmt.Sprintf(`
		SELECT 
			%s as key,
			COUNT(*) as total_bets,
			SUM(CASE WHEN result = 'win' THEN 1 ELSE 0 END) as wins,
			SUM(CASE WHEN result = 'loss' THEN 1 ELSE 0 END) as losses,
			COALESCE(SUM(CASE 
				WHEN result = 'win' THEN payout_amount - stake_amount
				WHEN result = 'loss' THEN -stake_amount
				ELSE 0
			END), 0) as net_profit,
			COALESCE(SUM(stake_amount), 0) as total_stake,
			COALESCE(AVG(bp.clv_cents), 0) as avg_clv
		FROM bets b
		LEFT JOIN bet_performance bp ON bp.bet_id = b.id
		WHERE b.placed_at >= $1 AND b.placed_at <= $2
		  AND b.result IN ('win', 'loss')
		GROUP BY %s
		HAVING %s IS NOT NULL AND %s != ''
	`, betDimension, betDimension, betDimension, betDimension)

	rows, err := w.db.QueryContext(ctx, betQuery, startTime, endTime)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var totalBets, wins, losses int
		var netProfit, totalStake, avgCLV float64

		err := rows.Scan(&key, &totalBets, &wins, &losses, &netProfit, &totalStake, &avgCLV)
		if err != nil {
			continue
		}

		stats := result[key]
		stats.TotalBets = totalBets
		stats.Wins = wins
		stats.Losses = losses
		stats.NetProfit = netProfit
		stats.AvgCLV = avgCLV
		if totalStake > 0 {
			stats.ROI = (netProfit / totalStake) * 100
		}
		if stats.OpportunityCount > 0 {
			stats.ExecutionRate = float64(stats.TotalBets) / float64(stats.OpportunityCount) * 100
		}
		result[key] = stats
	}
}

// GetTimeSeries retrieves time series data directly from source tables
func (w *HolocronWriter) GetTimeSeries(ctx context.Context, startTime, endTime time.Time, bookKey, oppType, gameStatus string) ([]models.TimeSeriesPoint, error) {
	query := `
		SELECT 
			date_trunc('hour', o.detected_at) + 
				(EXTRACT(minute FROM o.detected_at)::int / 5) * interval '5 minutes' as timestamp_bucket,
			COALESCE(o.opportunity_type, 'unknown') as opportunity_type,
			COALESCE(o.game_status, 'upcoming') as game_status,
			COUNT(DISTINCT o.id) as opportunity_count,
			COALESCE(AVG(o.edge_pct), 0) as avg_edge_pct,
			COALESCE(MIN(o.edge_pct), 0) as min_edge,
			COALESCE(MAX(o.edge_pct), 0) as max_edge,
			COALESCE(AVG(o.duration_seconds), 0) as avg_duration
		FROM opportunities o
		WHERE o.signature IS NOT NULL
		  AND o.detected_at >= $1 AND o.detected_at <= $2
	`
	query += w.liveFilter("o")

	args := []interface{}{startTime, endTime}
	argCount := 3

	if bookKey != "" {
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM opportunity_legs ol WHERE ol.opportunity_id = o.id AND ol.book_key = $%d)", argCount)
		args = append(args, bookKey)
		argCount++
	}

	if oppType != "" {
		query += fmt.Sprintf(" AND o.opportunity_type = $%d", argCount)
		args = append(args, oppType)
		argCount++
	}

	if gameStatus != "" {
		query += fmt.Sprintf(" AND COALESCE(o.game_status, 'upcoming') = $%d", argCount)
		args = append(args, gameStatus)
	}

	query += ` GROUP BY timestamp_bucket, o.opportunity_type, o.game_status
		ORDER BY timestamp_bucket ASC`

	rows, err := w.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query time series: %w", err)
	}
	defer rows.Close()

	var points []models.TimeSeriesPoint
	for rows.Next() {
		var point models.TimeSeriesPoint
		var avgDuration float64
		err := rows.Scan(
			&point.Timestamp,
			&point.OpportunityType,
			&point.GameStatus,
			&point.OpportunityCount,
			&point.AvgEdgePct,
			&point.MinEdgePct,
			&point.MaxEdgePct,
			&avgDuration,
		)
		if err != nil {
			continue
		}
		point.AvgHoldTimeSeconds = int(avgDuration)
		point.BookKey = "all"
		points = append(points, point)
	}

	return points, nil
}

// GetBestBookPairs retrieves book pair statistics directly from source tables
func (w *HolocronWriter) GetBestBookPairs(ctx context.Context, startTime, endTime time.Time, oppType, gameStatus string, limit int) ([]models.BookPairSummary, error) {
	query := `
		WITH book_pairs AS (
			SELECT DISTINCT
				o.id,
				o.opportunity_type,
				o.edge_pct,
				o.duration_seconds,
				COALESCE(o.game_status, 'upcoming') as game_status,
				CASE WHEN ol1.book_key < ol2.book_key THEN ol1.book_key ELSE ol2.book_key END as book_key_1,
				CASE WHEN ol1.book_key < ol2.book_key THEN ol2.book_key ELSE ol1.book_key END as book_key_2
			FROM opportunities o
			JOIN opportunity_legs ol1 ON ol1.opportunity_id = o.id
			JOIN opportunity_legs ol2 ON ol2.opportunity_id = o.id AND ol1.book_key < ol2.book_key
			WHERE o.signature IS NOT NULL
			  AND o.detected_at >= $1 AND o.detected_at <= $2
			  AND o.opportunity_type IN ('scalp', 'middle')
	`

	args := []interface{}{startTime, endTime}
	argCount := 3

	if oppType != "" {
		query += fmt.Sprintf(" AND o.opportunity_type = $%d", argCount)
		args = append(args, oppType)
		argCount++
	}

	if gameStatus != "" {
		query += fmt.Sprintf(" AND COALESCE(o.game_status, 'upcoming') = $%d", argCount)
		args = append(args, gameStatus)
		argCount++
	}

	query += `
		)
		SELECT 
			book_key_1,
			book_key_2,
			book_key_1 || ' + ' || book_key_2 as pair_name,
			opportunity_type,
			COUNT(DISTINCT id) as total_opportunities,
			COUNT(DISTINCT CASE WHEN game_status = 'live' THEN id END) as live_opportunities,
			COUNT(DISTINCT CASE WHEN game_status = 'upcoming' THEN id END) as pregame_opportunities,
			COALESCE(AVG(edge_pct), 0) as avg_edge,
			COALESCE(MAX(edge_pct), 0) as best_edge,
			COALESCE(AVG(duration_seconds), 0) as avg_duration,
			COALESCE(MIN(duration_seconds), 0) as min_duration,
			COALESCE(MAX(duration_seconds), 0) as max_duration
		FROM book_pairs
		GROUP BY book_key_1, book_key_2, opportunity_type
		ORDER BY COUNT(DISTINCT id) DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
	}

	rows, err := w.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query book pairs: %w", err)
	}
	defer rows.Close()

	var pairs []models.BookPairSummary
	for rows.Next() {
		var pair models.BookPairSummary
		var avgDuration, minDuration, maxDuration float64
		err := rows.Scan(
			&pair.BookKey1,
			&pair.BookKey2,
			&pair.PairName,
			&pair.OpportunityType,
			&pair.TotalOpportunities,
			&pair.LiveOpportunities,
			&pair.PregameOpportunities,
			&pair.AvgEdgePct,
			&pair.BestEdgePct,
			&avgDuration,
			&minDuration,
			&maxDuration,
		)
		if err != nil {
			continue
		}
		pair.AvgHoldTimeSeconds = int(avgDuration)
		pair.MinHoldTimeSeconds = int(minDuration)
		pair.MaxHoldTimeSeconds = int(maxDuration)
		pairs = append(pairs, pair)
	}

	// Enrich with bet stats for pairs
	w.enrichPairsWithBetStats(ctx, pairs, startTime, endTime)

	return pairs, nil
}

// enrichPairsWithBetStats adds bet statistics to book pairs
func (w *HolocronWriter) enrichPairsWithBetStats(ctx context.Context, pairs []models.BookPairSummary, startTime, endTime time.Time) {
	for i := range pairs {
		// Count bets placed on this pair
		betQuery := `
			SELECT 
				COUNT(DISTINCT b.opportunity_id) as bet_count,
				COALESCE(SUM(CASE 
					WHEN b.result = 'win' THEN b.payout_amount - b.stake_amount
					WHEN b.result = 'loss' THEN -b.stake_amount
					ELSE 0
				END), 0) as net_profit,
				COALESCE(SUM(b.stake_amount), 0) as total_stake
			FROM bets b
			WHERE b.bet_type = $1
			  AND b.placed_at >= $2 AND b.placed_at <= $3
			  AND b.result IN ('win', 'loss')
			  AND b.book_key IN ($4, $5)
		`
		var betCount int
		var netProfit, totalStake float64
		w.db.QueryRowContext(ctx, betQuery,
			pairs[i].OpportunityType, startTime, endTime,
			pairs[i].BookKey1, pairs[i].BookKey2,
		).Scan(&betCount, &netProfit, &totalStake)

		pairs[i].TotalBets = betCount
		pairs[i].TotalProfit = netProfit
		if totalStake > 0 {
			pairs[i].ROI = (netProfit / totalStake) * 100
		}
		if pairs[i].TotalOpportunities > 0 {
			pairs[i].ExecutionRate = float64(pairs[i].TotalBets) / float64(pairs[i].TotalOpportunities) * 100
		}
	}
}

// GetEdgeDistribution retrieves edge distribution directly from opportunities table
func (w *HolocronWriter) GetEdgeDistribution(ctx context.Context, startTime, endTime time.Time, bookKey, oppType, gameStatus string) (*models.EdgeDistribution, error) {
	query := `
		SELECT o.edge_pct
		FROM opportunities o
		WHERE o.signature IS NOT NULL
		  AND o.detected_at >= $1 AND o.detected_at <= $2
		  AND o.edge_pct > 0
	`
	query += w.liveFilter("o")

	args := []interface{}{startTime, endTime}
	argCount := 3

	if bookKey != "" {
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM opportunity_legs ol WHERE ol.opportunity_id = o.id AND ol.book_key = $%d)", argCount)
		args = append(args, bookKey)
		argCount++
	}

	if oppType != "" {
		query += fmt.Sprintf(" AND o.opportunity_type = $%d", argCount)
		args = append(args, oppType)
		argCount++
	}

	if gameStatus != "" {
		query += fmt.Sprintf(" AND COALESCE(o.game_status, 'upcoming') = $%d", argCount)
		args = append(args, gameStatus)
	}

	rows, err := w.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query edges: %w", err)
	}
	defer rows.Close()

	var edges []float64
	for rows.Next() {
		var edge float64
		if err := rows.Scan(&edge); err == nil {
			edges = append(edges, edge)
		}
	}

	if len(edges) == 0 {
		return &models.EdgeDistribution{
			Buckets: []models.EdgeBucket{},
			Stats:   models.EdgeStats{},
		}, nil
	}

	// Calculate distribution
	sort.Float64s(edges)

	// Create buckets: 0-2%, 2-5%, 5-10%, 10-20%, 20%+
	bucketRanges := [][2]float64{{0, 2}, {2, 5}, {5, 10}, {10, 20}, {20, 100}}
	buckets := make([]models.EdgeBucket, len(bucketRanges))

	for i, r := range bucketRanges {
		count := 0
		for _, e := range edges {
			if e >= r[0] && e < r[1] {
				count++
			}
		}
		buckets[i] = models.EdgeBucket{
			RangeStart: r[0],
			RangeEnd:   r[1],
			Count:      count,
			Percentage: float64(count) / float64(len(edges)) * 100,
		}
	}

	// Calculate stats
	sum := 0.0
	for _, e := range edges {
		sum += e
	}
	mean := sum / float64(len(edges))

	// Standard deviation
	variance := 0.0
	for _, e := range edges {
		variance += (e - mean) * (e - mean)
	}
	stddev := math.Sqrt(variance / float64(len(edges)))

	// Median
	median := edges[len(edges)/2]
	if len(edges)%2 == 0 {
		median = (edges[len(edges)/2-1] + edges[len(edges)/2]) / 2
	}

	return &models.EdgeDistribution{
		Buckets: buckets,
		Stats: models.EdgeStats{
			Min:    edges[0],
			Max:    edges[len(edges)-1],
			Mean:   mean,
			Median: median,
			Stddev: stddev,
			Total:  len(edges),
		},
	}, nil
}

// GetExecutionStats retrieves execution statistics directly from source tables
func (w *HolocronWriter) GetExecutionStats(ctx context.Context, startTime, endTime time.Time, bookKey string) (*models.ExecutionStats, error) {
	// Query opportunity stats
	oppQuery := `
		SELECT 
			COUNT(DISTINCT o.id) as total_opportunities,
			COALESCE(AVG(o.duration_seconds), 0) as avg_duration,
			COALESCE(MIN(o.duration_seconds), 0) as min_duration,
			COALESCE(MAX(o.duration_seconds), 0) as max_duration
		FROM opportunities o
		WHERE o.signature IS NOT NULL
		  AND o.detected_at >= $1 AND o.detected_at <= $2
	`
	oppQuery += w.liveFilter("o")

	oppArgs := []interface{}{startTime, endTime}
	if bookKey != "" {
		oppQuery += " AND EXISTS (SELECT 1 FROM opportunity_legs ol WHERE ol.opportunity_id = o.id AND ol.book_key = $3)"
		oppArgs = append(oppArgs, bookKey)
	}

	var stats models.ExecutionStats
	var avgDuration, minDuration, maxDuration float64
	err := w.db.QueryRowContext(ctx, oppQuery, oppArgs...).Scan(
		&stats.TotalOpportunities, &avgDuration, &minDuration, &maxDuration,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query opportunities: %w", err)
	}

	stats.AvgHoldTimeSeconds = int(avgDuration)
	stats.MinHoldTimeSeconds = int(minDuration)
	stats.MaxHoldTimeSeconds = int(maxDuration)

	// Query bet count
	betQuery := `SELECT COUNT(*) FROM bets WHERE placed_at >= $1 AND placed_at <= $2`
	betArgs := []interface{}{startTime, endTime}
	if bookKey != "" {
		betQuery += " AND book_key = $3"
		betArgs = append(betArgs, bookKey)
	}
	w.db.QueryRowContext(ctx, betQuery, betArgs...).Scan(&stats.TotalBetsPlaced)

	// Calculate execution rate
	if stats.TotalOpportunities > 0 {
		stats.ExecutionRate = float64(stats.TotalBetsPlaced) / float64(stats.TotalOpportunities) * 100
	}

	// Get conversion by book
	stats.ConversionByBook = make(map[string]float64)
	convQuery := `
		SELECT 
			ol.book_key,
			COUNT(DISTINCT o.id) as opps,
			COUNT(DISTINCT b.id) as bets
		FROM opportunities o
		JOIN opportunity_legs ol ON ol.opportunity_id = o.id
		LEFT JOIN bets b ON b.opportunity_id = o.id AND b.book_key = ol.book_key
		WHERE o.signature IS NOT NULL
		  AND o.detected_at >= $1 AND o.detected_at <= $2
		GROUP BY ol.book_key
	`
	convRows, err := w.db.QueryContext(ctx, convQuery, startTime, endTime)
	if err == nil {
		defer convRows.Close()
		for convRows.Next() {
			var book string
			var opps, bets int
			if convRows.Scan(&book, &opps, &bets) == nil && opps > 0 {
				stats.ConversionByBook[book] = float64(bets) / float64(opps) * 100
			}
		}
	}

	return &stats, nil
}

// GetOpportunityCLVSummary retrieves opportunity CLV statistics from opportunity_performance
func (w *HolocronWriter) GetOpportunityCLVSummary(ctx context.Context, startTime, endTime time.Time, bookKey, oppType string) (*models.OpportunityCLVSummary, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COALESCE(AVG(op.clv_cents), 0) as avg_clv,
			COALESCE(MIN(op.clv_cents), 0) as min_clv,
			COALESCE(MAX(op.clv_cents), 0) as max_clv,
			COALESCE(AVG(op.edge_at_detection), 0) as avg_edge_detection,
			COALESCE(AVG(op.edge_at_close), 0) as avg_edge_close,
			COALESCE(SUM(CASE WHEN op.clv_cents > 0 THEN 1 ELSE 0 END), 0) as positive_clv,
			COALESCE(SUM(CASE WHEN op.clv_cents <= 0 THEN 1 ELSE 0 END), 0) as negative_clv
		FROM opportunity_performance op
		JOIN opportunity_legs ol ON op.opportunity_leg_id = ol.id
		JOIN opportunities o ON ol.opportunity_id = o.id
		WHERE op.recorded_at >= $1 AND op.recorded_at <= $2
	`

	args := []interface{}{startTime, endTime}
	argCount := 3

	if bookKey != "" {
		query += fmt.Sprintf(" AND ol.book_key = $%d", argCount)
		args = append(args, bookKey)
		argCount++
	}

	if oppType != "" {
		query += fmt.Sprintf(" AND o.opportunity_type = $%d", argCount)
		args = append(args, oppType)
	}

	var summary models.OpportunityCLVSummary
	var positiveCount, negativeCount int
	err := w.db.QueryRowContext(ctx, query, args...).Scan(
		&summary.TotalOpportunities,
		&summary.AvgCLV,
		&summary.MinCLV,
		&summary.MaxCLV,
		&summary.AvgEdgeAtDetection,
		&summary.AvgEdgeAtClose,
		&positiveCount,
		&negativeCount,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query opportunity CLV: %w", err)
	}

	summary.PositiveCLVCount = positiveCount
	summary.NegativeCLVCount = negativeCount
	summary.AvgEdgeDecay = summary.AvgEdgeAtDetection - summary.AvgEdgeAtClose

	if summary.TotalOpportunities > 0 {
		summary.EdgeAccuracy = float64(positiveCount) / float64(summary.TotalOpportunities) * 100
		summary.FalsePositiveRate = float64(negativeCount) / float64(summary.TotalOpportunities) * 100
	}

	return &summary, nil
}

// GetHoldTimeStats retrieves hold time statistics
func (w *HolocronWriter) GetHoldTimeStats(ctx context.Context, startTime, endTime time.Time, bookKey string) (map[string]interface{}, error) {
	query := `
		SELECT 
			COALESCE(AVG(duration_seconds), 0) as avg_duration,
			COALESCE(MIN(duration_seconds), 0) as min_duration,
			COALESCE(MAX(duration_seconds), 0) as max_duration,
			COUNT(DISTINCT id) as total_opps
		FROM opportunities
		WHERE signature IS NOT NULL
		  AND detected_at >= $1 AND detected_at <= $2
		  AND duration_seconds IS NOT NULL
	`

	args := []interface{}{startTime, endTime}
	if bookKey != "" {
		query += " AND EXISTS (SELECT 1 FROM opportunity_legs ol WHERE ol.opportunity_id = opportunities.id AND ol.book_key = $3)"
		args = append(args, bookKey)
	}

	var avgDuration, minDuration, maxDuration float64
	var totalOpps int
	err := w.db.QueryRowContext(ctx, query, args...).Scan(&avgDuration, &minDuration, &maxDuration, &totalOpps)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Get total bets for execution rate
	betQuery := `SELECT COUNT(*) FROM bets WHERE placed_at >= $1 AND placed_at <= $2`
	var totalBets int
	w.db.QueryRowContext(ctx, betQuery, startTime, endTime).Scan(&totalBets)

	executionRate := 0.0
	if totalOpps > 0 {
		executionRate = float64(totalBets) / float64(totalOpps) * 100
	}

	return map[string]interface{}{
		"avg_hold_time_seconds": int(avgDuration),
		"min_hold_time_seconds": int(minDuration),
		"max_hold_time_seconds": int(maxDuration),
		"total_opportunities":   totalOpps,
		"execution_rate":        executionRate,
		"interpretation": map[string]string{
			"avg_window_description": fmt.Sprintf("Average opportunity window is %d seconds", int(avgDuration)),
			"execution_description":  fmt.Sprintf("%.1f%% of opportunities are being converted to bets", executionRate),
		},
	}, nil
}

// ============================================================================
// Legacy methods - kept for compatibility but simplified
// ============================================================================

// UpsertBookStats is kept for compatibility but now does nothing
// (we query directly from source tables instead of pre-aggregating)
func (w *HolocronWriter) UpsertBookStats(ctx context.Context, stats []models.BookStats) error {
	// No-op: we now query source tables directly
	return nil
}

// UpsertBookPairStats is kept for compatibility but now does nothing
func (w *HolocronWriter) UpsertBookPairStats(ctx context.Context, stats []models.BookPairStats) error {
	// No-op: we now query source tables directly
	return nil
}

// GetPairPerformance retrieves performance metrics for book pairs
func (w *HolocronWriter) GetPairPerformance(ctx context.Context, oppType string, limit int) ([]models.PairPerformance, error) {
	query := `
		SELECT 
			book_key_1,
			book_key_2,
			opportunity_type,
			total_opportunities,
			total_bets_placed,
			total_handle,
			realized_profit,
			roi_pct,
			win_count,
			loss_count,
			pending_count,
			execution_rate,
			updated_at
		FROM analytics_pair_performance
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 1

	if oppType != "" {
		query += fmt.Sprintf(" AND opportunity_type = $%d", argCount)
		args = append(args, oppType)
		argCount++
	}

	query += " ORDER BY total_handle DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
	}

	rows, err := w.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query pair performance: %w", err)
	}
	defer rows.Close()

	var results []models.PairPerformance
	for rows.Next() {
		var p models.PairPerformance
		var execRate sql.NullFloat64
		err := rows.Scan(
			&p.BookKey1, &p.BookKey2, &p.OpportunityType,
			&p.TotalOpportunities, &p.TotalBetsPlaced, &p.TotalHandle,
			&p.RealizedProfit, &p.ROIPct, &p.WinCount, &p.LossCount,
			&p.PendingCount, &execRate, &p.UpdatedAt,
		)
		if err != nil {
			continue
		}
		p.ExecutionRate = execRate.Float64
		p.PairName = p.BookKey1 + " × " + p.BookKey2
		results = append(results, p)
	}

	return results, nil
}

// RefreshPairPerformance recalculates pair performance from bets data
func (w *HolocronWriter) RefreshPairPerformance(ctx context.Context) error {
	query := `
		WITH pair_bets AS (
			-- Get all bets grouped by opportunity for scalps/middles
			SELECT 
				o.id as opportunity_id,
				o.opportunity_type,
				o.edge_pct,
				o.duration_seconds,
				LEAST(b1.book_key, b2.book_key) as book_key_1,
				GREATEST(b1.book_key, b2.book_key) as book_key_2,
				b1.stake_amount + COALESCE(b2.stake_amount, 0) as total_stake,
				CASE 
					WHEN b1.result IN ('win', 'loss') AND (b2.result IN ('win', 'loss') OR b2.id IS NULL) THEN
						COALESCE(
							CASE WHEN b1.result = 'win' THEN b1.payout_amount - b1.stake_amount ELSE -b1.stake_amount END, 0
						) + COALESCE(
							CASE WHEN b2.result = 'win' THEN b2.payout_amount - b2.stake_amount ELSE -b2.stake_amount END, 0
						)
					ELSE NULL
				END as profit,
				CASE 
					WHEN b1.result IN ('win', 'loss') AND (b2.result IN ('win', 'loss') OR b2.id IS NULL) THEN 'settled'
					ELSE 'pending'
				END as status,
				b1.placed_at as first_bet_at
			FROM opportunities o
			JOIN bets b1 ON b1.opportunity_id = o.id
			LEFT JOIN bets b2 ON b2.opportunity_id = o.id AND b2.id > b1.id
			WHERE o.opportunity_type IN ('scalp', 'middle')
			  AND o.signature IS NOT NULL
		),
		pair_stats AS (
			SELECT 
				book_key_1,
				book_key_2,
				opportunity_type,
				COUNT(DISTINCT opportunity_id) as total_bets,
				SUM(total_stake) as total_handle,
				SUM(CASE WHEN status = 'settled' THEN profit ELSE 0 END) as realized_profit,
				SUM(CASE WHEN status = 'settled' AND profit > 0 THEN 1 ELSE 0 END) as win_count,
				SUM(CASE WHEN status = 'settled' AND profit < 0 THEN 1 ELSE 0 END) as loss_count,
				SUM(CASE WHEN status = 'settled' AND profit = 0 THEN 1 ELSE 0 END) as push_count,
				SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_count,
				AVG(edge_pct) as avg_edge,
				AVG(duration_seconds) as avg_duration,
				MIN(first_bet_at) as first_bet_at,
				MAX(first_bet_at) as last_bet_at
			FROM pair_bets
			WHERE book_key_1 IS NOT NULL AND book_key_2 IS NOT NULL
			GROUP BY book_key_1, book_key_2, opportunity_type
		),
		opp_counts AS (
			SELECT 
				LEAST(ol1.book_key, ol2.book_key) as book_key_1,
				GREATEST(ol1.book_key, ol2.book_key) as book_key_2,
				o.opportunity_type,
				COUNT(DISTINCT o.id) as total_opportunities
			FROM opportunities o
			JOIN opportunity_legs ol1 ON ol1.opportunity_id = o.id
			JOIN opportunity_legs ol2 ON ol2.opportunity_id = o.id AND ol1.book_key < ol2.book_key
			WHERE o.opportunity_type IN ('scalp', 'middle')
			  AND o.signature IS NOT NULL
			GROUP BY LEAST(ol1.book_key, ol2.book_key), GREATEST(ol1.book_key, ol2.book_key), o.opportunity_type
		)
		INSERT INTO analytics_pair_performance (
			book_key_1, book_key_2, opportunity_type,
			total_opportunities, total_bets_placed, total_handle, realized_profit, roi_pct,
			win_count, loss_count, push_count, pending_count,
			avg_edge_at_detection, avg_hold_time_seconds, execution_rate,
			first_bet_at, last_bet_at, updated_at
		)
		SELECT 
			COALESCE(ps.book_key_1, oc.book_key_1),
			COALESCE(ps.book_key_2, oc.book_key_2),
			COALESCE(ps.opportunity_type, oc.opportunity_type),
			COALESCE(oc.total_opportunities, 0),
			COALESCE(ps.total_bets, 0),
			COALESCE(ps.total_handle, 0),
			COALESCE(ps.realized_profit, 0),
			CASE WHEN COALESCE(ps.total_handle, 0) > 0 
				THEN (COALESCE(ps.realized_profit, 0) / ps.total_handle) * 100 
				ELSE 0 
			END,
			COALESCE(ps.win_count, 0),
			COALESCE(ps.loss_count, 0),
			COALESCE(ps.push_count, 0),
			COALESCE(ps.pending_count, 0),
			ps.avg_edge,
			ps.avg_duration::int,
			CASE WHEN COALESCE(oc.total_opportunities, 0) > 0 
				THEN (COALESCE(ps.total_bets, 0)::float / oc.total_opportunities) * 100 
				ELSE 0 
			END,
			ps.first_bet_at,
			ps.last_bet_at,
			NOW()
		FROM opp_counts oc
		FULL OUTER JOIN pair_stats ps ON 
			oc.book_key_1 = ps.book_key_1 AND 
			oc.book_key_2 = ps.book_key_2 AND 
			oc.opportunity_type = ps.opportunity_type
		WHERE COALESCE(ps.book_key_1, oc.book_key_1) IS NOT NULL
		ON CONFLICT (book_key_1, book_key_2, opportunity_type) DO UPDATE SET
			total_opportunities = EXCLUDED.total_opportunities,
			total_bets_placed = EXCLUDED.total_bets_placed,
			total_handle = EXCLUDED.total_handle,
			realized_profit = EXCLUDED.realized_profit,
			roi_pct = EXCLUDED.roi_pct,
			win_count = EXCLUDED.win_count,
			loss_count = EXCLUDED.loss_count,
			push_count = EXCLUDED.push_count,
			pending_count = EXCLUDED.pending_count,
			avg_edge_at_detection = EXCLUDED.avg_edge_at_detection,
			avg_hold_time_seconds = EXCLUDED.avg_hold_time_seconds,
			execution_rate = EXCLUDED.execution_rate,
			first_bet_at = COALESCE(analytics_pair_performance.first_bet_at, EXCLUDED.first_bet_at),
			last_bet_at = EXCLUDED.last_bet_at,
			updated_at = NOW()
	`

	_, err := w.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to refresh pair performance: %w", err)
	}

	return nil
}

// GetOpportunityCLVByBook retrieves opportunity CLV stats grouped by book
func (w *HolocronWriter) GetOpportunityCLVByBook(ctx context.Context, startTime, endTime time.Time, oppType string) (map[string]models.OpportunityCLVStats, error) {
	query := `
		SELECT 
			ol.book_key,
			COUNT(*) as total,
			COALESCE(AVG(op.clv_cents), 0) as avg_clv,
			COALESCE(MIN(op.clv_cents), 0) as min_clv,
			COALESCE(MAX(op.clv_cents), 0) as max_clv,
			COALESCE(AVG(op.edge_at_detection), 0) as avg_edge_detection,
			COALESCE(SUM(CASE WHEN op.clv_cents > 0 THEN 1 ELSE 0 END), 0) as positive_clv,
			COALESCE(SUM(CASE WHEN op.clv_cents <= 0 THEN 1 ELSE 0 END), 0) as negative_clv
		FROM opportunity_performance op
		JOIN opportunity_legs ol ON op.opportunity_leg_id = ol.id
		JOIN opportunities o ON ol.opportunity_id = o.id
		WHERE op.recorded_at >= $1 AND op.recorded_at <= $2
	`

	args := []interface{}{startTime, endTime}
	if oppType != "" {
		query += " AND o.opportunity_type = $3"
		args = append(args, oppType)
	}

	query += " GROUP BY ol.book_key"

	rows, err := w.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]models.OpportunityCLVStats)
	for rows.Next() {
		var book string
		var stats models.OpportunityCLVStats
		err := rows.Scan(
			&book,
			&stats.TotalOpportunities,
			&stats.AvgCLV,
			&stats.MinCLV,
			&stats.MaxCLV,
			&stats.AvgEdgeAtDetection,
			&stats.PositiveCLVCount,
			&stats.NegativeCLVCount,
		)
		if err != nil {
			continue
		}
		if stats.TotalOpportunities > 0 {
			stats.EdgeAccuracy = float64(stats.PositiveCLVCount) / float64(stats.TotalOpportunities) * 100
		}
		result[book] = stats
	}

	return result, nil
}

// GetOpportunityCLVByType retrieves opportunity CLV stats grouped by opportunity type
func (w *HolocronWriter) GetOpportunityCLVByType(ctx context.Context, startTime, endTime time.Time, bookKey string) (map[string]models.OpportunityCLVStats, error) {
	query := `
		SELECT 
			o.opportunity_type,
			COUNT(*) as total,
			COALESCE(AVG(op.clv_cents), 0) as avg_clv,
			COALESCE(MIN(op.clv_cents), 0) as min_clv,
			COALESCE(MAX(op.clv_cents), 0) as max_clv,
			COALESCE(AVG(op.edge_at_detection), 0) as avg_edge_detection,
			COALESCE(SUM(CASE WHEN op.clv_cents > 0 THEN 1 ELSE 0 END), 0) as positive_clv,
			COALESCE(SUM(CASE WHEN op.clv_cents <= 0 THEN 1 ELSE 0 END), 0) as negative_clv
		FROM opportunity_performance op
		JOIN opportunity_legs ol ON op.opportunity_leg_id = ol.id
		JOIN opportunities o ON ol.opportunity_id = o.id
		WHERE op.recorded_at >= $1 AND op.recorded_at <= $2
	`

	args := []interface{}{startTime, endTime}
	if bookKey != "" {
		query += " AND ol.book_key = $3"
		args = append(args, bookKey)
	}

	query += " GROUP BY o.opportunity_type"

	rows, err := w.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]models.OpportunityCLVStats)
	for rows.Next() {
		var oppType string
		var stats models.OpportunityCLVStats
		err := rows.Scan(
			&oppType,
			&stats.TotalOpportunities,
			&stats.AvgCLV,
			&stats.MinCLV,
			&stats.MaxCLV,
			&stats.AvgEdgeAtDetection,
			&stats.PositiveCLVCount,
			&stats.NegativeCLVCount,
		)
		if err != nil {
			continue
		}
		if stats.TotalOpportunities > 0 {
			stats.EdgeAccuracy = float64(stats.PositiveCLVCount) / float64(stats.TotalOpportunities) * 100
		}
		result[oppType] = stats
	}

	return result, nil
}

// GetEdgeAccuracyTimeSeries retrieves time series data for edge accuracy
func (w *HolocronWriter) GetEdgeAccuracyTimeSeries(ctx context.Context, startTime, endTime time.Time, interval string) ([]models.EdgeAccuracyPoint, error) {
	// Determine bucket size
	bucketInterval := "1 hour"
	switch interval {
	case "5m", "5min":
		bucketInterval = "5 minutes"
	case "15m", "15min":
		bucketInterval = "15 minutes"
	case "hour", "1h":
		bucketInterval = "1 hour"
	case "day", "1d":
		bucketInterval = "1 day"
	}

	query := fmt.Sprintf(`
		SELECT 
			date_trunc('%s', op.recorded_at) as timestamp_bucket,
			COUNT(*) as total,
			COALESCE(AVG(op.clv_cents), 0) as avg_clv,
			COALESCE(AVG(op.edge_at_detection), 0) as avg_edge_detection,
			CASE WHEN COUNT(*) > 0 
				THEN (SUM(CASE WHEN op.clv_cents > 0 THEN 1 ELSE 0 END)::float / COUNT(*)::float) * 100
				ELSE 0
			END as edge_accuracy
		FROM opportunity_performance op
		WHERE op.recorded_at >= $1 AND op.recorded_at <= $2
		GROUP BY timestamp_bucket
		ORDER BY timestamp_bucket ASC
	`, bucketInterval)

	rows, err := w.db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query edge accuracy time series: %w", err)
	}
	defer rows.Close()

	var points []models.EdgeAccuracyPoint
	for rows.Next() {
		var point models.EdgeAccuracyPoint
		err := rows.Scan(
			&point.Timestamp,
			&point.TotalOpportunities,
			&point.AvgCLV,
			&point.AvgEdgeAtDetection,
			&point.EdgeAccuracy,
		)
		if err != nil {
			continue
		}
		points = append(points, point)
	}

	return points, nil
}
