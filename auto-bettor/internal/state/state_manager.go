package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
	"github.com/lib/pq"
)

// StateManager manages auto-betting state in database
type StateManager struct {
	db *sql.DB

	// Settings cache
	settingsCache       *models.UserSettings
	settingsCacheMutex  sync.RWMutex
	settingsCacheExpiry time.Time
	settingsCacheTTL    time.Duration
}

// NewStateManager creates a new state manager
func NewStateManager(db *sql.DB, cacheTTL time.Duration) *StateManager {
	return &StateManager{
		db:               db,
		settingsCacheTTL: cacheTTL,
	}
}

// GetUserSettings loads user settings from database or cache
func (sm *StateManager) GetUserSettings(ctx context.Context, userID string) (*models.UserSettings, error) {
	// Try cache first
	sm.settingsCacheMutex.RLock()
	if sm.settingsCache != nil && time.Now().Before(sm.settingsCacheExpiry) {
		defer sm.settingsCacheMutex.RUnlock()
		return sm.settingsCache, nil
	}
	sm.settingsCacheMutex.RUnlock()

	// Cache miss - fetch from DB
	query := `
		SELECT 
			auto_betting_enabled,
			auto_min_edge_pct,
			auto_enabled_opportunity_types,
			auto_enabled_markets,
			auto_enabled_books,
			auto_disabled_books,
			auto_max_stake_per_bet,
			auto_max_exposure_per_event,
			auto_max_exposure_total,
			auto_max_bets_per_hour,
			auto_max_bets_per_day,
			kelly_fraction,
			auto_max_kelly_pct,
			auto_min_stake,
			auto_max_data_age_seconds,
			auto_min_time_to_start_hours,
			auto_max_time_to_start_hours,
			auto_pause_on_loss_streak,
			auto_pause_on_daily_loss,
			auto_correlation_discount,
			auto_middle_enabled,
			auto_middle_execution_strategy,
			auto_middle_required_legs,
			auto_scalp_enabled,
			auto_scalp_bankroll_pct,
			auto_scalp_min_profit_pct,
			auto_scalp_execution_timeout_sec,
			auto_edge_allow_live_games
		FROM user_settings
		WHERE user_id = $1
	`

	settings := &models.UserSettings{UserID: userID}

	err := sm.db.QueryRowContext(ctx, query, userID).Scan(
		&settings.AutoBettingEnabled,
		&settings.AutoMinEdgePct,
		pq.Array(&settings.AutoEnabledOpportunityTypes),
		pq.Array(&settings.AutoEnabledMarkets),
		pq.Array(&settings.AutoEnabledBooks),
		pq.Array(&settings.AutoDisabledBooks),
		&settings.AutoMaxStakePerBet,
		&settings.AutoMaxExposurePerEvent,
		&settings.AutoMaxExposureTotal,
		&settings.AutoMaxBetsPerHour,
		&settings.AutoMaxBetsPerDay,
		&settings.KellyFraction,
		&settings.AutoMaxKellyPct,
		&settings.AutoMinStake,
		&settings.AutoMaxDataAgeSeconds,
		&settings.AutoMinTimeToStartHours,
		&settings.AutoMaxTimeToStartHours,
		&settings.AutoPauseOnLossStreak,
		&settings.AutoPauseOnDailyLoss,
		&settings.AutoCorrelationDiscount,
		&settings.AutoMiddleEnabled,
		&settings.AutoMiddleExecutionStrategy,
		&settings.AutoMiddleRequiredLegs,
		&settings.AutoScalpEnabled,
		&settings.AutoScalpBankrollPct,
		&settings.AutoScalpMinProfitPct,
		&settings.AutoScalpExecutionTimeoutSec,
		&settings.AutoEdgeAllowLiveGames,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to load user settings: %w", err)
	}

	// Update cache
	sm.settingsCacheMutex.Lock()
	sm.settingsCache = settings
	sm.settingsCacheExpiry = time.Now().Add(sm.settingsCacheTTL)
	sm.settingsCacheMutex.Unlock()

	return settings, nil
}

// InvalidateSettingsCache clears the settings cache (call when settings change)
func (sm *StateManager) InvalidateSettingsCache() {
	sm.settingsCacheMutex.Lock()
	sm.settingsCache = nil
	sm.settingsCacheMutex.Unlock()
}

// GetState loads auto-betting state from database
func (sm *StateManager) GetState(ctx context.Context, userID string) (*models.AutoBettingState, error) {
	query := `
		SELECT 
			total_exposure,
			exposure_by_event,
			exposure_by_book,
			bets_placed_last_hour,
			bets_placed_today,
			COALESCE(last_bet_placed_at, NOW()),
			todays_pnl,
			current_loss_streak,
			total_bets_placed,
			total_bets_won,
			total_bets_lost,
			is_paused,
			COALESCE(pause_reason, ''),
			COALESCE(paused_at, NOW()),
			last_updated
		FROM auto_betting_state
		WHERE user_id = $1
	`

	state := &models.AutoBettingState{UserID: userID}

	var exposureByEvent, exposureByBook []byte

	err := sm.db.QueryRowContext(ctx, query, userID).Scan(
		&state.TotalExposure,
		&exposureByEvent,
		&exposureByBook,
		&state.BetsPlacedLastHour,
		&state.BetsPlacedToday,
		&state.LastBetPlacedAt,
		&state.TodaysPnL,
		&state.CurrentLossStreak,
		&state.TotalBetsPlaced,
		&state.TotalBetsWon,
		&state.TotalBetsLost,
		&state.IsPaused,
		&state.PauseReason,
		&state.PausedAt,
		&state.LastUpdated,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Parse JSON maps
	state.ExposureByEvent = make(map[string]float64)
	state.ExposureByBook = make(map[string]float64)

	if len(exposureByEvent) > 0 {
		json.Unmarshal(exposureByEvent, &state.ExposureByEvent)
	}

	if len(exposureByBook) > 0 {
		json.Unmarshal(exposureByBook, &state.ExposureByBook)
	}

	return state, nil
}

// RecordBetPlaced updates state after a bet is placed
func (sm *StateManager) RecordBetPlaced(ctx context.Context, userID string, stake float64, eventID, bookKey string) error {
	// Update exposure and counters
	_, err := sm.db.ExecContext(ctx, `
		UPDATE auto_betting_state
		SET 
			total_exposure = total_exposure + $2,
			exposure_by_event = jsonb_set(
				COALESCE(exposure_by_event, '{}'::jsonb),
				$3::text[],
				(COALESCE((exposure_by_event->>$4)::numeric, 0) + $2)::text::jsonb
			),
			exposure_by_book = jsonb_set(
				COALESCE(exposure_by_book, '{}'::jsonb),
				$5::text[],
				(COALESCE((exposure_by_book->>$6)::numeric, 0) + $2)::text::jsonb
			),
			bets_placed_last_hour = bets_placed_last_hour + 1,
			bets_placed_today = bets_placed_today + 1,
			last_bet_placed_at = NOW(),
			total_bets_placed = total_bets_placed + 1
		WHERE user_id = $1
	`, userID, stake, []string{eventID}, eventID, []string{bookKey}, bookKey)

	return err
}

// LogDecision logs an auto-betting decision
func (sm *StateManager) LogDecision(ctx context.Context, decision *models.AutoBettingDecision) error {
	filterResultsJSON, _ := json.Marshal(decision.FilterResults)

	_, err := sm.db.ExecContext(ctx, `
		INSERT INTO auto_betting_decisions (
			user_id, opportunity_id, decision, decision_reason, filter_results,
			calculated_stake, calculated_edge, kelly_fraction_used, full_kelly_pct,
			bet_ids, execution_time_ms, current_exposure, current_bankroll, bets_placed_today
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, decision.UserID, decision.OpportunityID, decision.Decision, decision.DecisionReason,
		filterResultsJSON, decision.CalculatedStake, decision.CalculatedEdge,
		decision.KellyFractionUsed, decision.FullKellyPct, pq.Array(decision.BetIDs),
		decision.ExecutionTimeMs, decision.CurrentExposure, decision.CurrentBankroll,
		decision.BetsPlacedToday)

	return err
}

// parseStringArray parses PostgreSQL array bytes to []string
func parseStringArray(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}

	var result []string
	json.Unmarshal(data, &result)
	return result
}
