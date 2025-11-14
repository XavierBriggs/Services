package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/XavierBriggs/fortuna/services/api-gateway/pkg/models"
)

// HolocronDB defines the interface for Holocron database operations
type HolocronDB interface {
	Ping(ctx context.Context) error
	CreateBet(ctx context.Context, bet *models.Bet) (int64, error)
	CreateBetAndUpdateBankroll(ctx context.Context, bet *models.Bet, userID string) (*models.BetCreationResult, error)
	GetBets(ctx context.Context, filters models.BetFilters) ([]*models.BetWithPerformance, error)
	GetBetByID(ctx context.Context, id int64) (*models.BetWithPerformance, error)
	GetBetSummary(ctx context.Context) (*models.BetSummary, error)
	GetUserSettings(ctx context.Context, userID string) (*models.UserSettings, error)
	UpdateUserSettings(ctx context.Context, userID string, update *models.UserSettingsUpdate) error
}

// HolocronPostgres implements HolocronDB for PostgreSQL
type HolocronPostgres struct {
	db *sql.DB
}

// NewHolocronPostgres creates a new Holocron database client
func NewHolocronPostgres(dsn string) (*HolocronPostgres, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &HolocronPostgres{db: db}, nil
}

// Ping checks database connectivity
func (h *HolocronPostgres) Ping(ctx context.Context) error {
	return h.db.PingContext(ctx)
}

// CreateBet inserts a new bet into the database
func (h *HolocronPostgres) CreateBet(ctx context.Context, bet *models.Bet) (int64, error) {
	query := `
		INSERT INTO bets (
			opportunity_id, sport_key, event_id, market_key, book_key,
			outcome_name, bet_type, stake_amount, bet_price, point,
			placed_at, result
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`

	var betID int64
	err := h.db.QueryRowContext(
		ctx, query,
		bet.OpportunityID,
		bet.SportKey,
		bet.EventID,
		bet.MarketKey,
		bet.BookKey,
		bet.OutcomeName,
		bet.BetType,
		bet.StakeAmount,
		bet.BetPrice,
		bet.Point,
		bet.PlacedAt,
		"pending",
	).Scan(&betID)

	if err != nil {
		return 0, fmt.Errorf("insert bet: %w", err)
	}

	return betID, nil
}

// CreateBetAndUpdateBankroll atomically creates a bet and updates the user's bankroll
func (h *HolocronPostgres) CreateBetAndUpdateBankroll(ctx context.Context, bet *models.Bet, userID string) (*models.BetCreationResult, error) {
	// Start transaction with serializable isolation
	tx, err := h.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Lock and get current bankrolls
	var bankrollsJSON []byte
	err = tx.QueryRowContext(ctx,
		`SELECT bankrolls FROM user_settings WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&bankrollsJSON)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user settings not found for user_id: %s", userID)
	}
	if err != nil {
		return nil, fmt.Errorf("get bankrolls: %w", err)
	}

	var bankrolls map[string]float64
	if err := json.Unmarshal(bankrollsJSON, &bankrolls); err != nil {
		return nil, fmt.Errorf("parse bankrolls JSON: %w", err)
	}

	previousBankroll, exists := bankrolls[bet.BookKey]
	if !exists {
		previousBankroll = 0
	}

	// 2. Validate sufficient bankroll
	if previousBankroll < bet.StakeAmount {
		return nil, fmt.Errorf(
			"insufficient bankroll for %s: have $%.2f, need $%.2f",
			bet.BookKey, previousBankroll, bet.StakeAmount,
		)
	}

	// 3. Insert bet
	var betID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO bets (
			opportunity_id, sport_key, event_id, market_key, book_key,
			outcome_name, bet_type, stake_amount, bet_price, point,
			placed_at, result
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`,
		bet.OpportunityID, bet.SportKey, bet.EventID, bet.MarketKey, bet.BookKey,
		bet.OutcomeName, bet.BetType, bet.StakeAmount, bet.BetPrice, bet.Point,
		bet.PlacedAt, "pending",
	).Scan(&betID)

	if err != nil {
		return nil, fmt.Errorf("insert bet: %w", err)
	}

	// 4. Update bankroll
	newBankroll := previousBankroll - bet.StakeAmount
	bankrolls[bet.BookKey] = newBankroll

	updatedBankrollsJSON, err := json.Marshal(bankrolls)
	if err != nil {
		return nil, fmt.Errorf("marshal updated bankrolls: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE user_settings 
		 SET bankrolls = $1, updated_at = NOW()
		 WHERE user_id = $2`,
		updatedBankrollsJSON, userID,
	)

	if err != nil {
		return nil, fmt.Errorf("update bankrolls: %w", err)
	}

	// 5. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &models.BetCreationResult{
		BetID:            betID,
		Status:           "created",
		UpdatedBankrolls: bankrolls,
		PreviousBankroll: previousBankroll,
		NewBankroll:      newBankroll,
		BankrollChange:   bet.StakeAmount,
	}, nil
}

// GetBets retrieves bets with optional filters
func (h *HolocronPostgres) GetBets(ctx context.Context, filters models.BetFilters) ([]*models.BetWithPerformance, error) {
	query := `
		SELECT 
			b.id, b.opportunity_id, b.sport_key, b.event_id, b.market_key, b.book_key,
			b.outcome_name, b.bet_type, b.stake_amount, b.bet_price, b.point,
			b.placed_at, b.settled_at, b.result, b.payout_amount,
			bp.closing_line_price, bp.clv_cents, bp.hold_time_seconds, bp.recorded_at
		FROM bets b
		LEFT JOIN bet_performance bp ON b.id = bp.bet_id
		WHERE 1=1
	`

	args := []interface{}{}
	argPos := 1

	if filters.SportKey != "" {
		query += fmt.Sprintf(" AND b.sport_key = $%d", argPos)
		args = append(args, filters.SportKey)
		argPos++
	}

	if filters.BookKey != "" {
		query += fmt.Sprintf(" AND b.book_key = $%d", argPos)
		args = append(args, filters.BookKey)
		argPos++
	}

	if filters.Result != "" {
		query += fmt.Sprintf(" AND b.result = $%d", argPos)
		args = append(args, filters.Result)
		argPos++
	}

	if filters.Since != nil {
		query += fmt.Sprintf(" AND b.placed_at >= $%d", argPos)
		args = append(args, *filters.Since)
		argPos++
	}

	if filters.Until != nil {
		query += fmt.Sprintf(" AND b.placed_at <= $%d", argPos)
		args = append(args, *filters.Until)
		argPos++
	}

	query += " ORDER BY b.placed_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filters.Limit)
		argPos++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filters.Offset)
	}

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query bets: %w", err)
	}
	defer rows.Close()

	var bets []*models.BetWithPerformance
	for rows.Next() {
		bet := &models.BetWithPerformance{
			Bet: models.Bet{},
		}

		err := rows.Scan(
			&bet.ID, &bet.OpportunityID, &bet.SportKey, &bet.EventID, &bet.MarketKey, &bet.BookKey,
			&bet.OutcomeName, &bet.BetType, &bet.StakeAmount, &bet.BetPrice, &bet.Point,
			&bet.PlacedAt, &bet.SettledAt, &bet.Result, &bet.PayoutAmount,
			&bet.ClosingLinePrice, &bet.CLVCents, &bet.HoldTimeSeconds, &bet.PerformanceRecordedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scan bet: %w", err)
		}

		bets = append(bets, bet)
	}

	return bets, nil
}

// GetBetByID retrieves a single bet with performance metrics
func (h *HolocronPostgres) GetBetByID(ctx context.Context, id int64) (*models.BetWithPerformance, error) {
	query := `
		SELECT 
			b.id, b.opportunity_id, b.sport_key, b.event_id, b.market_key, b.book_key,
			b.outcome_name, b.bet_type, b.stake_amount, b.bet_price, b.point,
			b.placed_at, b.settled_at, b.result, b.payout_amount,
			bp.closing_line_price, bp.clv_cents, bp.hold_time_seconds, bp.recorded_at
		FROM bets b
		LEFT JOIN bet_performance bp ON b.id = bp.bet_id
		WHERE b.id = $1
	`

	bet := &models.BetWithPerformance{
		Bet: models.Bet{},
	}

	err := h.db.QueryRowContext(ctx, query, id).Scan(
		&bet.ID, &bet.OpportunityID, &bet.SportKey, &bet.EventID, &bet.MarketKey, &bet.BookKey,
		&bet.OutcomeName, &bet.BetType, &bet.StakeAmount, &bet.BetPrice, &bet.Point,
		&bet.PlacedAt, &bet.SettledAt, &bet.Result, &bet.PayoutAmount,
		&bet.ClosingLinePrice, &bet.CLVCents, &bet.HoldTimeSeconds, &bet.PerformanceRecordedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("query bet: %w", err)
	}

	return bet, nil
}

// GetBetSummary retrieves aggregate P&L statistics
func (h *HolocronPostgres) GetBetSummary(ctx context.Context) (*models.BetSummary, error) {
	query := `
		SELECT 
			COUNT(*) as total_bets,
			COALESCE(SUM(stake_amount), 0) as total_wagered,
			COALESCE(SUM(CASE WHEN payout_amount IS NOT NULL THEN payout_amount ELSE 0 END), 0) as total_returned,
			COALESCE(AVG(bp.clv_cents), 0) as avg_clv_cents,
			COALESCE(
				SUM(CASE WHEN result = 'win' THEN 1 ELSE 0 END)::float / 
				NULLIF(SUM(CASE WHEN result IN ('win', 'loss') THEN 1 ELSE 0 END), 0) * 100,
				0
			) as win_rate_pct
		FROM bets b
		LEFT JOIN bet_performance bp ON b.id = bp.bet_id
	`

	summary := &models.BetSummary{}

	err := h.db.QueryRowContext(ctx, query).Scan(
		&summary.TotalBets,
		&summary.TotalWagered,
		&summary.TotalReturned,
		&summary.AvgCLVCents,
		&summary.WinRatePct,
	)

	if err != nil {
		return nil, fmt.Errorf("query summary: %w", err)
	}

	// Calculate derived metrics
	summary.NetProfit = summary.TotalReturned - summary.TotalWagered
	if summary.TotalWagered > 0 {
		summary.ROIPct = (summary.NetProfit / summary.TotalWagered) * 100
	}

	// Get by-sport breakdown
	summary.BySport, err = h.getSummaryBySport(ctx)
	if err != nil {
		return nil, fmt.Errorf("get by sport: %w", err)
	}

	// Get by-book breakdown
	summary.ByBook, err = h.getSummaryByBook(ctx)
	if err != nil {
		return nil, fmt.Errorf("get by book: %w", err)
	}

	return summary, nil
}

func (h *HolocronPostgres) getSummaryBySport(ctx context.Context) (map[string]models.SportSummary, error) {
	query := `
		SELECT 
			sport_key,
			COUNT(*) as count,
			COALESCE(SUM(stake_amount), 0) as wagered,
			COALESCE(SUM(CASE WHEN payout_amount IS NOT NULL THEN payout_amount ELSE 0 END), 0) as returned
		FROM bets
		GROUP BY sport_key
	`

	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]models.SportSummary)
	for rows.Next() {
		var sportKey string
		var summary models.SportSummary

		err := rows.Scan(&sportKey, &summary.Count, &summary.Wagered, &summary.Returned)
		if err != nil {
			return nil, err
		}

		summary.NetProfit = summary.Returned - summary.Wagered
		if summary.Wagered > 0 {
			summary.ROIPct = (summary.NetProfit / summary.Wagered) * 100
		}

		result[sportKey] = summary
	}

	return result, nil
}

func (h *HolocronPostgres) getSummaryByBook(ctx context.Context) (map[string]models.BookSummary, error) {
	query := `
		SELECT 
			book_key,
			COUNT(*) as count,
			COALESCE(SUM(stake_amount), 0) as wagered,
			COALESCE(SUM(CASE WHEN payout_amount IS NOT NULL THEN payout_amount ELSE 0 END), 0) as returned,
			COALESCE(
				SUM(CASE WHEN result = 'win' THEN 1 ELSE 0 END)::float / 
				NULLIF(SUM(CASE WHEN result IN ('win', 'loss') THEN 1 ELSE 0 END), 0) * 100,
				0
			) as win_rate
		FROM bets
		GROUP BY book_key
	`

	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]models.BookSummary)
	for rows.Next() {
		var bookKey string
		var summary models.BookSummary

		err := rows.Scan(&bookKey, &summary.Count, &summary.Wagered, &summary.Returned, &summary.WinRate)
		if err != nil {
			return nil, err
		}

		summary.NetProfit = summary.Returned - summary.Wagered
		if summary.Wagered > 0 {
			summary.ROIPct = (summary.NetProfit / summary.Wagered) * 100
		}

		result[bookKey] = summary
	}

	return result, nil
}

// GetUserSettings retrieves user settings
func (h *HolocronPostgres) GetUserSettings(ctx context.Context, userID string) (*models.UserSettings, error) {
	query := `
		SELECT 
			id, user_id, bankrolls, kelly_fraction, min_edge_threshold, max_stake_pct,
			created_at, updated_at
		FROM user_settings
		WHERE user_id = $1
	`

	settings := &models.UserSettings{
		Bankrolls: make(map[string]float64),
	}

	var bankrollsJSON []byte
	err := h.db.QueryRowContext(ctx, query, userID).Scan(
		&settings.ID,
		&settings.UserID,
		&bankrollsJSON,
		&settings.KellyFraction,
		&settings.MinEdgeThreshold,
		&settings.MaxStakePct,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("query user settings: %w", err)
	}

	// Parse JSON bankrolls
	if err := json.Unmarshal(bankrollsJSON, &settings.Bankrolls); err != nil {
		return nil, fmt.Errorf("parse bankrolls JSON: %w", err)
	}

	return settings, nil
}

// UpdateUserSettings updates user settings
func (h *HolocronPostgres) UpdateUserSettings(ctx context.Context, userID string, update *models.UserSettingsUpdate) error {
	// Convert bankrolls map to JSON
	bankrollsJSON, err := json.Marshal(update.Bankrolls)
	if err != nil {
		return fmt.Errorf("marshal bankrolls: %w", err)
	}

	query := `
		INSERT INTO user_settings (user_id, bankrolls, kelly_fraction, min_edge_threshold, max_stake_pct)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			bankrolls = EXCLUDED.bankrolls,
			kelly_fraction = EXCLUDED.kelly_fraction,
			min_edge_threshold = EXCLUDED.min_edge_threshold,
			max_stake_pct = EXCLUDED.max_stake_pct,
			updated_at = NOW()
	`

	_, err = h.db.ExecContext(
		ctx, query,
		userID,
		bankrollsJSON,
		update.KellyFraction,
		update.MinEdgeThreshold,
		update.MaxStakePct,
	)

	if err != nil {
		return fmt.Errorf("update user settings: %w", err)
	}

	return nil
}

// Close closes the database connection
func (h *HolocronPostgres) Close() error {
	return h.db.Close()
}


